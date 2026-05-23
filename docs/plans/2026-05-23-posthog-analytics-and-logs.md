# PostHog analytics + OTLP logs — design

**Date:** 2026-05-23
**Status:** Phases 1–3 implemented and verified live on device (2026-05-23). Phases 4–5 deferred per plan.

## Setup status (2026-05-23)

- ✅ PostHog EU project `tiny-tracker` created; project token captured.
- ✅ Fly secrets set (`POSTHOG_PROJECT_TOKEN`, `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://eu.i.posthog.com/i/v1/logs`, `OTEL_EXPORTER_OTLP_LOGS_HEADERS`, `OTEL_SERVICE_NAME=goal-tracker-backend`, `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod`).
- ✅ EU OTLP endpoint smoke-tested via `curl` → HTTP 200.
- ✅ Phase 1 (backend OTel slog bridge + `X-Request-Id` header) — shipped in `9c3e8a4`.
- ✅ Phase 2 GitHub Actions secrets (`VITE_POSTHOG_KEY`, `VITE_POSTHOG_HOST`) set; frontend integration shipped in `13c08f4`.
- ✅ End-to-end verified: backend OTLP logs visible in PostHog Logs with `request_id`; frontend identify + curated events visible in PostHog Events with `request_id` super-property.


## Goal

Add product analytics and structured-log visibility for debugging user sessions, without disturbing the existing Sentry pipeline. Two concrete outcomes:

1. **Backend OTLP logs into PostHog** — every API call's method, path, status, duration, request_id, and user_id flows to PostHog Logs so we can reconstruct a user's session timeline server-side.
2. **Frontend PostHog product events** — identify users, record page/screen views and a small curated set of product events (goal CRUD, sync outcomes, debug-report submission, etc.) so we can answer "what is anyone actually doing in this app."

Sentry stays as-is for unhandled-error capture, breadcrumbs, and source-mapped stack traces (see `frontend/src/lib/diagnostics/sentry.ts`). PostHog and Sentry are optimizing for different jobs and we deliberately keep both.

## Non-goals

- **Distributed tracing / spans.** PostHog Logs does not ingest OTLP traces, only logs. If we later want spans across the Go backend, that's a separate destination (Honeycomb / Grafana Tempo / Dash0).
- **Session replay** — deferred. Privacy review and a replay-specific masking pass would be needed first.
- **Feature flags / experiments** — deferred. PostHog supports them; not blocking analytics rollout.
- **Removing Sentry.** Out of scope.
- **Anonymous (pre-login) tracking.** We stick to `identified_only` to match the privacy stance already established with Sentry.

## Approach

```
                    ┌──────────────── Frontend (Svelte + Capacitor webview) ────────────────┐
                    │                                                                        │
   user action ───► │   posthog-js (identify on auth, capture curated events, no autocap.)  │ ──► PostHog Events
                    │                                                                        │
                    │   wrapFetch() net breadcrumbs ──► (optional Phase 4) OTLP logs        │ ──► PostHog Logs
                    │                                                                        │
                    │   Sentry SDK (unchanged) ──► Sentry.io                                 │
                    └────────────────────────────────────────────────────────────────────────┘

                    ┌──────────────── Backend (Go, chi router) ─────────────────────────────┐
                    │                                                                        │
   HTTP request ──► │   requestLogger middleware (existing) ──► slog                        │
                    │                                              │                         │
                    │                                              ▼                         │
                    │                                  slog → OTel logs bridge              │
                    │                                              │                         │
                    │                                              ▼                         │
                    │                              OTLP/HTTP exporter (otlploghttp)         │ ──► PostHog Logs (OTLP)
                    └────────────────────────────────────────────────────────────────────────┘
```

**Why PostHog for both jobs**: a single destination correlates a user's product events with the backend log lines for their session, keyed off `user_id` / `request_id`. This is the visibility win we don't get from Sentry today.

**Why not the frontend net breadcrumbs as PostHog *events***: high volume, low signal per event, and would inflate the event-tier cost while the same data is already going to Sentry breadcrumbs. If we want them shipped, OTLP **logs** from the browser is the right channel (Phase 4) — logs are cheaper and that's where this kind of trace-y data belongs.

## Phases

Each phase is independently shippable and reversible.

### Phase 1 — Backend OTLP logs → PostHog ✅ SHIPPED (`9c3e8a4`)

**Goal:** every existing slog line — including the per-request `Logger.Info("request", …)` in `backend/internal/api/router.go:290` — flows to PostHog Logs.

Concretely:

1. Add Go deps (`go get`):
   - `go.opentelemetry.io/otel`
   - `go.opentelemetry.io/otel/log`
   - `go.opentelemetry.io/otel/sdk/log`
   - `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`
   - `go.opentelemetry.io/contrib/bridges/otelslog`
2. In `backend/cmd/server/main.go`, before `initLogger`, build an OTel `LoggerProvider` with:
   - OTLP/HTTP log exporter pointed at PostHog's OTLP endpoint (env-driven, see Configuration below).
   - Batching log processor.
   - Resource attrs: `service.name=goal-tracker-backend`, `service.version=<git sha or version>`, `deployment.environment=<env>`.
3. In `backend/internal/api/router.go:initLogger`, attach the `otelslog` bridge as an additional slog handler so output goes to **both** stdout (unchanged) and OTel logs. Use a `slog.NewMultiHandler`-style fan-out (write a small `multiHandler` if the stdlib lacks one; trivial).
4. Register a graceful shutdown for the LoggerProvider in main so buffered logs flush on SIGTERM (Fly sends SIGTERM before kill).
5. Gate the OTel bridge on env: if `POSTHOG_PROJECT_TOKEN` is unset, the bridge is not installed and behavior is identical to today. Keeps dev + tests clean (mirrors the `VITE_SENTRY_DSN` gating pattern).
6. **Expose `request_id` on responses** — extend the request-id middleware (or a small response-header middleware right after it) to set `X-Request-Id` on every response. This lets the frontend pick it up from `fetch` responses in `wrapFetch` and attach it to PostHog events (Phase 3) and to any client-side OTel log records (Phase 4), so a single user-reported issue can be pivoted from a client event straight to the matching server log line.

**What you'll see in PostHog Logs after Phase 1**: one log record per request with attributes `method`, `path`, `status`, `duration`, `request_id`, `user_id`, plus error-level lines from session/cleanup workers. Filter by `user_id` to get any user's server-side session timeline.

### Phase 2 — Frontend PostHog SDK + identify ✅ SHIPPED (`13c08f4`)

**Goal:** users are identified, page views recorded, no custom events yet. Smallest possible footprint to validate wiring end-to-end.

1. `pnpm add posthog-js` in `frontend/`.
2. New file `frontend/src/lib/analytics/posthog.ts` mirroring the shape of `diagnostics/sentry.ts`:
   - `initPostHog()` — idempotent, reads `VITE_POSTHOG_KEY` + `VITE_POSTHOG_HOST`, no-op if unset, calls `posthog.init` with `person_profiles: 'identified_only'`, `autocapture: false`, `capture_pageview: true`, `capture_pageleave: true`, `disable_session_recording: true`, `loaded` hook that sets a `before_send` for defense-in-depth PII stripping (same pattern as `stripPIIFromEvent`).
   - `setPostHogUser(userId: string | null)` — calls `posthog.identify(userId)` on sign-in, `posthog.reset()` on sign-out. No email, no name, no other PII — same posture as `setSentryUser`.
3. Wire into `frontend/src/lib/diagnostics/bootstrap.ts` after `initSentry()` and before `patchConsole()`. Wrap in try/catch — bootstrap must never break on analytics failure (same defensive pattern already in place for Sentry).
4. In `frontend/src/App.svelte`, alongside the existing `setSentryUser(user.id)` / `setSentryUser(null)` call sites (lines 520, 547, 563), add `setPostHogUser(...)`.
5. **CSP update** in `backend/internal/api/router.go:308` — add the PostHog EU ingest host to `connect-src`. The current rule is `connect-src 'self' https://*.sentry.io`; extend to include `https://eu.i.posthog.com` (and `https://eu-assets.i.posthog.com` if `posthog-js` lazy-loads any assets from there at runtime — verify during implementation).
6. **Privacy policy update** — add PostHog as a sub-processor with what we send (user id, event names, basic device fields) and what we don't (no email, no goal content).

### Phase 3 — Curated product events ✅ SHIPPED (`13c08f4`)

**Goal:** answer questions like "do users finish onboarding," "how often does sync fail," "are debug reports actually submitted."

Define a small initial taxonomy (event name + property keys, no free-text):

- `goal_created` — `{ goal_id, has_reminder, recurrence_kind }`
- `goal_completed` — `{ goal_id }`
- `goal_deleted` — `{ goal_id }`
- `view_switched` — `{ goal_id, view: 'month' | 'week' }`
- `sync_completed` — `{ ok, duration_ms, items_pushed, items_pulled }`
- `sync_failed` — `{ reason_code }` (codes, not messages)
- `notification_permission_changed` — `{ state }`
- `debug_report_submitted` — `{ size_bytes }`
- `shake_to_report_triggered` — `{}` (validates the gesture is being discovered)

**Hard rule:** event names and property keys are fixed strings; goal *names* and other user-generated content never leave the device in PostHog events. Enforced two ways (belt-and-braces, mirroring the Sentry pattern of emitter-scrub + `beforeSend`-scrub):

1. **TypeScript** — a single `Events` map type defines the allowed property shape per event name; `capture<K extends keyof Events>(event: K, props: Events[K])` gives autocomplete and edit-time errors.
2. **Runtime whitelist (authoritative)** — the `capture()` wrapper holds a per-event allowlist of property keys and filters the outgoing object to those keys only. A spread of a full domain object (`{ ...goal, goal_id: goal.id }`) cannot leak — anything not on the allowlist is dropped at the network boundary.

Emit from the existing stores/actions in `frontend/src/lib/stores.ts` and the sync layer.

### Phase 4 — Frontend OTLP logs (optional) — DEFERRED

**Goal:** ship the existing `net` breadcrumbs (already produced by `wrapFetch` in `frontend/src/lib/diagnostics/net.ts:96`) to PostHog Logs as OTLP records, so we have client-side request timelines next to server-side ones.

Approach:

- Add `@opentelemetry/api-logs` + `@opentelemetry/sdk-logs` + `@opentelemetry/exporter-logs-otlp-http`.
- In `bootstrap.ts`, after Sentry init, build a browser `LoggerProvider` pointed at the same PostHog OTLP endpoint (the project token authorizes both).
- Subscribe to the same scrubbed breadcrumb stream the Sentry forwarder uses (`subscribe` in `frontend/src/lib/diagnostics/breadcrumbs.ts`) and emit each `net` crumb as an OTel log record. Reuse the scrubbing — do not introduce a second PII path.
- Sample aggressively for `2xx` requests (e.g. 10%); always send `4xx`/`5xx`/network errors. Keeps bundle weight and log volume bounded.

Defer this until Phase 1 + 3 are live and we know whether the server-side picture is enough. Bundle-size cost is real (~30-50 KB gzipped for the OTel browser SDK) and we should only pay it if it earns its keep.

### Phase 5+ — Deferred

- **Feature flags** — would unlock progressive rollout for things like notifications UX changes.
- **Session replay** — needs a masking-rules pass and a privacy-policy revision; not for this round.
- **Funnels / retention dashboards** — these come for free once events are flowing; just dashboard work, no code.

## Configuration

Env vars (added to Fly secrets for prod; left unset for local/dev/CI to no-op):

**Backend:**
- `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` — `https://eu.i.posthog.com/i/v1/logs` (per PostHog Go install docs; path is region-independent, only host differs between US/EU).
- `OTEL_EXPORTER_OTLP_LOGS_HEADERS` — `Authorization=Bearer <project-token>` (PostHog accepts the token in either header or query param; header is cleaner).
- `OTEL_SERVICE_NAME` — `goal-tracker-backend`.
- `OTEL_RESOURCE_ATTRIBUTES` — `deployment.environment=prod,service.version=<release>`.

**Frontend (Vite):**
- `VITE_POSTHOG_KEY` — project token.
- `VITE_POSTHOG_HOST` — `https://eu.i.posthog.com`.

If the analytics envs are absent everything degrades to today's behavior — same gating contract as Sentry.

## Privacy & PII rules

Single source of truth for what leaves the device:

| Channel | Identifier | What's allowed | What's blocked |
|---|---|---|---|
| Sentry (existing) | user id | error events, scrubbed breadcrumbs | email, goal name, free text |
| PostHog Events | user id | event name + whitelisted property keys (IDs, enums, counts, durations) | email, goal name, free text |
| PostHog Logs (backend) | user_id attr | method, path, status, duration, request_id, error codes | request bodies, query params with PII, goal names |
| PostHog Logs (frontend, Phase 4) | user id | method, parsed `path` (no query), status, duration | full URLs with query, request/response bodies |

Two enforcement points, mirroring the existing Sentry hygiene:
1. **Source-side scrubbing** at the emitter (already in place for breadcrumbs; extend to PostHog events via the whitelist wrapper).
2. **Defense-in-depth `before_send`** in `posthog.ts` that strips known-bad fields if anything slips through.

Update `docs/privacy-policy.md` and `frontend/public/privacy.html` in Phase 2.

## Testing

- **Backend:** unit test that the OTel bridge is *not* installed when `POSTHOG_PROJECT_TOKEN` is absent. Smoke test in staging that a known request produces a log record in PostHog with the expected attrs. No tests should hit PostHog's real endpoint.
- **Frontend:** mirror `frontend/src/lib/__tests__/sentry.test.ts` — `posthog.test.ts` covering: no-init without env, init is idempotent, `before_send` strips disallowed fields, `setPostHogUser(null)` calls `reset`, the `capture()` wrapper drops non-whitelisted property keys.
- **CSP regression:** add a Playwright check that fetching `https://*.posthog.com` from the page does not violate CSP (the existing E2E has CSP coverage; extend it).

## Setup checklist (one-time, outside the codebase)

These are the human steps that bracket the code work. None of them are reversible damage, but skipping any of them means the code from Phases 1–3 silently no-ops in the affected environment.

### PostHog account

1. Create a PostHog **EU** project (`eu.posthog.com`). One project covers all environments; events and logs are tagged with an `environment` property (`prod`, `dev`, `ci-build`) for filtering. Separate projects per env aren't worth the management overhead at this scale.
2. Capture the project token (looks like `phc_…`).
3. **Source-map upload**: skip. Sentry already symbolicates frontend stack traces; PostHog source-map upload is for Error Tracking, which this plan does not adopt. No PostHog auth token / org / project secrets are needed.

### Backend (Fly) — Phase 1 enablement

Set as Fly app secrets (run from a shell with `flyctl` auth — do not commit, do not put in CI):

```
flyctl secrets set \
  POSTHOG_PROJECT_TOKEN='phc_…' \
  OTEL_EXPORTER_OTLP_LOGS_ENDPOINT='https://eu.i.posthog.com/i/v1/logs' \
  OTEL_EXPORTER_OTLP_LOGS_HEADERS='Authorization=Bearer phc_…' \
  OTEL_SERVICE_NAME='goal-tracker-backend' \
  OTEL_RESOURCE_ATTRIBUTES='deployment.environment=prod'
```

The two `phc_…` values are the same token (one in `POSTHOG_PROJECT_TOKEN` for any direct backend use, one in the OTLP `Authorization` header). The PostHog Go install docs show only the US host; the EU host follows their standard `us → eu` substitution.

### Frontend (GitHub Actions) — Phase 2 enablement

`VITE_POSTHOG_KEY` is baked into the JS bundle at build time (same model as `VITE_SENTRY_DSN`), so it must be present as a GitHub Actions secret and passed into both build stanzas in `.github/workflows/android-build.yml`.

1. Add repo-level GitHub Actions secrets:
   - `VITE_POSTHOG_KEY` — the same `phc_…` project token.
   - `VITE_POSTHOG_HOST` — `https://eu.i.posthog.com`. (Could be a hardcoded literal in the workflow instead of a secret; secret is consistent with how `VITE_SENTRY_DSN` is handled.)
2. Add both vars to the `env:` blocks of both build stanzas in `android-build.yml` (around lines 83-87 and 209-213, alongside the existing `VITE_SENTRY_DSN`).
3. Decide the dev/CI-build posture: same as Sentry today — both the CI debug build and the release build ship with the key set. Distinguish at query time via the `environment` super-property (`ci-build` for debug AAB, `prod` for tagged release AAB). Set this in `posthog.ts` init by reading `import.meta.env.MODE` (or a build-time flag the workflow sets explicitly).

### Capacitor / Android app

No Android-native PostHog SDK. The frontend is a webview; `posthog-js` runs inside it and the token is in the bundle. The only Android-side change is the CSP update in `router.go:308` (covered in Phase 2) — PostHog ingest from the webview goes through the same CSP as Sentry.

### Privacy & legal

1. Update `docs/privacy-policy.md` and `frontend/public/privacy.html` to add PostHog as a sub-processor (EU region) before the release that first ships `VITE_POSTHOG_KEY` set.
2. If a Play Store data-safety form is on file, update it to disclose analytics (event names, user id) and the PostHog destination.

## Rollout

1. Phase 1 lands behind missing env vars (no-op). Set Fly secrets in a separate step — flip on with a config push, not a code deploy. Watch PostHog Logs for ~24h.
2. Phase 2 ships behind a missing `VITE_POSTHOG_KEY`. Cut a release with the key set after privacy-policy update has been live for a release cycle.
3. Phase 3 incremental — add events one or two at a time so dashboards can be built and validated against known activity.
4. Phase 4 only if Phase 1 logs leave a visibility gap on the client side.

If PostHog becomes a problem (cost, outage, privacy concern), unsetting the two project envs in Fly + a frontend release with `VITE_POSTHOG_KEY` cleared reverts all PostHog behavior. No data model in our DB depends on PostHog.

## Decisions

- **Region**: PostHog **EU** (`eu.i.posthog.com`). CSP and OTLP endpoint reflect this throughout.
- **`request_id` cross-side correlation**: backend sets `X-Request-Id` on responses (Phase 1, step 6); frontend reads it in `wrapFetch` and attaches it to PostHog events / logs as a property.
- **User identification**: PostHog is initialized with `person_profiles: 'identified_only'`. `posthog.identify(userId)` fires from the same App.svelte sites that already call `setSentryUser` (lines 520, 547, 563); `posthog.reset()` on sign-out. ID only — no email, no name.
- **Property whitelist enforcement**: both TypeScript typing *and* a runtime allowlist in the `capture()` wrapper. The runtime check is authoritative — TS catches mistakes at edit time but cannot cover spread-of-domain-object footguns.
- **Backend log sampling**: none initially. The 50 GB/month free tier covers current traffic with room to spare; revisit only if usage approaches the limit.

## Follow-ups (post-ship, 2026-05-23)

Non-blocking items surfaced during implementation/review or in-flight testing. None of these block the rollout; group them into a follow-up PR (or several) when the time comes.

### Schema / event taxonomy

- **`sync_completed.items_pulled` is misnamed.** The sync layer is push-only; the field actually holds the count of events the server accepted from our push. Either rename to `items_accepted` in `Events`/`EVENT_ALLOWLIST` and the call site, or document the meaning on the PostHog event description so dashboards aren't misread.
- **`goal_deleted` event fires from `archiveGoal`** (soft delete). If a true hard-delete is ever added, the taxonomy will need split or rename. Acceptable for now since archive is the only user-facing delete.
- **`notification_permission_changed` coverage gap.** Fires only from `requestPermission()`. The resume-listener path in `frontend/src/lib/components/NotificationSettings.svelte` (around lines 97-106) detects the deny → OS settings → grant → return-to-app transition, but does not emit the event. If this transition matters for analytics, wire it.

### Backend hygiene

- **Per-request slog message body is the literal `"request"`.** Low-signal in PostHog Logs at a glance. Make it `method path status` (e.g. `"GET /api/goals 200"`) — keep all existing attrs unchanged. Located at `backend/internal/api/router.go:353`.
- **Dedicated shutdown context for OTel flush.** `loggerProvider.Shutdown` currently shares the 10s `server.Shutdown` budget. If HTTP drain takes most of it, log flush starves. Use `context.WithTimeout(context.Background(), 5*time.Second)` for the OTel flush.
- **`multiHandler.Handle` swallows OTel handler errors silently.** Return the first non-nil error so failures surface in tests / future debugging.
- **No build-time `service.version`.** Wire an `ldflags -X main.version=…` set from `git describe` (or the version computed in `android-build.yml`) and pass it as a resource attribute on the OTel `LoggerProvider`.
- **Startup/shutdown `log.Printf(...)` calls in `backend/cmd/server/main.go` don't flow through slog**, so they won't appear in PostHog Logs. Consider migrating to `slog.Info` if you want them in the same feed.

### Frontend hygiene

- **Unused import in `frontend/src/lib/api.ts`**: `loadNotificationSettings` was used by the previous `goal_created` async path; the simplified call site no longer references it. Remove on next touch.
- **`disable_external_dependency_loading: true`** is set in `posthog.init`. If you ever want to use PostHog Toolbar, Surveys, or Web Vitals, this will need to flip and CSP will need `https://eu-assets.i.posthog.com`. Document in onboarding.

### Tests

- **Backend `TestOTelBridge_NotInstalledWithoutToken` is weak** — currently asserts /health 200, which would pass even if the bridge were installed. Strengthen by inspecting `api.Logger`'s handler type, or expose a small `isOTelInstalled() bool` accessor.
- **No "bridge-on" path test** for the multiHandler fan-out. `go.opentelemetry.io/otel/sdk/log/logtest` is already in `go.sum` and would make this a small addition.

### Privacy / operational

- **Play Store data-safety form** — if a form is on file, update it to disclose analytics (event names, user id) and PostHog destination. (Plan called this out; doing it post-ship once analytics is verified in production.)
- **CSP / VITE_POSTHOG_HOST / Fly OTLP endpoint region must stay in sync.** All three are hardcoded EU. A US move would need lockstep changes — comment is in place at `router.go:382`.

## Exit points

- **Just want server-side visibility?** Stop after Phase 1.
- **Want product analytics too?** Add Phase 2 + 3.
- **Want full client-side request telemetry?** Phase 4.
- **Decide PostHog isn't the right fit?** Unset envs to revert; no schema changes to undo.
