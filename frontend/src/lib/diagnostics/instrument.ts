// Phase 4 convenience helpers for app code. Each wraps `emit()` with the
// right category + level so call sites don't have to remember the taxonomy.
// PII scrubbing happens inside emit(); callers pass raw data.

import { emit } from './breadcrumbs';

/**
 * Route change: from → to. Emitted once per navigation.
 */
export function breadcrumbNav(from: string, to: string): void {
  emit({
    ts: Date.now(),
    category: 'nav',
    level: 'info',
    message: `${from} → ${to}`,
    data: { from, to },
  });
}

/**
 * User action (goal created/completed/deleted/edited, etc.). `data.goal_id`
 * should always be preferred over a name; the scrubber drops `goal_name`
 * and `completion_note` unconditionally, but keeping the call-site honest
 * is still worth it.
 */
export function breadcrumbAction(
  message: string,
  data?: { goal_id?: string; [k: string]: unknown },
): void {
  emit({
    ts: Date.now(),
    category: 'action',
    level: 'info',
    message,
    ...(data ? { data } : {}),
  });
}

/**
 * Sync lifecycle: start, end, error. `error` bumps the level to `error`.
 */
export function breadcrumbSync(
  phase: 'start' | 'end' | 'error',
  data?: Record<string, unknown>,
): void {
  emit({
    ts: Date.now(),
    category: 'sync',
    level: phase === 'error' ? 'error' : 'info',
    message: `sync ${phase}`,
    ...(data ? { data } : {}),
  });
}

/**
 * Auth lifecycle: login, logout, session_expired, token_refresh,
 * session_restored. `session_expired` bumps the level to `warn`.
 */
export function breadcrumbAuth(
  event: 'login' | 'logout' | 'session_expired' | 'token_refresh' | 'session_restored',
  data?: Record<string, unknown>,
): void {
  emit({
    ts: Date.now(),
    category: 'auth',
    level: event === 'session_expired' ? 'warn' : 'info',
    message: `auth ${event}`,
    ...(data ? { data } : {}),
  });
}
