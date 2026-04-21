// Phase 7 Sentry wiring. Parallel to the custom debug-report path:
// Sentry is the auto-capture path for unhandled errors, while the custom
// user-initiated path (shake → DebugReportModal → POST) stays untouched.
//
// Design invariants (see docs/plans/2026-04-14-debug-log-collection-design.md):
//   - Breadcrumbs flow from our scrubbed emitter (Phase 3) to Sentry via
//     `subscribe()` — Sentry's own auto-capture integrations are disabled.
//   - User context is ID-only. No email ever leaves the app through Sentry.
//   - No DSN → no init, no network, no errors. Keeps dev + e2e clean.
//   - `beforeSend` is a defense-in-depth strip of user-generated fields in
//     case something slipped past the emitter scrubbing.

import * as Sentry from '@sentry/capacitor';
import type { SeverityLevel } from '@sentry/capacitor';
import * as SentrySvelte from '@sentry/svelte';

import { subscribe, type Breadcrumb } from './breadcrumbs';

// Local minimal shapes so we don't depend on the exact `ErrorEvent` /
// `Integration` type paths, which differ between the @sentry/capacitor and
// @sentry/svelte dual install and would force cross-module structural casts.
type IntegrationLike = { name: string };

/**
 * Map the emitter's breadcrumb level to Sentry's canonical `SeverityLevel`.
 * Our emitter uses `'warn'` while Sentry's union is
 * `'fatal' | 'error' | 'warning' | 'log' | 'info' | 'debug'` — `'warn'` is
 * NOT a member, so we translate it to `'warning'`. `Sentry.addBreadcrumb`
 * does not normalize, and sending `'warn'` verbatim risks being dropped
 * server-side.
 */
function mapLevel(level: 'info' | 'warn' | 'error'): SeverityLevel {
  return level === 'warn' ? 'warning' : level;
}

let initialized = false;
let hasDsn = false;
let unsubscribeForwarder: (() => void) | undefined;

/**
 * Read the DSN from Vite env. Returns undefined (rather than empty string)
 * when unset so callers can use a single truthy check.
 */
function readDsn(): string | undefined {
  const dsn = import.meta.env.VITE_SENTRY_DSN;
  if (typeof dsn !== 'string' || dsn.length === 0) return undefined;
  return dsn;
}

/**
 * `beforeSend` is invoked for every event Sentry is about to send. This is
 * defense-in-depth: even though our emitter scrubs everything upstream, a
 * Sentry integration we don't control could still attach raw fields.
 * Exported for unit testing.
 */
export function stripPIIFromEvent<T extends Record<string, unknown>>(
  event: T,
): T {
  // Remove goal_name if it snuck into extras.
  const extra = event.extra as Record<string, unknown> | undefined;
  if (extra && 'goal_name' in extra) {
    delete extra.goal_name;
  }
  // Coerce user down to ID only — never ship email, ip_address, or username.
  const user = event.user as { id?: string | number } | undefined;
  if (user) {
    (event as Record<string, unknown>).user =
      user.id !== undefined ? { id: user.id } : {};
  }
  return event;
}

/**
 * Initialize Sentry. Idempotent: the first call with a DSN wires everything;
 * later calls no-op. If `VITE_SENTRY_DSN` is unset/empty, logs a single info
 * line and returns without touching Sentry.
 */
export function initSentry(): void {
  if (initialized) return;
  initialized = true;

  const dsn = readDsn();
  if (!dsn) {
    // Intentional: one informational line so a dev sees why Sentry isn't
    // reporting. The info level stays out of console.warn/error telemetry.
    console.info('Sentry disabled (no DSN)');
    return;
  }
  hasDsn = true;

  const release =
    typeof import.meta.env.VITE_APP_VERSION === 'string' &&
    import.meta.env.VITE_APP_VERSION.length > 0
      ? import.meta.env.VITE_APP_VERSION
      : 'dev';

  // The Sentry options type comes from @sentry/capacitor's private bundle,
  // and TypeScript's bivariant checks for the `integrations`/`beforeSend`
  // callbacks can't see past the dual @sentry/core install in this monorepo.
  // Build the options as a loose record and cast once — the runtime shape is
  // exactly what Sentry expects (see @sentry/capacitor CapacitorOptions).
  const options = {
    dsn,
    environment: import.meta.env.MODE,
    release,
    tracesSampleRate: 0,
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,
    // Keep ONLY the unhandled-error capture. Every other auto-capture
    // integration (breadcrumbs, console, fetch, history, …) is dropped —
    // those channels feed from our emitter via the forwarder below, after
    // PII scrubbing.
    integrations: (defaults: IntegrationLike[]) =>
      defaults.filter((i) => i.name === 'GlobalHandlers'),
    beforeSend: (event: Record<string, unknown>) => stripPIIFromEvent(event),
  };
  Sentry.init(
    options as unknown as Parameters<typeof Sentry.init>[0],
    SentrySvelte.init as unknown as Parameters<typeof Sentry.init>[1],
  );

  // Forward every scrubbed crumb into Sentry. `subscribe` returns an
  // unsubscribe handle; we store it so test helpers can tear down cleanly.
  unsubscribeForwarder = subscribe((crumb: Breadcrumb) => {
    try {
      Sentry.addBreadcrumb({
        category: crumb.category,
        level: mapLevel(crumb.level),
        message: crumb.message,
        data: crumb.data,
        // Sentry expects timestamp in seconds, not ms.
        timestamp: crumb.ts / 1000,
      });
    } catch {
      // Never let Sentry breadcrumb forwarding break the emitter.
    }
  });
}

/**
 * Set the Sentry user context after auth. ID only; passing `null` clears it.
 * No-op when Sentry isn't initialized (no DSN). Safe to call from auth
 * transitions without checking DSN presence first.
 */
export function setSentryUser(userId: string | null): void {
  if (!hasDsn) return;
  if (userId === null) {
    Sentry.setUser(null);
    return;
  }
  Sentry.setUser({ id: userId });
}

// ---------- Test helpers ----------

/**
 * Reset internal state so `initSentry()` can be called again in tests.
 * Not part of the public API.
 */
export function __resetSentryForTest(): void {
  if (unsubscribeForwarder) {
    try {
      unsubscribeForwarder();
    } catch {
      // ignore
    }
  }
  unsubscribeForwarder = undefined;
  initialized = false;
  hasDsn = false;
}
