import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';
import {
  initStorage,
  resetDB,
  saveSyncEvent,
  getUnsyncedEvents,
  getSyncEvents,
  saveLocalGoal,
} from '../storage';
import { syncStatus } from '../sync';
import type { SyncEvent } from '../events';

// Mock @capacitor/core to avoid native platform detection
vi.mock('@capacitor/core', () => ({
  Capacitor: { isNativePlatform: () => false },
}));

// Mock token-storage to return null (no mobile auth token)
vi.mock('../token-storage', () => ({
  getToken: vi.fn().mockResolvedValue(null),
}));

// Mock stores — default to authenticated
const mockAuthStore = {
  subscribe: vi.fn((cb: (v: unknown) => void) => {
    cb({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com', created_at: '' } });
    return () => {};
  }),
};

vi.mock('../stores', () => ({
  authStore: mockAuthStore,
}));

function makeEvent(overrides: Partial<SyncEvent> = {}): SyncEvent {
  return {
    id: crypto.randomUUID(),
    type: 'goal_upsert',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: { id: 'g-1', name: 'Run', color: '#FF0000', position: 1 },
    ...overrides,
  };
}

describe('sendEvent', () => {
  beforeEach(() => {
    resetDB();
    vi.restoreAllMocks();
    // Re-apply default authenticated state
    mockAuthStore.subscribe = vi.fn((cb: (v: unknown) => void) => {
      cb({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com', created_at: '' } });
      return () => {};
    });
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('should mark event as synced on successful POST', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-success' });
    await saveSyncEvent(event);

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ processed: ['evt-success'] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { sendEvent } = await import('../event-sync');
    await sendEvent(event);

    // Event should now be marked as synced
    const unsynced = await getUnsyncedEvents();
    expect(unsynced).toHaveLength(0);

    const all = await getSyncEvents();
    expect(all).toHaveLength(1);
    expect(all[0].synced).toBe(true);
  }, 10000);

  it('should leave event unsynced when fetch throws', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-fail' });
    await saveSyncEvent(event);

    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const { sendEvent } = await import('../event-sync');
    await sendEvent(event);

    // Event should still be unsynced
    const unsynced = await getUnsyncedEvents();
    expect(unsynced).toHaveLength(1);
    expect(unsynced[0].id).toBe('evt-fail');
  }, 10000);

  it('should skip fetch when offline', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-offline' });
    await saveSyncEvent(event);

    const originalOnLine = navigator.onLine;
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });

    globalThis.fetch = vi.fn();

    const { sendEvent } = await import('../event-sync');
    await sendEvent(event);

    expect(globalThis.fetch).not.toHaveBeenCalled();

    // Restore
    Object.defineProperty(navigator, 'onLine', { value: originalOnLine, configurable: true });
  }, 10000);

  it('should not send synced field to server', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-payload-check' });
    await saveSyncEvent(event);

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ processed: ['evt-payload-check'] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { sendEvent } = await import('../event-sync');
    await sendEvent(event);

    const fetchCall = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const body = JSON.parse(fetchCall[1].body);
    const sentEvent = body.events[0];

    // The sent payload should NOT contain 'synced'
    expect(sentEvent).not.toHaveProperty('synced');
    expect(sentEvent).toHaveProperty('id');
    expect(sentEvent).toHaveProperty('type');
    expect(sentEvent).toHaveProperty('timestamp');
    expect(sentEvent).toHaveProperty('payload');
  }, 10000);
});

describe('flushPendingEvents', () => {
  beforeEach(() => {
    resetDB();
    vi.restoreAllMocks();
    mockAuthStore.subscribe = vi.fn((cb: (v: unknown) => void) => {
      cb({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com', created_at: '' } });
      return () => {};
    });
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('should batch-send all unsynced events and mark them synced', async () => {
    await initStorage();

    const e1 = makeEvent({ id: 'evt-1' });
    const e2 = makeEvent({ id: 'evt-2', type: 'completion_set', payload: { goal_id: 'g-1', date: '2026-04-05' } });
    const e3 = makeEvent({ id: 'evt-3', type: 'goal_delete', payload: { id: 'g-2' } });

    await saveSyncEvent(e1);
    await saveSyncEvent(e2);
    await saveSyncEvent(e3);

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ processed: ['evt-1', 'evt-2', 'evt-3'] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    // All events should be synced
    const unsynced = await getUnsyncedEvents();
    expect(unsynced).toHaveLength(0);

    // fetch called exactly once with all 3 events
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    const body = JSON.parse((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1].body);
    expect(body.events).toHaveLength(3);
  }, 10000);

  it('should not call fetch when there are no unsynced events', async () => {
    await initStorage();

    // Save only synced events
    await saveSyncEvent(makeEvent({ id: 'evt-synced', synced: true }));

    globalThis.fetch = vi.fn();

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    expect(globalThis.fetch).not.toHaveBeenCalled();
  }, 10000);

  it('should leave events unsynced when server returns error', async () => {
    await initStorage();

    await saveSyncEvent(makeEvent({ id: 'evt-1' }));
    await saveSyncEvent(makeEvent({ id: 'evt-2' }));

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response('Internal Server Error', { status: 500 })
    );

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    const unsynced = await getUnsyncedEvents();
    expect(unsynced).toHaveLength(2);
  }, 10000);
});

describe('api.ts creates SyncEvent on mutation', () => {
  beforeEach(async () => {
    resetDB();
    vi.restoreAllMocks();
    mockAuthStore.subscribe = vi.fn((cb: (v: unknown) => void) => {
      cb({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com', created_at: '' } });
      return () => {};
    });
    // Stub fetch so fire-and-forget sendEvent doesn't fail unhandled
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ processed: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );
    await initStorage();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('createGoal should save a goal_upsert event', async () => {
    const { createGoal } = await import('../api');
    const goal = await createGoal('Exercise', '#5B8C5A');

    const events = await getSyncEvents();
    expect(events.length).toBeGreaterThanOrEqual(1);

    const goalEvent = events.find(e => e.type === 'goal_upsert' && e.payload.id === goal.id);
    expect(goalEvent).toBeDefined();
    expect(goalEvent!.synced).toBe(false);
    expect(goalEvent!.payload).toMatchObject({
      id: goal.id,
      name: 'Exercise',
      color: '#5B8C5A',
    });
  }, 10000);

  it('updateGoal should save a goal_upsert event with full state', async () => {
    // Pre-create a goal locally
    await saveLocalGoal({
      id: 'g-upd',
      name: 'Read',
      color: '#708090',
      position: 1,
      created_at: '2026-01-01T00:00:00Z',
    });

    const { updateGoal } = await import('../api');
    const updated = await updateGoal('g-upd', { name: 'Read More' });

    const events = await getSyncEvents();
    const upsertEvent = events.find(
      e => e.type === 'goal_upsert' && e.payload.id === 'g-upd'
    );
    expect(upsertEvent).toBeDefined();
    // Should contain full state, not just the delta
    expect(upsertEvent!.payload).toMatchObject({
      id: 'g-upd',
      name: 'Read More',
      color: '#708090',
      position: 1,
    });
  }, 10000);

  it('archiveGoal should save a goal_delete event', async () => {
    await saveLocalGoal({
      id: 'g-del',
      name: 'Meditate',
      color: '#5B8C5A',
      position: 1,
      created_at: '2026-01-01T00:00:00Z',
    });

    const { archiveGoal } = await import('../api');
    await archiveGoal('g-del');

    const events = await getSyncEvents();
    const deleteEvent = events.find(e => e.type === 'goal_delete');
    expect(deleteEvent).toBeDefined();
    expect(deleteEvent!.payload).toEqual({ id: 'g-del' });
  }, 10000);

  it('createCompletion should save a completion_set event', async () => {
    const { createCompletion } = await import('../api');
    await createCompletion('g-1', '2026-04-05');

    const events = await getSyncEvents();
    const setEvent = events.find(e => e.type === 'completion_set');
    expect(setEvent).toBeDefined();
    expect(setEvent!.payload).toEqual({ goal_id: 'g-1', date: '2026-04-05' });
  }, 10000);

  it('deleteCompletion should save a completion_unset event', async () => {
    const { deleteCompletion } = await import('../api');
    await deleteCompletion('g-1:2026-04-05', 'g-1', '2026-04-05');

    const events = await getSyncEvents();
    const unsetEvent = events.find(e => e.type === 'completion_unset');
    expect(unsetEvent).toBeDefined();
    expect(unsetEvent!.payload).toEqual({ goal_id: 'g-1', date: '2026-04-05' });
  }, 10000);

  it('reorderGoals should save N goal_upsert events', async () => {
    await saveLocalGoal({ id: 'g-a', name: 'A', color: '#111', position: 1, created_at: '2026-01-01T00:00:00Z' });
    await saveLocalGoal({ id: 'g-b', name: 'B', color: '#222', position: 2, created_at: '2026-01-01T00:00:00Z' });
    await saveLocalGoal({ id: 'g-c', name: 'C', color: '#333', position: 3, created_at: '2026-01-01T00:00:00Z' });

    const { reorderGoals } = await import('../api');
    await reorderGoals(['g-c', 'g-a', 'g-b']);

    const events = await getSyncEvents();
    const upsertEvents = events.filter(e => e.type === 'goal_upsert');

    // Should have 3 separate events
    expect(upsertEvents).toHaveLength(3);

    // Verify positions match new order
    const byGoalId = Object.fromEntries(upsertEvents.map(e => [e.payload.id, e.payload]));
    expect(byGoalId['g-c'].position).toBe(1);
    expect(byGoalId['g-a'].position).toBe(2);
    expect(byGoalId['g-b'].position).toBe(3);
  }, 10000);
});

describe('flushPendingEvents syncStatus', () => {
  beforeEach(() => {
    resetDB();
    vi.restoreAllMocks();
    syncStatus.set({ state: 'idle' });
    mockAuthStore.subscribe = vi.fn((cb: (v: unknown) => void) => {
      cb({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com', created_at: '' } });
      return () => {};
    });
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('should set syncStatus to syncing then idle on successful flush', async () => {
    await initStorage();

    await saveSyncEvent(makeEvent({ id: 'evt-status-1' }));

    const statusSnapshots: Array<{ state: string }> = [];
    const unsubscribe = syncStatus.subscribe(s => statusSnapshots.push({ ...s }));

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ processed: ['evt-status-1'] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    unsubscribe();

    // Should have transitioned: idle -> syncing -> idle
    const states = statusSnapshots.map(s => s.state);
    expect(states).toContain('syncing');
    expect(states[states.length - 1]).toBe('idle');
  }, 10000);

  it('should set syncStatus to error on network failure', async () => {
    await initStorage();

    await saveSyncEvent(makeEvent({ id: 'evt-status-err' }));

    const statusSnapshots: Array<Record<string, unknown>> = [];
    const unsubscribe = syncStatus.subscribe(s => statusSnapshots.push({ ...s }));

    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    unsubscribe();

    const lastStatus = statusSnapshots[statusSnapshots.length - 1];
    expect(lastStatus.state).toBe('error');
    expect(lastStatus.message).toBe('Sync failed');
    expect(lastStatus.canRetry).toBe(true);
  }, 10000);

  it('should set syncStatus to error on HTTP error response', async () => {
    await initStorage();

    await saveSyncEvent(makeEvent({ id: 'evt-status-500' }));

    const statusSnapshots: Array<Record<string, unknown>> = [];
    const unsubscribe = syncStatus.subscribe(s => statusSnapshots.push({ ...s }));

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response('Internal Server Error', { status: 500 })
    );

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    unsubscribe();

    const lastStatus = statusSnapshots[statusSnapshots.length - 1];
    expect(lastStatus.state).toBe('error');
    expect(lastStatus.message).toBe('Sync failed');
    expect(lastStatus.canRetry).toBe(true);
  }, 10000);

  it('should not set syncStatus to syncing when there are no unsynced events', async () => {
    await initStorage();

    const statusSnapshots: Array<{ state: string }> = [];
    const unsubscribe = syncStatus.subscribe(s => statusSnapshots.push({ ...s }));

    globalThis.fetch = vi.fn();

    const { flushPendingEvents } = await import('../event-sync');
    await flushPendingEvents();

    unsubscribe();

    // Should never have reached 'syncing' state
    const states = statusSnapshots.map(s => s.state);
    expect(states).not.toContain('syncing');
    expect(globalThis.fetch).not.toHaveBeenCalled();
  }, 10000);
});
