import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { deleteDB } from 'idb';
import type { SyncEvent } from '../events';
import {
  initStorage,
  saveSyncEvent,
  getUnsyncedEvents,
  markEventsSynced,
  getSyncEvents,
  saveQueuedOperation,
  getQueuedOperations,
  saveLocalGoal,
  getLocalGoals,
  saveLocalCompletion,
  getLocalCompletions,
  clearLocalData,
  resetDB,
  type QueuedOperation,
} from '../storage';

function makeEvent(overrides: Partial<SyncEvent> = {}): SyncEvent {
  return {
    id: crypto.randomUUID(),
    type: 'goal_upsert',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: { id: 'g-1', name: 'Run', color: '#FF0000', position: 0 },
    ...overrides,
  };
}

describe('SyncEvent storage', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should save and retrieve a sync event', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-1', type: 'goal_upsert' });
    await saveSyncEvent(event);

    const events = await getSyncEvents();
    expect(events).toHaveLength(1);
    expect(events[0]).toEqual(event);
  }, 10000);

  it('should save multiple events and retrieve all', async () => {
    await initStorage();

    const e1 = makeEvent({ id: 'evt-1', type: 'goal_upsert' });
    const e2 = makeEvent({
      id: 'evt-2',
      type: 'completion_set',
      payload: { goal_id: 'g-1', date: '2026-04-05' },
    });
    const e3 = makeEvent({
      id: 'evt-3',
      type: 'goal_delete',
      payload: { id: 'g-2' },
    });

    await saveSyncEvent(e1);
    await saveSyncEvent(e2);
    await saveSyncEvent(e3);

    const events = await getSyncEvents();
    expect(events).toHaveLength(3);
  }, 10000);

  it('should overwrite an event with the same id (put semantics)', async () => {
    await initStorage();

    const event = makeEvent({ id: 'evt-1', type: 'goal_upsert' });
    await saveSyncEvent(event);

    const updated = { ...event, synced: true };
    await saveSyncEvent(updated);

    const events = await getSyncEvents();
    expect(events).toHaveLength(1);
    expect(events[0].synced).toBe(true);
  }, 10000);
});

describe('getUnsyncedEvents', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should return only unsynced events', async () => {
    await initStorage();

    const synced = makeEvent({ id: 'evt-synced', synced: true, timestamp: '2026-01-01T00:00:00Z' });
    const unsynced1 = makeEvent({ id: 'evt-u1', synced: false, timestamp: '2026-01-01T00:00:01Z' });
    const unsynced2 = makeEvent({ id: 'evt-u2', synced: false, timestamp: '2026-01-01T00:00:02Z' });

    await saveSyncEvent(synced);
    await saveSyncEvent(unsynced1);
    await saveSyncEvent(unsynced2);

    const result = await getUnsyncedEvents();
    expect(result).toHaveLength(2);
    expect(result.map((e) => e.id)).toEqual(['evt-u1', 'evt-u2']);
  }, 10000);

  it('should return unsynced events ordered by timestamp', async () => {
    await initStorage();

    const e3 = makeEvent({ id: 'evt-3', synced: false, timestamp: '2026-01-01T00:00:03Z' });
    const e1 = makeEvent({ id: 'evt-1', synced: false, timestamp: '2026-01-01T00:00:01Z' });
    const e2 = makeEvent({ id: 'evt-2', synced: false, timestamp: '2026-01-01T00:00:02Z' });

    // Save in non-chronological order
    await saveSyncEvent(e3);
    await saveSyncEvent(e1);
    await saveSyncEvent(e2);

    const result = await getUnsyncedEvents();
    expect(result.map((e) => e.id)).toEqual(['evt-1', 'evt-2', 'evt-3']);
  }, 10000);

  it('should return empty array when all events are synced', async () => {
    await initStorage();

    const synced1 = makeEvent({ id: 'evt-1', synced: true });
    const synced2 = makeEvent({ id: 'evt-2', synced: true });

    await saveSyncEvent(synced1);
    await saveSyncEvent(synced2);

    const result = await getUnsyncedEvents();
    expect(result).toHaveLength(0);
  }, 10000);

  it('should return empty array when no events exist', async () => {
    await initStorage();

    const result = await getUnsyncedEvents();
    expect(result).toHaveLength(0);
  }, 10000);
});

describe('markEventsSynced', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should mark specified events as synced', async () => {
    await initStorage();

    const e1 = makeEvent({ id: 'evt-1', synced: false });
    const e2 = makeEvent({ id: 'evt-2', synced: false });
    const e3 = makeEvent({ id: 'evt-3', synced: false });

    await saveSyncEvent(e1);
    await saveSyncEvent(e2);
    await saveSyncEvent(e3);

    await markEventsSynced(['evt-1', 'evt-3']);

    const all = await getSyncEvents();
    const syncedMap = Object.fromEntries(all.map((e) => [e.id, e.synced]));
    expect(syncedMap['evt-1']).toBe(true);
    expect(syncedMap['evt-2']).toBe(false);
    expect(syncedMap['evt-3']).toBe(true);
  }, 10000);

  it('should handle marking non-existent ids gracefully', async () => {
    await initStorage();

    const e1 = makeEvent({ id: 'evt-1', synced: false });
    await saveSyncEvent(e1);

    // Should not throw when an ID doesn't exist
    await markEventsSynced(['evt-1', 'evt-nonexistent']);

    const all = await getSyncEvents();
    expect(all).toHaveLength(1);
    expect(all[0].synced).toBe(true);
  }, 10000);

  it('should handle empty ids array', async () => {
    await initStorage();

    const e1 = makeEvent({ id: 'evt-1', synced: false });
    await saveSyncEvent(e1);

    await markEventsSynced([]);

    const result = await getUnsyncedEvents();
    expect(result).toHaveLength(1);
  }, 10000);
});

describe('v2 to v3 upgrade preserves existing stores', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should preserve goals, completions, operations, and meta after upgrade', async () => {
    await initStorage();

    // Populate existing stores
    const goal = {
      id: 'g-1',
      name: 'Exercise',
      color: '#5B8C5A',
      position: 0,
      created_at: '2026-01-01T00:00:00Z',
    };
    await saveLocalGoal(goal);

    const completion = {
      id: 'c-1',
      goal_id: 'g-1',
      date: '2026-04-05',
      created_at: '2026-04-05T10:00:00Z',
    };
    await saveLocalCompletion(completion);

    const operation: QueuedOperation = {
      id: 'op-1',
      type: 'create_goal',
      entityId: 'g-1',
      payload: { name: 'Exercise' },
      timestamp: '2026-01-01T00:00:00Z',
      retryCount: 0,
    };
    await saveQueuedOperation(operation);

    // Verify pre-existing data is accessible
    const goals = await getLocalGoals();
    expect(goals).toHaveLength(1);
    expect(goals[0].name).toBe('Exercise');

    const completions = await getLocalCompletions('2026-04');
    expect(completions).toHaveLength(1);

    const ops = await getQueuedOperations();
    expect(ops).toHaveLength(1);

    // Verify the new events store works alongside existing stores
    const event = makeEvent({ id: 'evt-1' });
    await saveSyncEvent(event);

    const events = await getSyncEvents();
    expect(events).toHaveLength(1);

    // Verify old data is still intact
    const goalsAfter = await getLocalGoals();
    expect(goalsAfter).toHaveLength(1);

    const opsAfter = await getQueuedOperations();
    expect(opsAfter).toHaveLength(1);
  }, 10000);

  it('clearLocalData should clear events store too', async () => {
    await initStorage();

    await saveSyncEvent(makeEvent({ id: 'evt-1' }));
    await saveSyncEvent(makeEvent({ id: 'evt-2' }));

    let events = await getSyncEvents();
    expect(events).toHaveLength(2);

    await clearLocalData();

    events = await getSyncEvents();
    expect(events).toHaveLength(0);
  }, 10000);
});
