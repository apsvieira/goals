# Debug log collection — design

**Date:** 2026-04-14
**Status:** design validated, ready for implementation plan

## Goal

Let us collect diagnostic information from user devices, both user-initiated (shake to report a problem) and automatic (unhandled errors). Reports must identify the reporting user so we can follow up, and include enough device and app context to diagnose issues without requiring the user to describe their environment.

## Approach: hybrid (Sentry + custom shake-to-send)

Two parallel paths from the device to us:

- **Auto-capture path → Sentry.** Unhandled JS errors, promise rejections, and Android native (JVM) crashes are captured by `@sentry/capacitor`. Sentry handles source maps, grouping, dedup, and the dashboard.
- **User-initiated path → our backend.** A shake gesture opens a form where the user describes the problem; the form + a ring buffer of recent logs + app state + device info is POSTed to a new `/api/v1/debug-reports` endpoint and stored in Postgres.

**Why hybrid:** Sentry gets us best-in-class error telemetry for effectively zero code. The custom path captures *user-reported issues that aren't errors* ("sync seems stuck", "goals disappeared") — something Sentry's User Feedback flow doesn't model well. The custom path is also the vendor-independence hedge: if Sentry ever becomes cost-prohibitive, we already own the storage, transport, and admin tooling for diagnostic reports — we just add auto-error capture to the same path (see *Exit points*).

## Architecture

```
              ┌──────────────────── Frontend (Svelte + Capacitor) ────────────────────┐
              │                                                                        │
   events ──► │   Breadcrumb emitter ──► Ring buffer (memory + flush-on-pause)        │
   (log,      │         │                                                              │
   nav,       │         │ forwards (scrubbed)                                          │
   action,    │         ▼                                                              │
   net,       │   Sentry SDK ──► Sentry.io (scrubbed, IDs only)                       │
   sync)      │                                                                        │
              │                                                                        │
              │   Shake detector ──► DebugReportModal ──► payload builder              │
              │                                                       │                │
              │                                                       ▼                │
              │                                          Offline queue (IndexedDB)    │
              │                                                       │                │
              └───────────────────────────────────────────────────────┼────────────────┘
                                                                      ▼
                                          POST /api/v1/debug-reports (Go + Postgres, 90-day retention)
                                                                      │
                                                                      ▼
                                                         CLI viewer (backend/cmd/debug-reports)
```

**Key invariant:** the breadcrumb emitter is the single choke point where app events fan out to (a) our ring buffer and (b) Sentry. PII scrubbing happens inside `emit()`, so no downstream consumer ever sees unscrubbed user-generated content. Removing Sentry later is one deleted forwarder.

## Frontend components

### `frontend/src/lib/diagnostics/breadcrumbs.ts`

Pure module owning the ring buffer and emitter API.

```typescript
type BreadcrumbCategory =
  | 'log'       // console.log/info/warn/error (captured via patching)
  | 'nav'       // route changes
  | 'action'    // user actions (goal created, completed, deleted)
  | 'sync'      // sync start/end/error
  | 'auth'      // login, logout, session expiry
  | 'net';      // fetch/XHR summary (method, path, status, duration)

type Breadcrumb = {
  ts: number;           // epoch ms
  category: BreadcrumbCategory;
  level: 'info' | 'warn' | 'error';
  message: string;      // short, pre-scrubbed
  data?: Record<string, unknown>;  // pre-scrubbed, max 512 B after serialization
};

function emit(b: Breadcrumb): void;
function snapshot(): Breadcrumb[];  // for shake payload
```

**Ring buffer policy:** fixed-size array capped at **500 entries** *and* **5 minutes** (whichever trims first). In-memory during normal use; on `visibilitychange → hidden`, Capacitor `App.addListener('pause')`, or `beforeunload`, the buffer is serialized to IndexedDB under `diagnostics_buffer`. On app startup, it's restored into memory before emit() is called. This guarantees the buffer survives JS crash + app kill + reload.

**Log capture:** patch `console.log/info/warn/error` at app bootstrap. Patched versions call the original and emit a `log` breadcrumb. `window.onerror`, `unhandledrejection`, and failing `.catch` paths additionally forward to Sentry.

**PII scrubbing inside `emit()`** (single choke point):

- Strip `Authorization` / `Cookie` headers from `net` data.
- Regex-scrub emails → `[email]` and bearer tokens → `[token]` from any string field.
- For `action` category: replace `goal_name` with `goal_id`; drop completion notes.
- OAuth `code=` / `state=` URL params → `[oauth]`.

**Sentry forwarder:** a small listener registered separately from the emitter module. Subscribes to `emit()` and calls `Sentry.addBreadcrumb()` with the already-scrubbed data. The emitter has no knowledge of Sentry.

### `frontend/src/lib/diagnostics/shake.ts`

Uses `@capacitor/motion`. Computes acceleration magnitude `√(x² + y² + z²) − 9.8` (gravity-subtracted) on every sample, requires **three peaks above 15 m/s² within 1 second**, then fires. Thresholds match Android's `ShakeDetector` heuristic and will need on-device tuning. Detection is paused when:

- The shake modal is already open.
- The app is in background / paused.

### `frontend/src/lib/components/DebugReportModal.svelte`

```
┌──────────────────────────────────┐
│  Report a problem                │
│                                  │
│  What's going wrong?             │
│  ┌────────────────────────────┐  │
│  │ (optional, free text, 2KB) │  │
│  │                            │  │
│  └────────────────────────────┘  │
│                                  │
│  This will send recent app logs  │
│  and your device info so we can  │
│  investigate. Your goal data     │
│  is not included.                │
│                                  │
│         [Cancel]  [Send]         │
└──────────────────────────────────┘
```

- Description: `<textarea>`, optional, 2000 char cap, auto-focus on open.
- On submit: build payload → `POST /api/v1/debug-reports`. Success → toast "Thanks, report sent" + close. Network failure → write to IndexedDB `debug_report_queue`, toast "Saved — will send when online".
- **Client-side rate limit:** `localStorage['last_debug_report_ts']`. If <60 s since last, modal opens but `[Send]` is disabled with inline message: *"You just sent a report a moment ago — you can send another in a few seconds."*
- **Server-side 429** returns the same friendly text, which the modal shows instead of a generic error.

### Offline queue

Modelled on existing `event-sync.ts` pending-sync pattern. Queue is drained by the existing `isOnline` store — when it flips to `true`, pending reports are POSTed in order. Capped at 10 queued reports (prevents runaway growth if shake is held/pocketed).

### Auto-error path

Handled entirely by `@sentry/capacitor`. No custom code — unhandled errors, promise rejections, and native Android crashes are all SDK defaults. Auto-captured errors also produce a breadcrumb via the forwarder, but don't hit our backend.

## Sentry configuration

**Install:** `npm i @sentry/capacitor @sentry/svelte`. The capacitor package installs a small Java shim via `npx cap sync` that captures JVM crashes from plugins and Capacitor internals.

**Initialization** (`main.ts`, before any other app code):

```typescript
import * as Sentry from '@sentry/capacitor';
import * as SentrySvelte from '@sentry/svelte';

Sentry.init({
  dsn: import.meta.env.VITE_SENTRY_DSN,
  environment: import.meta.env.MODE,
  release: import.meta.env.VITE_APP_VERSION,
  tracesSampleRate: 0,
  replaysSessionSampleRate: 0,
  replaysOnErrorSampleRate: 0,
  // Our breadcrumb emitter forwards scrubbed breadcrumbs; disable Sentry's
  // auto-capture to avoid double-capture and unscrubbed data leakage.
  // Keep only GlobalHandlers (unhandled-error capture) active.
  integrations: (defaults) =>
    defaults.filter((i) => i.name === 'GlobalHandlers'),
  beforeSend(event) {
    // Defense-in-depth: strip user-generated fields even if something slipped
    // past the emitter scrubbing.
    if (event.extra?.goal_name) delete event.extra.goal_name;
    if (event.user) event.user = { id: event.user.id };
    return event;
  },
}, SentrySvelte.init);
```

**User context:** after auth, call `Sentry.setUser({ id: userId })` — **ID only, no email**. To find a user's email from a Sentry event, cross-reference to our Postgres.

**DSN handling:** `VITE_SENTRY_DSN` set as GitHub Actions secret for CI builds, `.env.local` for dev. If empty, `Sentry.init` is skipped entirely — no network calls, no errors. Keeps local dev and e2e tests clean.

**Source maps:** upload via `@sentry/vite-plugin` on CI builds only. Needs `SENTRY_AUTH_TOKEN` (org-scoped, read-write on project). Without source maps, production stack traces are unreadable.

**Android native:** handled by the Capacitor SDK after `npx cap sync`. No additional Gradle config needed for basic crash capture. ProGuard/R8 mapping upload would be needed if we enabled minification — currently we don't.

## Backend

### New endpoint: `POST /api/v1/debug-reports`

Mounted under the existing auth-required group in `router.go`. Body capped at **256 KB** by per-route `http.MaxBytesReader` (tighter than the 1 MB global).

**Request body:**

```json
{
  "client_id": "uuid-v4-generated-on-first-launch",
  "app_version": "0.4.1",
  "platform": "android" | "ios" | "web",
  "device": { "model": "Pixel 8", "os": "Android 14", "webview": "Chrome/127" },
  "state": {
    "route": "home",
    "online": true,
    "pending_events": 0,
    "goal_count": 7,
    "auth_state": "authenticated",
    "notif_permission": "granted"
  },
  "description": "my morning run goal completion from yesterday vanished",
  "breadcrumbs": [ /* array of Breadcrumb objects, same shape as frontend */ ],
  "trigger": "shake" | "auto",
  "client_ts": 1733356800000
}
```

`user_id` is **not** in the body — it's read from the authenticated session in the handler, same pattern as `/goals`. Prevents a client from forging reports as another user.

### Rate limiter

Reuse existing `RateLimiter` struct. Add two limiters, both keyed by `user_id` (not IP) for fairness across devices:

- `debugReportHourly := NewRateLimiter(5, time.Hour)`
- `debugReportDaily := NewRateLimiter(20, 24*time.Hour)`

Requires a small addition to `ratelimit.go` to support user-keyed limiting — swapping the IP extraction for user extraction when the middleware runs inside the authenticated group.

### Postgres schema

New migration `0XX_debug_reports.sql`:

```sql
CREATE TABLE debug_reports (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id   UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trigger     TEXT NOT NULL CHECK (trigger IN ('shake','auto')),
    app_version TEXT NOT NULL,
    platform    TEXT NOT NULL,
    device      JSONB NOT NULL,
    state       JSONB NOT NULL,
    description TEXT,
    breadcrumbs JSONB NOT NULL
);

CREATE INDEX idx_debug_reports_user_created ON debug_reports (user_id, created_at DESC);
CREATE INDEX idx_debug_reports_created      ON debug_reports (created_at);
```

JSONB for flexible schema evolution. `ON DELETE CASCADE` ensures reports are purged when a user deletes their account — matches existing `DeleteAccount` behavior.

### Cleanup goroutine

Extend existing `StartSessionCleanup` pattern with `StartDebugReportsCleanup(ctx, 24*time.Hour)` that runs `DELETE FROM debug_reports WHERE created_at < NOW() - INTERVAL '90 days'` once per day.

### Database interface additions

Added to `db.Database`:

```go
CreateDebugReport(report *models.DebugReport) error
ListDebugReports(filter DebugReportFilter) ([]models.DebugReport, error)
GetDebugReport(id string) (*models.DebugReport, error)
DeleteOldDebugReports(olderThan time.Time) (int64, error)
```

## CLI viewer

`backend/cmd/debug-reports/main.go`:

```bash
go run ./cmd/debug-reports list --user email@x --since 7d    # list summaries
go run ./cmd/debug-reports view <report-id>                  # pretty-print payload
go run ./cmd/debug-reports purge --older-than 90d            # manual cleanup
```

- `list` output columns: `id | created_at | user_email | trigger | description_snippet | app_version`.
- `view` formats: header block (user, device, state), then chronological breadcrumb feed color-coded by level (info gray, warn yellow, error red).
- Connects via `DATABASE_URL`, reuses `db.Open()`. Single file, ~150 lines, no new dependencies.

## Privacy policy updates

Add to `docs/privacy-policy.md` under *Data We Collect*:

> **Debug Reports (optional):** If you shake your device to report a problem, the app sends us: your user ID, app version, device model, recent technical logs (with goal names replaced by internal identifiers), and your description of the problem. Debug reports are kept for 90 days and then automatically deleted.

Add under *Third-Party Services*:

> **Sentry (sentry.io):** For automated error reporting. When the app encounters an unexpected error, a technical report is sent to Sentry containing: your user ID (a random identifier — not your email), your app version, device model, operating system, and a stack trace of the error. Your goal names, completions, and any personal content are **not** sent to Sentry. Subject to [Sentry's Privacy Policy](https://sentry.io/privacy/).

Bump the *Last updated* date. CSP in `router.go` needs `connect-src` extended to include `https://*.sentry.io` (or the specific region host once the DSN is chosen).

## Testing strategy

- **Breadcrumb emitter (vitest unit).** Each PII scrub case (email regex, bearer token, OAuth URL param, goal_name in action data), buffer eviction (501st entry evicts oldest), persistence (flush to IDB on `visibilitychange` + restore on reload).
- **Shake detector (vitest unit).** Peak-detection algorithm tested with synthetic acceleration sequences: deliberate shake → fires, walking → doesn't fire, phone-in-pocket jostling → doesn't fire, single hard impact → doesn't fire.
- **Backend handler (Go table-test).** Happy path, 429 over hourly limit, 429 over daily limit, 413 over size cap, 401 unauthenticated, 400 on bad `trigger` value, 400 on missing `client_id`. Reuses existing `e2e_test.go` patterns and test DB setup.
- **Cleanup goroutine.** Add a row to the existing `StartSessionCleanup` test pattern: insert an 91-day-old report, trigger cleanup, assert it's deleted.
- **Manual QA checklist (pre-release).**
  - Shake opens modal; modal only opens on mobile; double-shake while open doesn't reopen.
  - Offline-shake → queued → report appears in backend after reconnecting.
  - Rate-limit message renders correctly when shaking twice within 60 s.
  - Sentry dev event appears in dashboard for a thrown exception in dev build.
  - Android native crash test: add a plugin that calls `throw new RuntimeException()` in debug build, confirm Sentry receives the crash.

## Out of scope for v1

Explicit list for future revisions:

- Screenshot attachment (Capacitor Screenshot plugin; permissions, PII redaction).
- Session replay (Sentry paid feature).
- Breadcrumb filtering by category in CLI.
- Admin web UI.
- iOS support (app is Android-first right now).
- Per-user debug-log retention opt-out.

## Exit points (replace-when-Sentry-costs-hurt)

Ordered by effort, so we can see how replaceable each piece is:

1. **Delete Sentry forwarder wiring** in `breadcrumbs.ts` (breadcrumbs still flow to ring buffer; Sentry no longer sees them). Zero user-visible impact.
2. **Delete `Sentry.init` in `main.ts`**, remove `@sentry/*` dependencies, remove `VITE_SENTRY_DSN` + `SENTRY_AUTH_TOKEN` secrets from CI, remove `@sentry/vite-plugin` from build. Auto-error capture is now off.
3. **Add a `window.onerror` + `unhandledrejection` listener** that builds a payload with `trigger='auto'` and POSTs to `/api/v1/debug-reports`. Existing endpoint, schema, rate limiter, storage, cleanup, and CLI viewer are all unchanged.
4. **Remove Sentry section from privacy policy**; bump *Last updated*.
5. **Rebuild Android** (`npx cap sync` + CI build) to drop the native Sentry shim.

Steps 1–4 are maybe half a day. Step 5 is automatic via CI.

Native Android JVM crashes are the one capability we lose by removing Sentry — if that starts mattering we'd add a Capacitor plugin to catch and forward native exceptions, but it's not worth pre-building.

## Implementation sequencing (suggested)

1. Backend: migration + `Database` methods + handler + user-keyed rate limiter + cleanup goroutine.
2. Backend: CLI viewer.
3. Frontend: `breadcrumbs.ts` emitter + ring buffer + PII scrubbing (pure, no Sentry, no UI).
4. Frontend: console patching + navigation/action/sync/auth breadcrumb instrumentation.
5. Frontend: `DebugReportModal.svelte` + offline queue + POST client.
6. Frontend: `shake.ts` detector + wire to modal.
7. Sentry: install + init + forwarder + source-map CI step + user-context hook.
8. CSP update + privacy policy update.
9. End-to-end manual QA on a real Android device.

Each step is independently testable; we can land them in separate PRs.
