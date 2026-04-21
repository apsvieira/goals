// Pure breadcrumb ring buffer. No Sentry, no Svelte, no app-store imports.
// Clock is injected via setClock(fn) for deterministic testing; default is Date.now.

import { openDB, type DBSchema, type IDBPDatabase } from 'idb';

export type BreadcrumbCategory =
  | 'log'
  | 'nav'
  | 'action'
  | 'sync'
  | 'auth'
  | 'net';

export type BreadcrumbLevel = 'info' | 'warn' | 'error';

export interface Breadcrumb {
  ts: number;
  category: BreadcrumbCategory;
  level: BreadcrumbLevel;
  message: string;
  data?: Record<string, unknown>;
}

// ---------- Ring-buffer policy ----------

const MAX_ENTRIES = 500;
const MAX_AGE_MS = 5 * 60 * 1000; // 5 minutes
const MAX_DATA_BYTES = 512;

// ---------- IndexedDB config ----------

const DB_NAME = 'goal-tracker-diagnostics';
const DB_VERSION = 1;
const STORE_NAME = 'diagnostics_buffer';
const ROW_KEY = 'current';

interface DiagnosticsDB extends DBSchema {
  diagnostics_buffer: {
    key: string;
    value: { id: string; crumbs: Breadcrumb[] };
  };
}

// ---------- Module state ----------

let buffer: Breadcrumb[] = [];
const listeners = new Set<(b: Breadcrumb) => void>();
let now: () => number = Date.now;

// Capture the original console.error at module load so Phase 4's console
// patching can't recursively re-enter this module when we surface listener
// errors.
const originalConsoleError: (...args: unknown[]) => void =
  typeof console !== 'undefined' && typeof console.error === 'function'
    ? console.error.bind(console)
    : () => {};

// ---------- Clock injection (for tests) ----------

export function setClock(fn: () => number): void {
  now = fn;
}

export function __resetForTest(): void {
  buffer = [];
  listeners.clear();
  now = Date.now;
}

// ---------- PII scrubbing ----------

const EMAIL_RE = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g;
const BEARER_RE = /Bearer\s+[A-Za-z0-9._~+/=-]+/gi;
// OAuth scrub is intentionally scoped to URL-separator-prefixed params: `?`,
// `&`, `#`, or `/`. Using a bare `\b` boundary fires after underscores/hyphens
// (e.g. `status_code=500`, `error_code=ENOENT`, `country_code=US`), which
// would destroy diagnostic fields that happen to share the `code=` suffix.
// OAuth codes appearing in free-form text *outside* URL syntax are not
// scrubbed by design — the plan scoped this to URL params.
const OAUTH_CODE_RE = /([?&#/])code=[^&#\s]+/g;
const OAUTH_STATE_RE = /([?&#/])state=[^&#\s]+/g;
// Any string containing `authorization:` / `cookie:` (case-insensitive) has
// its value replaced, regardless of field location. This guarantees the
// single-choke-point invariant: Authorization/Cookie values never leak into
// a breadcrumb, even if they arrive as free-form text in a `message` rather
// than a `headers` dict.
const AUTH_HEADER_STR_RE = /\bauthorization:\s*\S+/gi;
const COOKIE_HEADER_STR_RE = /\bcookie:\s*\S+/gi;

function scrubString(s: string): string {
  return s
    .replace(EMAIL_RE, '[email]')
    .replace(BEARER_RE, '[token]')
    .replace(AUTH_HEADER_STR_RE, 'authorization: [token]')
    .replace(COOKIE_HEADER_STR_RE, 'cookie: [cookie]')
    .replace(OAUTH_CODE_RE, '$1code=[oauth]')
    .replace(OAUTH_STATE_RE, '$1state=[oauth]');
}

function scrubValue(v: unknown): unknown {
  if (typeof v === 'string') return scrubString(v);
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return v.map(scrubValue);
  const out: Record<string, unknown> = {};
  for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
    out[k] = scrubValue(val);
  }
  return out;
}

function stripSensitiveHeaders(headers: unknown): unknown {
  if (headers === null || typeof headers !== 'object' || Array.isArray(headers)) {
    return headers;
  }
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(headers as Record<string, unknown>)) {
    const lower = k.toLowerCase();
    if (lower === 'authorization' || lower === 'cookie') continue;
    out[k] = v;
  }
  return out;
}

function scrubData(
  category: BreadcrumbCategory,
  data: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!data) return undefined;
  // Clone so we don't mutate the caller's object.
  const cloned: Record<string, unknown> = { ...data };

  // net-category: strip Authorization/Cookie from any headers-shaped field.
  if (category === 'net' && 'headers' in cloned) {
    cloned.headers = stripSensitiveHeaders(cloned.headers);
  }

  // action-category: drop goal_name and completion_note.
  if (category === 'action') {
    delete cloned.goal_name;
    delete cloned.completion_note;
  }

  // Scrub every string recursively for emails/bearer/oauth params.
  return scrubValue(cloned) as Record<string, unknown>;
}

// ---------- Core trim policy ----------

function trim(nowMs: number): void {
  const cutoff = nowMs - MAX_AGE_MS;
  // Drop crumbs older than cutoff.
  while (buffer.length > 0 && buffer[0].ts < cutoff) {
    buffer.shift();
  }
  // Cap at MAX_ENTRIES (drop oldest).
  while (buffer.length > MAX_ENTRIES) {
    buffer.shift();
  }
}

// ---------- Public API ----------

export function emit(b: Breadcrumb): void {
  // Defensive coercion: the diagnostic pipeline must be infallible, so we
  // never trust caller-supplied fields to be well-formed.
  const ts =
    typeof b.ts === 'number' && Number.isFinite(b.ts) ? b.ts : now();
  const rawMessage = typeof b.message === 'string' ? b.message : '';

  // Start with scrubbed message, scrubbed/cleaned data. Scrubbing walks
  // recursively; circular references or other hostile shapes can blow the
  // stack, so we guard the whole scrub+serialize pipeline.
  const scrubbedMessage = scrubString(rawMessage);
  let scrubbedData: Record<string, unknown> | undefined;
  let scrubFailed = false;
  try {
    scrubbedData = scrubData(b.category, b.data);
  } catch {
    scrubFailed = true;
  }

  // Serialization cap: if data > 512 bytes after stringify, replace with
  // marker. Wrap in try/catch to survive BigInt / other unserializable
  // payloads.
  let finalData: Record<string, unknown> | undefined = scrubbedData;
  if (scrubFailed) {
    finalData = { truncated: true, reason: 'unserializable' };
  } else if (scrubbedData !== undefined) {
    try {
      const serialized = JSON.stringify(scrubbedData);
      const size = new TextEncoder().encode(serialized).length;
      if (size > MAX_DATA_BYTES) {
        finalData = { truncated: true, size };
      }
    } catch {
      finalData = { truncated: true, reason: 'unserializable' };
    }
  }

  const crumb: Breadcrumb = {
    ts,
    category: b.category,
    level: b.level,
    message: scrubbedMessage,
    ...(finalData !== undefined ? { data: finalData } : {}),
  };

  buffer.push(crumb);
  trim(now());

  // Notify subscribers with the already-scrubbed crumb. Errors from one
  // listener must not block delivery to other listeners; surface them via
  // the captured original console.error so they aren't silently swallowed.
  for (const listener of listeners) {
    try {
      listener(crumb);
    } catch (err) {
      originalConsoleError('[breadcrumbs] listener threw', err);
    }
  }
}

export function snapshot(): Breadcrumb[] {
  trim(now());
  return buffer.slice();
}

export function subscribe(listener: (b: Breadcrumb) => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function clear(): void {
  buffer = [];
}

// ---------- Persistence ----------

function getIDB(): IDBFactory | undefined {
  if (typeof indexedDB === 'undefined') return undefined;
  return indexedDB;
}

async function openDiagnosticsDB(): Promise<IDBPDatabase<DiagnosticsDB> | null> {
  if (!getIDB()) return null;
  try {
    return await openDB<DiagnosticsDB>(DB_NAME, DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains(STORE_NAME)) {
          db.createObjectStore(STORE_NAME, { keyPath: 'id' });
        }
      },
    });
  } catch {
    return null;
  }
}

export async function persist(): Promise<void> {
  // Trim before serializing so the on-disk copy cannot violate the
  // "never more than MAX_ENTRIES, never older than MAX_AGE_MS" invariant.
  trim(now());
  const db = await openDiagnosticsDB();
  if (!db) return;
  try {
    await db.put(STORE_NAME, { id: ROW_KEY, crumbs: buffer.slice() });
  } catch {
    // No-op on write failure.
  } finally {
    db.close();
  }
}

export async function restore(): Promise<void> {
  const db = await openDiagnosticsDB();
  if (!db) return;
  try {
    const row = await db.get(STORE_NAME, ROW_KEY);
    if (row && Array.isArray(row.crumbs)) {
      buffer = row.crumbs.slice();
      trim(now());
    }
  } catch {
    // No-op on read failure.
  } finally {
    db.close();
  }
}
