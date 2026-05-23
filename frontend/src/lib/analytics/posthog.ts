// Phase 2 PostHog wiring. Parallel to the Sentry path — PostHog handles
// product analytics (identify + page views) while Sentry handles error capture.
// The two are independent and deliberately kept separate.
//
// Design invariants (see docs/plans/2026-05-23-posthog-analytics-and-logs.md):
//   - No key → no init, no network, no errors. Keeps dev + e2e clean.
//   - User context is ID-only. No email, no name, no goal content ever leaves
//     the device through PostHog.
//   - `before_send` is a defense-in-depth strip of user-generated fields in
//     case something slipped through — mirrors the Sentry `beforeSend` pattern.
//   - Idempotent: the first call with a key wires everything; later calls no-op.

import posthog, { type CaptureResult } from 'posthog-js';

let initialized = false;
let hasKey = false;

/**
 * Read the PostHog project key from Vite env. Returns undefined (rather than
 * empty string) when unset so callers can use a single truthy check.
 */
function readKey(): string | undefined {
  const key = import.meta.env.VITE_POSTHOG_KEY;
  if (typeof key !== 'string' || key.length === 0) return undefined;
  return key;
}

/**
 * Read the PostHog host from Vite env. Returns undefined when unset.
 */
function readHost(): string | undefined {
  const host = import.meta.env.VITE_POSTHOG_HOST;
  if (typeof host !== 'string' || host.length === 0) return undefined;
  return host;
}

/**
 * Derive the environment name for the PostHog `environment` super-property.
 * `VITE_APP_ENV` is set explicitly by the CI workflow for each build type
 * (`prod` for tagged release, `ci-build` for debug CI build). Falls back to
 * `import.meta.env.MODE` so local `vite dev` sessions appear as `development`.
 */
function deriveEnvironment(): string {
  const explicit = import.meta.env.VITE_APP_ENV;
  if (typeof explicit === 'string' && explicit.length > 0) return explicit;
  const mode = import.meta.env.MODE;
  if (typeof mode === 'string' && mode.length > 0) return mode;
  return 'dev';
}

/**
 * `stripPIIFromEvent` is invoked via `before_send` for every event PostHog is
 * about to send. Defense-in-depth: even though callers never attach PII, this
 * ensures no stray field reaches the network.
 * Exported for unit testing. Accepts a loose record so tests can pass plain
 * objects without constructing a full CaptureResult.
 */
export function stripPIIFromEvent<T extends { properties?: Record<string, unknown> }>(
  event: T,
): T {
  const props = event.properties;
  if (props) {
    // Strip goal_name if it snuck into properties.
    if ('goal_name' in props) delete props.goal_name;
    // Strip any email field.
    if ('email' in props) delete props.email;
    // Strip IP address — PostHog may auto-populate this.
    if ('$ip' in props) delete props.$ip;
    // Defense-in-depth: strip $set / $set_once person-profile payloads in case
    // posthog.people.set() is ever called inadvertently in a future phase.
    delete props.$set;
    delete props.$set_once;
  }
  return event;
}

/**
 * Initialize PostHog. Idempotent: the first call with a key wires everything;
 * later calls no-op. If `VITE_POSTHOG_KEY` or `VITE_POSTHOG_HOST` are
 * unset/empty, logs a single info line and returns without touching PostHog.
 */
export function initPostHog(): void {
  if (initialized) return;
  initialized = true;

  const key = readKey();
  const host = readHost();

  if (!key || !host) {
    console.info('PostHog disabled (no key/host)');
    return;
  }
  hasKey = true;

  try {
    posthog.init(key, {
      api_host: host,
      person_profiles: 'identified_only',
      autocapture: false,
      capture_pageview: true,
      capture_pageleave: true,
      disable_session_recording: true,
      // Prevent posthog-js from fetching assets from eu-assets.i.posthog.com.
      // With autocapture off and session recording disabled this path is never
      // needed; setting the flag makes that explicit and keeps the CSP tight.
      disable_external_dependency_loading: true,
      before_send: (event: CaptureResult | null): CaptureResult | null => {
        if (event === null) return null;
        return stripPIIFromEvent(event);
      },
    });
    // Register environment as a super-property synchronously after init so
    // that the first auto-$pageview already carries it. Vite uses
    // 'production' / 'development' for MODE; VITE_APP_ENV lets the CI
    // workflow set an explicit value ('prod', 'ci-build').
    posthog.register({ environment: deriveEnvironment() });
  } catch {
    // PostHog init must never break app bootstrap.
  }
}

/**
 * Set the PostHog user after auth. ID only; passing `null` resets the
 * anonymous identity on sign-out. No email, no name, no other PII.
 * No-op when PostHog isn't initialized (no key). Safe to call from auth
 * transitions without checking key presence first.
 */
export function setPostHogUser(userId: string | null): void {
  if (!hasKey) return;
  try {
    if (userId === null) {
      posthog.reset();
      return;
    }
    posthog.identify(userId);
  } catch {
    // Never let PostHog user context calls break auth transitions.
  }
}

// ---------- Phase 3: typed capture() wrapper ----------

/**
 * Taxonomy of product events and their allowed property shapes.
 * Names and keys are exact strings — changing them is a breaking schema change.
 */
export type Events = {
  // goal_created: only goal_id — per-goal reminder settings don't exist in the
  // model; has_reminder / recurrence_kind described global notification state,
  // which is misleading. Dropped to avoid shipping a broken metric.
  goal_created: { goal_id: string };
  goal_completed: { goal_id: string };
  goal_deleted: { goal_id: string };
  // view_switched: goal_id dropped — the toggle is app-wide, no per-goal
  // context is available at the call site. Emitting goal_id: '' pollutes dashboards.
  view_switched: { view: 'month' | 'week' };
  // items_acked = server-acknowledged event IDs from the batch; renamed from
  // items_pulled (misleading — this is a push-ack path, not a server→client pull).
  sync_completed: { ok: boolean; duration_ms: number; items_pushed: number; items_acked: number };
  sync_failed: { reason_code: 'network' | 'auth' | 'server' | 'unknown' };
  // source distinguishes how permission was obtained: 'prompt' (in-app request)
  // vs 'resume' (user granted in OS settings then returned to the app).
  notification_permission_changed: { state: string; source?: 'prompt' | 'resume' };
  debug_report_submitted: { size_bytes: number };
  shake_to_report_triggered: Record<string, never>;
};

/**
 * Per-event allowlist of property keys. Authoritative at the network boundary:
 * `capture()` filters the outgoing object to these keys before calling
 * `posthog.capture`. A spread of a full domain object cannot leak additional
 * fields even if the TypeScript type check is bypassed at a call site.
 */
const EVENT_ALLOWLIST: { [K in keyof Events]: ReadonlyArray<keyof Events[K]> } = {
  goal_created: ['goal_id'],
  goal_completed: ['goal_id'],
  goal_deleted: ['goal_id'],
  view_switched: ['view'],
  sync_completed: ['ok', 'duration_ms', 'items_pushed', 'items_acked'],
  sync_failed: ['reason_code'],
  notification_permission_changed: ['state', 'source'],
  debug_report_submitted: ['size_bytes'],
  shake_to_report_triggered: [],
};

/**
 * Type-safe, allowlist-enforced PostHog event capture.
 *
 * - TypeScript enforces the property shape at edit time.
 * - The runtime allowlist drops any key not in the whitelist before the event
 *   reaches the network — belt-and-braces against spread-of-domain-object leaks.
 * - No-op when PostHog is not initialized (no key). Defensive try/catch.
 */
export function capture<K extends keyof Events>(event: K, props: Events[K]): void {
  if (!hasKey) return;
  try {
    const allowed = EVENT_ALLOWLIST[event] as readonly string[];
    const filtered: Record<string, unknown> = {};
    for (const key of allowed) {
      if (Object.prototype.hasOwnProperty.call(props, key)) {
        filtered[key] = (props as Record<string, unknown>)[key];
      }
    }
    posthog.capture(event, filtered);
  } catch {
    // Never let analytics calls break app logic.
  }
}

// ---------- Test helpers ----------

/**
 * Reset internal state so `initPostHog()` can be called again in tests.
 * Not part of the public API.
 */
export function __resetPostHogForTest(): void {
  initialized = false;
  hasKey = false;
}
