import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { deleteDB } from 'idb';
import {
  emit,
  snapshot,
  subscribe,
  clear,
  persist,
  restore,
  setClock,
  __resetForTest,
  type Breadcrumb,
} from '../diagnostics/breadcrumbs';

const DIAG_DB_NAME = 'goal-tracker-diagnostics';

async function cleanupDiagDB(): Promise<void> {
  try {
    await deleteDB(DIAG_DB_NAME);
  } catch {
    // ignore
  }
}

describe('breadcrumbs — PII scrubbing', () => {
  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('replaces emails in message with [email]', () => {
    emit({
      ts: 1_000_000,
      category: 'log',
      level: 'info',
      message: 'user alice@example.com signed in',
    });
    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    expect(crumbs[0].message).toBe('user [email] signed in');
  });

  it('replaces bearer tokens in message and strips Authorization header', () => {
    emit({
      ts: 1_000_000,
      category: 'net',
      level: 'info',
      message: 'GET /api with Bearer abc123.def-456_xyz',
      data: {
        headers: {
          Authorization: 'Bearer secret-token-goes-here',
          Cookie: 'session=xyz',
          'Content-Type': 'application/json',
        },
      },
    });
    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    expect(crumbs[0].message).toBe('GET /api with [token]');
    const headers = (crumbs[0].data as { headers: Record<string, unknown> }).headers;
    expect(headers).not.toHaveProperty('Authorization');
    expect(headers).not.toHaveProperty('Cookie');
    expect(headers['Content-Type']).toBe('application/json');
  });

  it('replaces OAuth code= and state= in URL params with [oauth]', () => {
    emit({
      ts: 1_000_000,
      category: 'auth',
      level: 'info',
      message: 'callback https://app/cb?code=abc123xyz&state=987zxy&other=keep',
    });
    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    expect(crumbs[0].message).toContain('code=[oauth]');
    expect(crumbs[0].message).toContain('state=[oauth]');
    expect(crumbs[0].message).toContain('other=keep');
  });

  it('drops goal_name and completion_note on action crumbs, keeps goal_id', () => {
    emit({
      ts: 1_000_000,
      category: 'action',
      level: 'info',
      message: 'goal completed',
      data: {
        goal_id: 'g-42',
        goal_name: 'Morning Run',
        completion_note: 'felt great today',
        extra: 'keep',
      },
    });
    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    const data = crumbs[0].data as Record<string, unknown>;
    expect(data).toHaveProperty('goal_id', 'g-42');
    expect(data).not.toHaveProperty('goal_name');
    expect(data).not.toHaveProperty('completion_note');
    expect(data).toHaveProperty('extra', 'keep');
  });
});

describe('breadcrumbs — ring buffer policy', () => {
  beforeEach(() => {
    __resetForTest();
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('caps buffer at 500 entries and evicts oldest first', () => {
    let t = 1;
    setClock(() => t);

    for (let i = 0; i < 501; i++) {
      emit({
        ts: t,
        category: 'log',
        level: 'info',
        message: `msg-${i}`,
      });
      t += 1; // keep all crumbs well within the 5-minute window
    }

    const crumbs = snapshot();
    expect(crumbs).toHaveLength(500);
    // Oldest (msg-0) should be gone; oldest remaining should be msg-1.
    expect(crumbs[0].message).toBe('msg-1');
    expect(crumbs[crumbs.length - 1].message).toBe('msg-500');
  });

  it('evicts crumbs older than 5 minutes when a newer crumb arrives', () => {
    let t = 0;
    setClock(() => t);

    emit({ ts: t, category: 'log', level: 'info', message: 'old' });

    t = 6 * 60 * 1000; // 6 minutes later
    emit({ ts: t, category: 'log', level: 'info', message: 'new' });

    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    expect(crumbs[0].message).toBe('new');
  });
});

describe('breadcrumbs — persistence round-trip', () => {
  beforeEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('persist + clear + restore returns the same crumbs', async () => {
    let t = 1000;
    setClock(() => t);

    const originals: Breadcrumb[] = [
      { ts: t, category: 'log', level: 'info', message: 'first' },
      { ts: t + 1, category: 'nav', level: 'info', message: 'second' },
      { ts: t + 2, category: 'sync', level: 'warn', message: 'third' },
    ];

    for (const b of originals) emit(b);

    expect(snapshot()).toHaveLength(3);

    await persist();
    clear();
    expect(snapshot()).toHaveLength(0);

    await restore();
    const restored = snapshot();
    expect(restored).toHaveLength(3);
    expect(restored.map((c) => c.message)).toEqual(['first', 'second', 'third']);
  });

  it('restore evicts crumbs older than 5 minutes', async () => {
    let t = 0;
    setClock(() => t);

    emit({ ts: t, category: 'log', level: 'info', message: 'ancient' });
    await persist();
    clear();

    // Advance clock past the 5-minute window.
    t = 10 * 60 * 1000;

    await restore();
    expect(snapshot()).toHaveLength(0);
  });
});

describe('breadcrumbs — subscribe', () => {
  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('notifies listener with a scrubbed crumb and unsubscribe stops delivery', () => {
    const received: Breadcrumb[] = [];
    const unsub = subscribe((c) => received.push(c));

    emit({
      ts: 1_000_000,
      category: 'log',
      level: 'info',
      message: 'ping from foo@bar.com',
    });

    expect(received).toHaveLength(1);
    // Listener must receive scrubbed content.
    expect(received[0].message).toBe('ping from [email]');

    unsub();

    emit({
      ts: 1_000_001,
      category: 'log',
      level: 'info',
      message: 'after unsubscribe',
    });
    expect(received).toHaveLength(1);
  });
});

describe('breadcrumbs — data serialization cap', () => {
  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('replaces data with { truncated: true, size: N } when > 512 bytes', () => {
    // Build a payload whose JSON stringify exceeds 512 bytes.
    const bigString = 'x'.repeat(1000);
    emit({
      ts: 1_000_000,
      category: 'log',
      level: 'info',
      message: 'big',
      data: { blob: bigString },
    });

    const crumbs = snapshot();
    expect(crumbs).toHaveLength(1);
    const data = crumbs[0].data as Record<string, unknown>;
    expect(data).toHaveProperty('truncated', true);
    expect(typeof data.size).toBe('number');
    expect(data.size as number).toBeGreaterThan(512);
    expect(data).not.toHaveProperty('blob');
  });
});

describe('breadcrumbs — reviewer-requested hardening', () => {
  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
  });

  afterEach(async () => {
    __resetForTest();
    await cleanupDiagDB();
  });

  it('does not scrub status_code= or error_code= in free-form messages', () => {
    emit({
      ts: 1_000_000,
      category: 'net',
      level: 'info',
      message: 'request failed with status_code=500 and error_code=ENOENT',
    });
    expect(snapshot()[0].message).toBe(
      'request failed with status_code=500 and error_code=ENOENT',
    );
  });

  it('scrubs authorization: header when it appears in a message string', () => {
    emit({
      ts: 1_000_000,
      category: 'net',
      level: 'error',
      message: 'failed: authorization: abc123.def456',
    });
    expect(snapshot()[0].message).not.toContain('abc123');
  });

  it('scrubs cookie: header strings case-insensitively', () => {
    emit({
      ts: 1_000_000,
      category: 'net',
      level: 'error',
      message: 'Cookie: session=deadbeef',
    });
    expect(snapshot()[0].message).not.toContain('deadbeef');
  });

  it('a throwing listener does not break delivery to other listeners', () => {
    const received: Breadcrumb[] = [];
    subscribe(() => {
      throw new Error('boom');
    });
    subscribe((c) => received.push(c));
    emit({ ts: 1_000_000, category: 'log', level: 'info', message: 'ok' });
    expect(received).toHaveLength(1);
  });

  it('does not throw on malformed breadcrumb (missing message)', () => {
    expect(() =>
      emit({ ts: 1_000_000, category: 'log', level: 'info' } as unknown as Breadcrumb),
    ).not.toThrow();
  });

  it('does not throw on unserializable data (BigInt / circular)', () => {
    const circ: Record<string, unknown> = {};
    circ.self = circ;
    expect(() =>
      emit({
        ts: 1_000_000,
        category: 'log',
        level: 'info',
        message: 'x',
        data: circ,
      }),
    ).not.toThrow();
    const d = snapshot()[0].data as Record<string, unknown> | undefined;
    expect(d?.truncated).toBe(true);
  });

  it('snapshot re-applies 5-min window on stale buffer without new emits', () => {
    setClock(() => 1_000_000);
    emit({ ts: 1_000_000, category: 'log', level: 'info', message: 'old' });
    setClock(() => 1_000_000 + 10 * 60 * 1000);
    expect(snapshot()).toEqual([]);
  });
});

describe('breadcrumbs — IndexedDB unavailable', () => {
  let savedIDB: unknown;

  beforeEach(() => {
    __resetForTest();
    setClock(() => 1_000_000);
    // Remove indexedDB from globalThis to simulate SSR / restricted environment.
    savedIDB = (globalThis as Record<string, unknown>).indexedDB;
    delete (globalThis as Record<string, unknown>).indexedDB;
  });

  afterEach(async () => {
    (globalThis as Record<string, unknown>).indexedDB = savedIDB;
    __resetForTest();
    await cleanupDiagDB();
  });

  it('persist() and restore() resolve without throwing when indexedDB is undefined', async () => {
    emit({
      ts: 1_000_000,
      category: 'log',
      level: 'info',
      message: 'noop path',
    });
    await expect(persist()).resolves.toBeUndefined();
    await expect(restore()).resolves.toBeUndefined();
  });
});
