import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';

// Mock Capacitor so platform detection doesn't touch a real native bridge.
vi.mock('@capacitor/core', () => ({
  Capacitor: {
    isNativePlatform: () => false,
    getPlatform: () => 'web',
  },
}));

// Default: authenticated user with pending events lookup mocked separately.
const mockAuthStore = vi.hoisted(() => {
  let val: unknown = { type: 'authenticated', user: { id: 'u1', email: 'a@b.c', created_at: '' } };
  const subs = new Set<(v: unknown) => void>();
  return {
    subscribe: vi.fn((cb: (v: unknown) => void) => {
      cb(val);
      subs.add(cb);
      return () => subs.delete(cb);
    }),
    set: (v: unknown) => {
      val = v;
      subs.forEach((cb) => cb(v));
    },
    get: () => val,
  };
});

// isOnline store — writable-shaped mock that emits initial value on subscribe
// and supports set().
const mockIsOnline = vi.hoisted(() => {
  let val = true;
  const subs = new Set<(v: boolean) => void>();
  return {
    subscribe: vi.fn((cb: (v: boolean) => void) => {
      cb(val);
      subs.add(cb);
      return () => subs.delete(cb);
    }),
    set: (v: boolean) => {
      val = v;
      subs.forEach((cb) => cb(v));
    },
  };
});

const mockDebugReportModalOpen = vi.hoisted(() => ({
  subscribe: vi.fn(() => () => {}),
  set: vi.fn(),
}));

vi.mock('../stores', () => ({
  authStore: mockAuthStore,
  isOnline: mockIsOnline,
  debugReportModalOpen: mockDebugReportModalOpen,
  openDebugReport: vi.fn(),
  closeDebugReport: vi.fn(),
}));

vi.mock('../token-storage', () => ({
  getToken: vi.fn().mockResolvedValue(null),
}));

vi.mock('../storage', () => ({
  getUnsyncedEvents: vi.fn().mockResolvedValue([]),
}));

// Re-import after mocks are registered.
import {
  sendDebugReport,
  drainQueue,
  isClientRateLimited,
  __resetForTest,
} from '../diagnostics/debug-report';
import {
  emit,
  snapshot,
  clear,
  __resetForTest as __resetBreadcrumbsForTest,
} from '../diagnostics/breadcrumbs';

const QUEUE_DB_NAME = 'goal-tracker-debug-queue';
const QUEUE_STORE = 'debug_report_queue';

async function queueCount(): Promise<number> {
  // Open raw IDB via idb-compat wrapper; using native IDB keeps us agnostic
  // of the module's schema types.
  const req = indexedDB.open(QUEUE_DB_NAME);
  return new Promise((resolve) => {
    req.onsuccess = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(QUEUE_STORE)) {
        db.close();
        resolve(0);
        return;
      }
      const tx = db.transaction(QUEUE_STORE, 'readonly');
      const store = tx.objectStore(QUEUE_STORE);
      const countReq = store.count();
      countReq.onsuccess = () => {
        db.close();
        resolve(countReq.result);
      };
      countReq.onerror = () => {
        db.close();
        resolve(0);
      };
    };
    req.onerror = () => resolve(0);
  });
}

async function queueKeys(): Promise<number[]> {
  const req = indexedDB.open(QUEUE_DB_NAME);
  return new Promise((resolve) => {
    req.onsuccess = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(QUEUE_STORE)) {
        db.close();
        resolve([]);
        return;
      }
      const tx = db.transaction(QUEUE_STORE, 'readonly');
      const store = tx.objectStore(QUEUE_STORE);
      const r = store.getAllKeys();
      r.onsuccess = () => {
        db.close();
        resolve(r.result as number[]);
      };
      r.onerror = () => {
        db.close();
        resolve([]);
      };
    };
    req.onerror = () => resolve([]);
  });
}

function okResponse(body: unknown = { ok: true }): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function createdResponse(): Response {
  return new Response(JSON.stringify({ ok: true }), {
    status: 201,
    headers: { 'Content-Type': 'application/json' },
  });
}

function rateLimitedResponse(body: unknown = { error: 'slow down' }): Response {
  return new Response(JSON.stringify(body), {
    status: 429,
    headers: { 'Content-Type': 'application/json' },
  });
}

function errorResponse(status: number): Response {
  return new Response('error', { status });
}

describe('debug-report', () => {
  beforeEach(() => {
    __resetForTest();
    __resetBreadcrumbsForTest();
    localStorage.clear();
    vi.restoreAllMocks();
  });

  afterEach(async () => {
    __resetForTest();
    __resetBreadcrumbsForTest();
    localStorage.clear();
    try {
      await deleteDB(QUEUE_DB_NAME);
    } catch {
      // ignore
    }
  });

  it('builds a payload with all required fields and includes breadcrumbs', async () => {
    emit({
      ts: Date.now(),
      category: 'nav',
      level: 'info',
      message: 'home → privacy',
    });

    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const result = await sendDebugReport({ description: 'hi', trigger: 'shake' });
    expect(result.outcome).toBe('sent');
    expect(fetchMock).toHaveBeenCalledTimes(1);

    const call = fetchMock.mock.calls[0];
    const url = call[0] as string;
    const init = call[1] as RequestInit;
    expect(url).toContain('/debug-reports');
    expect(init.method).toBe('POST');
    expect(init.credentials).toBe('include');

    const body = JSON.parse(init.body as string);
    expect(typeof body.client_id).toBe('string');
    expect(body.client_id.length).toBeGreaterThan(0);
    expect(typeof body.app_version).toBe('string');
    expect(['android', 'ios', 'web']).toContain(body.platform);
    expect(body.device).toEqual(expect.objectContaining({
      model: expect.any(String),
      os: expect.any(String),
      webview: expect.any(String),
    }));
    expect(body.state).toEqual(expect.objectContaining({
      route: expect.any(String),
      online: expect.any(Boolean),
      pending_events: expect.any(Number),
      goal_count: expect.any(Number),
      auth_state: expect.any(String),
      notif_permission: expect.any(String),
    }));
    expect(body.description).toBe('hi');
    expect(Array.isArray(body.breadcrumbs)).toBe(true);
    expect(body.breadcrumbs.length).toBe(1);
    expect(body.breadcrumbs[0].category).toBe('nav');
    expect(body.trigger).toBe('shake');
    expect(typeof body.client_ts).toBe('number');
  });

  it('returns client_rate_limited if last send was within 60s', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    localStorage.setItem('last_debug_report_ts', String(Date.now()));
    expect(isClientRateLimited()).toBe(true);

    const result = await sendDebugReport({ description: '', trigger: 'shake' });
    expect(result.outcome).toBe('client_rate_limited');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('passes through once the 60s client rate limit expires', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    localStorage.setItem('last_debug_report_ts', String(Date.now() - 61_000));
    expect(isClientRateLimited()).toBe(false);

    const result = await sendDebugReport({ description: '', trigger: 'shake' });
    expect(result.outcome).toBe('sent');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('maps server 429 to rate_limited outcome with the server message', async () => {
    const fetchMock = vi.fn().mockResolvedValue(rateLimitedResponse({ error: 'slow down' }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const result = await sendDebugReport({ description: 'x', trigger: 'shake' });
    expect(result.outcome).toBe('rate_limited');
    expect(result.message).toBe('slow down');
  });

  it('queues the payload to IDB on network error', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('network down'));
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const result = await sendDebugReport({ description: 'offline', trigger: 'shake' });
    expect(result.outcome).toBe('queued');
    expect(await queueCount()).toBe(1);
  });

  it('caps the queue at 10 and drops the oldest entry when full', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('network down'));
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    for (let i = 0; i < 11; i++) {
      // Reset the client rate limit so subsequent sends go through.
      localStorage.removeItem('last_debug_report_ts');
      await sendDebugReport({ description: `msg-${i}`, trigger: 'shake' });
    }

    const count = await queueCount();
    expect(count).toBe(10);
    const keys = await queueKeys();
    // Keys are auto-incrementing: after 11 pushes with oldest dropped, the
    // remaining key range should be exactly [2..11], proving FIFO eviction
    // (id=1 was dropped, id=11 is newest). LIFO would have kept id=1.
    expect(keys.length).toBe(10);
    expect(Math.min(...keys)).toBe(2);
    expect(Math.max(...keys)).toBe(11);
  });

  it('drainQueue posts each entry in FIFO order and empties the queue on success', async () => {
    // Seed the queue with 2 entries via network errors.
    const failingFetch = vi.fn().mockRejectedValue(new TypeError('network down'));
    globalThis.fetch = failingFetch as unknown as typeof fetch;
    await sendDebugReport({ description: 'first', trigger: 'shake' });
    localStorage.removeItem('last_debug_report_ts');
    await sendDebugReport({ description: 'second', trigger: 'shake' });
    expect(await queueCount()).toBe(2);

    // Now swap fetch for a happy one and drain.
    const okFetch = vi.fn().mockResolvedValue(createdResponse());
    globalThis.fetch = okFetch as unknown as typeof fetch;

    await drainQueue();

    expect(okFetch).toHaveBeenCalledTimes(2);
    expect(await queueCount()).toBe(0);
  });

  it('drainQueue stops on the first server failure and leaves the rest queued', async () => {
    const failingFetch = vi.fn().mockRejectedValue(new TypeError('network down'));
    globalThis.fetch = failingFetch as unknown as typeof fetch;
    for (let i = 0; i < 3; i++) {
      localStorage.removeItem('last_debug_report_ts');
      await sendDebugReport({ description: `m${i}`, trigger: 'shake' });
    }
    expect(await queueCount()).toBe(3);

    // Sequence: 201, 500, (should not be called)
    const seq = [createdResponse(), errorResponse(500)];
    const drainFetch = vi.fn().mockImplementation(() => {
      const next = seq.shift();
      if (!next) throw new Error('fetch called more times than expected');
      return Promise.resolve(next);
    });
    globalThis.fetch = drainFetch as unknown as typeof fetch;

    await drainQueue();

    expect(drainFetch).toHaveBeenCalledTimes(2);
    expect(await queueCount()).toBe(2);
  });

  it('clears the breadcrumbs buffer on a successful send', async () => {
    const t = Date.now();
    emit({ ts: t, category: 'action', level: 'info', message: 'goal created' });
    emit({ ts: t + 1, category: 'nav', level: 'info', message: 'home → privacy' });
    expect(snapshot().length).toBe(2);

    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const result = await sendDebugReport({ description: 'x', trigger: 'shake' });
    expect(result.outcome).toBe('sent');
    expect(snapshot().length).toBe(0);
  });

  it('reuses the same client_id across calls (persisted in localStorage)', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await sendDebugReport({ description: 'a', trigger: 'shake' });
    // First call recorded a send timestamp; clear the client rate limit so
    // the second call doesn't short-circuit.
    localStorage.removeItem('last_debug_report_ts');
    await sendDebugReport({ description: 'b', trigger: 'shake' });

    const body1 = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    const body2 = JSON.parse(fetchMock.mock.calls[1][1].body as string);
    expect(body1.client_id).toBe(body2.client_id);
    expect(localStorage.getItem('debug_client_id')).toBe(body1.client_id);
  });

  it('regenerates client_id if localStorage contains a non-UUID value', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    // Pre-seed a poisoned value from a prior build's bad fallback.
    localStorage.setItem('debug_client_id', 'not-a-uuid');

    const result = await sendDebugReport({ description: 'x', trigger: 'shake' });
    expect(result.outcome).toBe('sent');

    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.client_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
    );
    // And the bad value is overwritten so future sends aren't poisoned.
    expect(localStorage.getItem('debug_client_id')).toBe(body.client_id);
    expect(localStorage.getItem('debug_client_id')).not.toBe('not-a-uuid');
  });

  it('accepts trigger: "auto" and posts body.trigger === "auto"', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const result = await sendDebugReport({ description: 'boom', trigger: 'auto' });
    expect(result.outcome).toBe('sent');
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.trigger).toBe('auto');
  });

  it('trims the description before sending', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await sendDebugReport({ description: '  hi  ', trigger: 'shake' });
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.description).toBe('hi');
  });

  it('drainQueue guards against concurrent reentry', async () => {
    // Seed the queue with 2 entries via a failing fetch.
    const failingFetch = vi.fn().mockRejectedValue(new TypeError('network down'));
    globalThis.fetch = failingFetch as unknown as typeof fetch;
    await sendDebugReport({ description: 'first', trigger: 'shake' });
    localStorage.removeItem('last_debug_report_ts');
    await sendDebugReport({ description: 'second', trigger: 'shake' });
    expect(await queueCount()).toBe(2);

    // Swap in a happy fetch. Call drainQueue() twice concurrently — the
    // in-flight guard should cause the second call to no-op while the first
    // is still running, so fetch is called exactly twice total (once per
    // queue entry), not four times.
    const okFetch = vi.fn().mockResolvedValue(createdResponse());
    globalThis.fetch = okFetch as unknown as typeof fetch;

    const first = drainQueue();
    const second = drainQueue();
    await Promise.all([first, second]);

    expect(okFetch).toHaveBeenCalledTimes(2);
    expect(await queueCount()).toBe(0);
  });
});
