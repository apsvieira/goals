# PostHog analytics + OTLP logs — follow-ups

**Date:** 2026-05-23
**Status:** Shipped.

## Validity audit

Each follow-up item was verified against the current tree. Result:

| # | Item | Verdict | File:line |
|---|------|---------|-----------|
| A | `sync_completed.items_pulled` is actually push-acks | VALID | `frontend/src/lib/event-sync.ts:122` |
| B | `goal_deleted` fires from `archiveGoal` (soft delete) | VALID | `frontend/src/lib/api.ts:251` |
| C | `notification_permission_changed` not fired on resume → grant | VALID | `frontend/src/lib/components/NotificationSettings.svelte:97-106` |
| D | Per-request slog body is literal `"request"` | VALID | `backend/internal/api/router.go:353` |
| E | `loggerProvider.Shutdown` shares the 10s server-shutdown context | VALID | `backend/cmd/server/main.go:148,158` |
| F | `multiHandler.Handle` discards OTel errors with `_ =` | VALID | `backend/internal/api/router.go:51` |
| G | No build-time `service.version` resource attr | VALID | `backend/cmd/server/main.go:27`, `Dockerfile:17` |
| H | Startup/shutdown lines use stdlib `log.Printf`, bypass OTel bridge | VALID | `backend/cmd/server/main.go` (multiple lines) |
| I | `loadNotificationSettings` imported but unused in `api.ts` | VALID | `frontend/src/lib/api.ts:23` |
| J | `disable_external_dependency_loading: true` | VALID | `frontend/src/lib/analytics/posthog.ts:106` |
| K | `TestOTelBridge_NotInstalledWithoutToken` only asserts `/health` 200 | VALID | `backend/internal/api/otel_test.go:13-28` |
| L | No `logtest` test for the bridge-on fan-out path | VALID | (no occurrences in tests) |
| M | Play Store data-safety form not updated for analytics | (external, user action) | — |
| N | Region hardcoded in CSP (`VITE_POSTHOG_HOST` and Fly OTLP endpoint are already env-driven) | VALID — only the CSP host is hardcoded | `backend/internal/api/router.go:383` |

## Triage

### Will fix in this batch
Items that fix a real gap, are cheap, and don't change product behavior beyond telemetry quality:

- **C** — closes a real telemetry hole (grant-via-resume is silent today).
- **D** — already on the user's radar; trivial.
- **E, F** — correctness on the shutdown/error-fanout paths.
- **G** — gives every log a version pin; small build wiring cost.
- **H** — only the lines that run *after* slog is initialized; pre-init lines stay on stdlib `log`.
- **I** — trivial cleanup.
- **K, L** — strengthen the only safety net we have for the bridge.
- **A** — rename `items_pulled` → `items_acked`. Done now while the event volume is days old and no dashboards depend on the name yet; later the rename costs a property-merge in PostHog.
- **N** — parametrize the CSP PostHog host so region moves are pure secrets, no code change. ~5 lines.

### Defer with rationale (not in this batch)

- **B** (`goal_deleted` from `archiveGoal`) — semantically defensible today (soft-delete is the user-visible "delete"). Revisit only when/if a hard-delete path is introduced. Captured as a TODO in the event taxonomy doc instead of code.
- **J** (`disable_external_dependency_loading`) — purely forward-looking. The app has no CSP today, so flipping this is a one-line change when (if) Toolbar/Surveys/Web-Vitals is wanted. No action now.
- **M** (Play Store data-safety form) — out of code scope. Calling out here so it isn't forgotten when the next Play Store update goes out.

## Implementation plan

Each step is independently shippable and can land as its own commit. Suggested commit grouping in parentheses.

### Step 1 — Backend slog/OTel hardening *(commit: `fix(backend): tighten OTel shutdown, error fan-out, log formatting, and CSP region`)*

Touches `backend/internal/api/router.go`, `backend/cmd/server/main.go`.

1. **D — informative request log body.** Replace the static `"request"` body with `fmt.Sprintf("%s %s %d", r.Method, r.URL.Path, ww.Status())` at `router.go:353`. Keep all current slog attrs unchanged (PostHog Logs uses both `body` and structured attrs; existing filters keyed on `method`/`path`/`status` continue to work).

2. **F — surface multiHandler errors.** In `router.go:48-55`, accumulate errors with `errors.Join` and return the joined error from `Handle`. (Continue to call every enabled child handler before returning, so a failing OTel handler doesn't block stdout.)

3. **E — dedicated shutdown context for the LoggerProvider.** In `main.go`, after `server.Shutdown(ctx)`, build a fresh `context.WithTimeout(context.Background(), 5*time.Second)` for `loggerProvider.Shutdown` so a slow HTTP drain can't eat the flush budget. Defer-cancel both contexts.

4. **H — route startup/shutdown messages through slog where the logger exists.** Convert `log.Printf`/`log.Println` calls that run **after** `init()` (which runs `initLogger`) — i.e. all of `main()` — to `api.Logger.Info(...)` / `api.Logger.Error(...)`. Keep the pre-`flag.Parse` stdlib `log.Fatal*` calls and the bridge-init error message (`"posthog token set but OTEL_EXPORTER_OTLP_LOGS_ENDPOINT missing"`) on stdlib `log` because (a) they may run before slog is ready, and (b) one of them is *about* the bridge being down. Bodies should be short structured-log-friendly strings, e.g. `Logger.Info("server starting", "addr", serverAddr)`.

5. **N — parametrize the CSP PostHog ingest host.** In `securityHeaders` (`router.go:365`), read `POSTHOG_INGEST_HOST` at middleware-construction time (server boot, not per-request), default to `eu.i.posthog.com` if empty, and interpolate into the `connect-src` directive. Validate the value with a strict host regex (`^[a-z0-9.-]+$`) at startup and `log.Fatalf` on mismatch — a malformed CSP silently breaks every request, so loud failure is better than silent. The Fly secret should hold just the host (no scheme, no trailing slash); the CSP grammar adds `https://` itself. After this, a region move is: rotate `VITE_POSTHOG_HOST`, `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`, and `POSTHOG_INGEST_HOST`, redeploy — zero code change. Update the existing `router.go:382` comment to reflect the new env var.

### Step 2 — Build-time service version *(commit: `feat(backend): pin service.version from build-time ldflags`)*

Touches `backend/cmd/server/main.go`, `Dockerfile`, `Makefile`, GitHub Actions backend build workflow (if present).

1. Add a package-level `var version = "dev"` in `cmd/server/main.go`.
2. In `buildLoggerProvider`, after `resource.New(...)`, merge in `semconv.ServiceVersion(version)` (use `resource.Merge` or pass it via `resource.WithAttributes`).
3. Update the Docker build step (`Dockerfile:17`) to `go build -ldflags "-X main.version=${VERSION}"`. Pass `VERSION` as a build-arg sourced from `git describe --tags --always --dirty` in the Makefile and CI workflow. Locally, `make` should default to `git describe` so dev builds also tag themselves.
4. Verify in PostHog Logs that incoming records carry `service.version` after a redeploy.

### Step 3 — Strengthen the OTel bridge tests *(commit: `test(backend): exercise OTel bridge install + fan-out paths`)*

Touches `backend/internal/api/otel_test.go` (and adds a sibling test if it cleans up).

1. **K — strengthen `TestOTelBridge_NotInstalledWithoutToken`.** Instead of asserting `/health` 200, assert that the slog default handler **is not** a `multiHandler` (or that `otelLoggerProvider` is nil) when `POSTHOG_PROJECT_TOKEN` is empty. Use the package-level `otelLoggerProvider` sentinel or expose a small `loggerProviderInstalled()` accessor.

2. **L — add a bridge-on fan-out test.** Use `go.opentelemetry.io/otel/sdk/log/logtest` (already pulled in transitively — confirm and `go get` if not) to construct an in-memory `LoggerProvider`, call `SetOTelLoggerProvider` with it, re-run `initLogger`, emit `Logger.Info("hello", "k", "v")`, and assert:
   - the in-memory recorder received one record with body `"hello"` and attr `k=v`;
   - stdout (captured via redirect) also received a JSON line for the same record (proves both branches of the fan-out fire).
   Tear down by resetting the package-level provider to nil and re-running `initLogger`.

### Step 4 — Frontend: rename `items_pulled`, plug the resume gap, cleanup *(commit: `fix(frontend): correct sync_completed property name; capture grant-via-resume; drop dead import`)*

Touches `frontend/src/lib/event-sync.ts`, `frontend/src/lib/components/NotificationSettings.svelte`, `frontend/src/lib/api.ts`.

1. **A — rename property.** In `event-sync.ts:122`, change `items_pulled: data.processed?.length ?? 0` to `items_acked: data.processed?.length ?? 0`. If/when a true server→client pull arrives in the sync protocol, a separate `items_pulled` field can be added then without ambiguity. Update the event taxonomy notes in `docs/plans/2026-05-23-posthog-analytics-and-logs.md` Phase 3 section to match. No PostHog dashboard touches needed — none rely on this property yet.

2. **C — capture grant-via-resume.** In `NotificationSettings.svelte:97-106`, after the resume listener confirms `granted === true` and the settings update succeeds, call `capture('notification_permission_changed', { granted: true, source: 'resume' })`. Mirror the same call on the in-app prompt path (add a `source: 'prompt'` discriminator there too if it isn't present) so analytics can split by where the grant happened.

3. **I — remove unused import.** Delete `loadNotificationSettings` from the import list at `frontend/src/lib/api.ts:23`.

### Step 5 — Documentation touch-ups *(commit: `docs(plans): log posthog follow-ups; flag deferred items`)*

1. In `docs/plans/2026-05-23-posthog-analytics-and-logs.md`, append a short "Follow-ups" section linking to this plan and to the four deferred items (B, J, M, N) with one-line context each.
2. Mark this plan as `Shipped` once Steps 1–4 land and the deploy goes out.

## Test plan

- `go test ./...` from `backend/` — Steps 1–3.
- Backend integration: deploy to staging (or run locally with `POSTHOG_PROJECT_TOKEN`/`OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` set against a throwaway PostHog project) and confirm:
  - One log per request, body now reads `"GET /api/goals 200"`.
  - `service.version` populated in PostHog Logs.
  - Startup line "server starting" appears in PostHog Logs (previously it wouldn't have).
- `npm run check && npm run test` from `frontend/` — Step 4.
- Manual frontend verification of C:
  - Fresh install → deny notifications → background app → flip OS permission to grant → resume app.
  - PostHog Events should show one `notification_permission_changed` with `granted: true, source: 'resume'`.

## Risks / open questions

- **`items_pulled` rename** is technically a breaking property change. Mitigation: only days old, no dashboards. If the user has already pinned an insight on `items_pulled`, the safer path is to keep the old name and add a `Why:` doc comment instead. **Confirm with user before Step 4.1.**
- **slog conversion in `main.go`** (H) changes a small amount of log shape (stdlib `log` includes a date prefix; slog records don't unless configured). Production already runs JSON slog elsewhere, so combined logs become more consistent, not less.
- **`-ldflags` version wiring** (G) adds a build-arg dependency. If CI passes the wrong value (e.g. empty), the default `"dev"` keeps things working — no hard failure mode.
