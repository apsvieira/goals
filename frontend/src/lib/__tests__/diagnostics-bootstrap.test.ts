import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';

// Capacitor is mocked so bootstrap's native-pause hook doesn't try to reach
// the real plugin layer in jsdom. A non-native platform means the App
// plugin is never called.
vi.mock('@capacitor/core', () => ({
  Capacitor: { isNativePlatform: () => false },
}));

vi.mock('@capacitor/app', () => ({
  App: { addListener: vi.fn().mockResolvedValue({ remove: vi.fn() }) },
}));

import {
  initDiagnostics,
  __resetBootstrapForTest,
  __isBootstrapInitialized,
} from '../diagnostics/bootstrap';
import { wrapFetch, __resetWrapFetchForTest } from '../diagnostics/net';
import { breadcrumbAction } from '../diagnostics/instrument';
import {
  snapshot,
  persist,
  __resetForTest,
  setClock,
} from '../diagnostics/breadcrumbs';

const DIAG_DB_NAME = 'goal-tracker-diagnostics';

async function cleanupDiagDB(): Promise<void> {
  try {
    await deleteDB(DIAG_DB_NAME);
  } catch {
    // ignore
  }
}

describe('initDiagnostics — idempotency', () => {
  beforeEach(() => {
    __resetForTest();
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    __resetForTest();
    await cleanupDiagDB();
  });

  it('patches console exactly once across multiple init calls', async () => {
    const originalLog = console.log;

    await initDiagnostics();
    expect(__isBootstrapInitialized()).toBe(true);
    const patchedLog = console.log;
    expect(patchedLog).not.toBe(originalLog);

    await initDiagnostics();
    // Second init is a no-op: console.log must still be the first patch,
    // not a double-wrapped variant.
    expect(console.log).toBe(patchedLog);
  });
});

describe('initDiagnostics — console patching', () => {
  beforeEach(() => {
    __resetForTest();
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    __resetForTest();
    await cleanupDiagDB();
  });

  it('console.warn("hi") emits a warn-level log breadcrumb and still calls original', async () => {
    // Spy on the original console.warn BEFORE bootstrap patches it so the
    // spy is what gets captured as "original" inside bootstrap.
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    await initDiagnostics();

    console.warn('hi');

    // Original still fires — the patched version wraps it.
    expect(spy).toHaveBeenCalledWith('hi');

    const crumbs = snapshot();
    const warnCrumbs = crumbs.filter(
      (c) => c.category === 'log' && c.level === 'warn',
    );
    expect(warnCrumbs).toHaveLength(1);
    expect(warnCrumbs[0].message).toBe('hi');
  });

  it('maps console.log / console.info to info level', async () => {
    vi.spyOn(console, 'log').mockImplementation(() => {});
    vi.spyOn(console, 'info').mockImplementation(() => {});
    await initDiagnostics();

    console.log('via-log');
    console.info('via-info');

    const crumbs = snapshot().filter((c) => c.category === 'log');
    const levels = crumbs.map((c) => c.level);
    expect(levels.every((l) => l === 'info')).toBe(true);
    const messages = crumbs.map((c) => c.message);
    expect(messages).toContain('via-log');
    expect(messages).toContain('via-info');
  });

  it('attaches extra args as data.args', async () => {
    vi.spyOn(console, 'log').mockImplementation(() => {});
    await initDiagnostics();

    console.log('primary', { id: 42 }, 'tail');

    const crumb = snapshot().find(
      (c) => c.category === 'log' && c.message === 'primary',
    );
    expect(crumb).toBeDefined();
    const data = crumb!.data as { args?: unknown[] } | undefined;
    expect(Array.isArray(data?.args)).toBe(true);
    expect(data!.args).toHaveLength(2);
  });
});

describe('wrapFetch — success', () => {
  beforeEach(() => {
    __resetForTest();
    __resetWrapFetchForTest();
    setClock(() => 1_000_000);
  });

  afterEach(() => {
    __resetWrapFetchForTest();
    __resetForTest();
  });

  it('emits a net breadcrumb with info level on 2xx', async () => {
    const fakeResponse = new Response('ok', { status: 200 });
    const fakeFetch = vi.fn().mockResolvedValue(fakeResponse);
    globalThis.fetch = fakeFetch as unknown as typeof fetch;

    wrapFetch();
    const res = await fetch('https://api.example.com/v1/thing?token=secret');
    expect(res).toBe(fakeResponse);

    const netCrumbs = snapshot().filter((c) => c.category === 'net');
    expect(netCrumbs).toHaveLength(1);
    expect(netCrumbs[0].level).toBe('info');
    expect(netCrumbs[0].message).toMatch(/^GET \/v1\/thing 200 \d+ms$/);
    const data = netCrumbs[0].data as Record<string, unknown>;
    expect(data.method).toBe('GET');
    expect(data.path).toBe('/v1/thing');
    expect(data.status).toBe(200);
  });

  it('emits a net breadcrumb with error level on 4xx / 5xx', async () => {
    const fakeResponse = new Response('nope', { status: 500 });
    globalThis.fetch = vi.fn().mockResolvedValue(fakeResponse) as unknown as typeof fetch;

    wrapFetch();
    await fetch('/api/broken', { method: 'POST' });

    const netCrumbs = snapshot().filter((c) => c.category === 'net');
    expect(netCrumbs).toHaveLength(1);
    expect(netCrumbs[0].level).toBe('error');
    expect(netCrumbs[0].message).toMatch(/^POST \/api\/broken 500 \d+ms$/);
  });

  it('wrapFetch — Request input > extracts method and path correctly', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(null, { status: 200 }),
    ) as unknown as typeof fetch;

    wrapFetch();
    await fetch(new Request('http://localhost/api/goals', { method: 'PUT' }));

    const crumb = snapshot().find((c) => c.category === 'net')!;
    expect(crumb.message).toContain('PUT /api/goals 200');
  });
});

describe('wrapFetch — network error', () => {
  beforeEach(() => {
    __resetForTest();
    __resetWrapFetchForTest();
    setClock(() => 1_000_000);
  });

  afterEach(() => {
    __resetWrapFetchForTest();
    __resetForTest();
  });

  it('emits an error breadcrumb with status -1 and rethrows', async () => {
    const boom = new TypeError('network down');
    globalThis.fetch = vi.fn().mockRejectedValue(boom) as unknown as typeof fetch;

    wrapFetch();
    await expect(fetch('/api/x')).rejects.toBe(boom);

    const netCrumbs = snapshot().filter((c) => c.category === 'net');
    expect(netCrumbs).toHaveLength(1);
    expect(netCrumbs[0].level).toBe('error');
    expect(netCrumbs[0].message).toMatch(/^GET \/api\/x -1 \d+ms$/);
    const data = netCrumbs[0].data as Record<string, unknown>;
    expect(data.status).toBe(-1);
  });
});

describe('breadcrumbAction — scrubbing integration', () => {
  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('goal_id survives but goal_name is dropped by the Phase 3 scrubber', () => {
    breadcrumbAction('goal completed', {
      goal_id: 'abc',
      goal_name: 'secret',
    });

    const crumbs = snapshot().filter((c) => c.category === 'action');
    expect(crumbs).toHaveLength(1);
    const data = crumbs[0].data as Record<string, unknown>;
    expect(data.goal_id).toBe('abc');
    expect(data).not.toHaveProperty('goal_name');
  });
});

describe('initDiagnostics — visibilitychange triggers persist', () => {
  beforeEach(() => {
    __resetForTest();
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    __resetForTest();
    await cleanupDiagDB();
  });

  it('dispatching visibilitychange triggers a persist to IndexedDB', async () => {
    await initDiagnostics();

    // Seed a distinctive crumb that we can round-trip through persist.
    breadcrumbAction('before-hide', { goal_id: 'g-x' });
    expect(snapshot().some((c) => c.message === 'before-hide')).toBe(true);

    // Fire visibilitychange on document with visibilityState === 'hidden' —
    // bootstrap registered a listener that calls persist() fire-and-forget.
    // Wait for the microtask chain to finish.
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'hidden',
    });
    document.dispatchEvent(new Event('visibilitychange'));
    await new Promise((resolve) => setTimeout(resolve, 50));

    // Clear in-memory buffer then restore from IndexedDB — if the event
    // listener triggered persist(), the crumb lives on disk now.
    __resetForTest();
    setClock(() => 1_000_000);
    expect(snapshot()).toHaveLength(0);

    const { restore } = await import('../diagnostics/breadcrumbs');
    await restore();
    expect(snapshot().some((c) => c.message === 'before-hide')).toBe(true);
  });
});

describe('initDiagnostics — restore runs before emit', () => {
  beforeEach(async () => {
    __resetForTest();
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    await cleanupDiagDB();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetBootstrapForTest();
    __resetWrapFetchForTest();
    __resetForTest();
    await cleanupDiagDB();
  });

  it('persisted crumbs are restored into the in-memory buffer', async () => {
    // Seed the persistent buffer.
    breadcrumbAction('prior session', { goal_id: 'g-old' });
    await persist();
    __resetForTest();
    setClock(() => 1_000_000);
    expect(snapshot()).toHaveLength(0);

    await initDiagnostics();

    const crumbs = snapshot().filter((c) => c.category === 'action');
    expect(crumbs.map((c) => c.message)).toContain('prior session');
  });
});
