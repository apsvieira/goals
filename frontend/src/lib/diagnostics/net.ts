// Phase 4 fetch wrapper. Replaces window.fetch with a version that emits a
// `net` breadcrumb per request. Headers are NOT included — the breadcrumb
// module's `stripSensitiveHeaders` only handles plain objects, and the four
// fields we care about (method, path, status, duration) are already enough
// to reconstruct an app session without leaking auth material.

import { emit } from './breadcrumbs';

let wrapped = false;
let originalFetch: typeof fetch | undefined;

function extractUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  if (input instanceof URL) return input.toString();
  // Request object
  try {
    return (input as Request).url;
  } catch {
    return String(input);
  }
}

function extractMethod(input: RequestInfo | URL, init?: RequestInit): string {
  if (init?.method) return init.method.toUpperCase();
  if (input instanceof Request) return input.method.toUpperCase();
  return 'GET';
}

interface PathParts {
  path: string;
  origin?: string;
}

function parsePath(rawUrl: string): PathParts {
  try {
    // URL() needs a base for relative URLs; use current origin if available.
    const base =
      typeof window !== 'undefined' && window.location
        ? window.location.origin
        : 'http://localhost';
    const u = new URL(rawUrl, base);
    const currentOrigin =
      typeof window !== 'undefined' && window.location
        ? window.location.origin
        : undefined;
    const parts: PathParts = { path: u.pathname };
    if (u.origin && u.origin !== currentOrigin && u.origin !== 'null') {
      parts.origin = u.origin;
    }
    return parts;
  } catch {
    // Fall back to stripping the query string by hand.
    const q = rawUrl.indexOf('?');
    return { path: q >= 0 ? rawUrl.slice(0, q) : rawUrl };
  }
}

/**
 * Install a fetch wrapper on window.fetch / globalThis.fetch that emits a
 * `net` breadcrumb for every request. Idempotent: safe to call from both
 * bootstrap.ts and tests without stacking wrappers.
 */
export function wrapFetch(): void {
  if (wrapped) return;
  const target: { fetch?: typeof fetch } =
    typeof globalThis !== 'undefined'
      ? (globalThis as { fetch?: typeof fetch })
      : ({} as { fetch?: typeof fetch });

  if (typeof target.fetch !== 'function') return;

  originalFetch = target.fetch;
  const orig = originalFetch;
  wrapped = true;

  target.fetch = async function patchedFetch(
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> {
    const start =
      typeof performance !== 'undefined' && typeof performance.now === 'function'
        ? performance.now()
        : Date.now();
    const method = extractMethod(input, init);
    const rawUrl = extractUrl(input);
    const { path, origin } = parsePath(rawUrl);

    try {
      const response = await orig(input as RequestInfo, init);
      const duration = Math.round(
        (typeof performance !== 'undefined' && typeof performance.now === 'function'
          ? performance.now()
          : Date.now()) - start,
      );
      try {
        emit({
          ts: Date.now(),
          category: 'net',
          level: response.status >= 400 ? 'error' : 'info',
          message: `${method} ${path} ${response.status} ${duration}ms`,
          data: {
            method,
            path,
            status: response.status,
            duration_ms: duration,
            ...(origin ? { origin } : {}),
          },
        });
      } catch {
        // breadcrumb emission must never break the request
      }
      return response;
    } catch (err) {
      const duration = Math.round(
        (typeof performance !== 'undefined' && typeof performance.now === 'function'
          ? performance.now()
          : Date.now()) - start,
      );
      try {
        emit({
          ts: Date.now(),
          category: 'net',
          level: 'error',
          message: `${method} ${path} -1 ${duration}ms`,
          data: {
            method,
            path,
            status: -1,
            duration_ms: duration,
            ...(origin ? { origin } : {}),
          },
        });
      } catch {
        // noop
      }
      throw err;
    }
  } as typeof fetch;
}

/**
 * Test helper: restore the original fetch and allow wrapFetch to re-run.
 */
export function __resetWrapFetchForTest(): void {
  if (wrapped && originalFetch && typeof globalThis !== 'undefined') {
    (globalThis as { fetch?: typeof fetch }).fetch = originalFetch;
  }
  wrapped = false;
  originalFetch = undefined;
}
