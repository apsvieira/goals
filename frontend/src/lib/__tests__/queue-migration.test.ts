import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { deleteDB } from 'idb';
import {
  initStorage,
  resetDB,
  saveQueuedOperation,
  getQueuedOperations,
  saveLocalGoal,
  getUnsyncedEvents,
} from '../storage';
import type { QueuedOperation } from '../storage';
import { migrateOldQueue } from '../event-sync';

function makeOp(overrides: Partial<QueuedOperation>): QueuedOperation {
  return {
    id: crypto.randomUUID(),
    type: 'create_goal',
    entityId: 'g-1',
    payload: {},
    timestamp: new Date().toISOString(),
    retryCount: 0,
    ...overrides,
  };
}

describe('migrateOldQueue', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('should migrate create_goal operation to goal_upsert event', async () => {
    await initStorage();

    // Seed a local goal that the operation references
    await saveLocalGoal({
      id: 'g-1',
      name: 'Exercise',
      color: '#5B8C5A',
      position: 1,
      target_count: 5,
      target_period: 'week',
      created_at: '2026-01-01T00:00:00Z',
    });

    // Seed old queued operation
    await saveQueuedOperation(makeOp({
      id: 'op-1',
      type: 'create_goal',
      entityId: 'g-1',
      payload: {},
      timestamp: '2026-04-01T10:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('goal_upsert');
    expect(events[0].timestamp).toBe('2026-04-01T10:00:00Z');
    expect(events[0].synced).toBe(false);
    expect(events[0].payload).toMatchObject({
      id: 'g-1',
      name: 'Exercise',
      color: '#5B8C5A',
      position: 1,
      target_count: 5,
      target_period: 'week',
    });
  }, 10000);

  it('should migrate update_goal operation to goal_upsert event', async () => {
    await initStorage();

    await saveLocalGoal({
      id: 'g-2',
      name: 'Read',
      color: '#708090',
      position: 2,
      created_at: '2026-01-01T00:00:00Z',
    });

    await saveQueuedOperation(makeOp({
      id: 'op-2',
      type: 'update_goal',
      entityId: 'g-2',
      payload: { name: 'Read' },
      timestamp: '2026-04-01T11:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('goal_upsert');
    expect(events[0].payload).toMatchObject({
      id: 'g-2',
      name: 'Read',
      color: '#708090',
      position: 2,
    });
  }, 10000);

  it('should migrate delete_goal operation to goal_delete event', async () => {
    await initStorage();

    await saveQueuedOperation(makeOp({
      id: 'op-3',
      type: 'delete_goal',
      entityId: 'g-del',
      payload: {},
      timestamp: '2026-04-01T12:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('goal_delete');
    expect(events[0].timestamp).toBe('2026-04-01T12:00:00Z');
    expect(events[0].payload).toEqual({ id: 'g-del' });
  }, 10000);

  it('should migrate create_completion to completion_set event', async () => {
    await initStorage();

    await saveQueuedOperation(makeOp({
      id: 'op-4',
      type: 'create_completion',
      entityId: 'c-1',
      payload: { goal_id: 'g-1', date: '2026-04-05' },
      timestamp: '2026-04-05T08:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('completion_set');
    expect(events[0].timestamp).toBe('2026-04-05T08:00:00Z');
    expect(events[0].payload).toEqual({ goal_id: 'g-1', date: '2026-04-05' });
  }, 10000);

  it('should migrate delete_completion to completion_unset event', async () => {
    await initStorage();

    await saveQueuedOperation(makeOp({
      id: 'op-5',
      type: 'delete_completion',
      entityId: 'c-1',
      payload: { goal_id: 'g-1', date: '2026-04-05' },
      timestamp: '2026-04-05T09:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('completion_unset');
    expect(events[0].timestamp).toBe('2026-04-05T09:00:00Z');
    expect(events[0].payload).toEqual({ goal_id: 'g-1', date: '2026-04-05' });
  }, 10000);

  it('should migrate reorder_goals to N goal_upsert events', async () => {
    await initStorage();

    await saveLocalGoal({ id: 'g-a', name: 'A', color: '#111', position: 1, created_at: '2026-01-01T00:00:00Z' });
    await saveLocalGoal({ id: 'g-b', name: 'B', color: '#222', position: 2, created_at: '2026-01-01T00:00:00Z' });
    await saveLocalGoal({ id: 'g-c', name: 'C', color: '#333', position: 3, created_at: '2026-01-01T00:00:00Z' });

    await saveQueuedOperation(makeOp({
      id: 'op-6',
      type: 'reorder_goals',
      entityId: '',
      payload: { goal_ids: ['g-c', 'g-a', 'g-b'] },
      timestamp: '2026-04-01T14:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(3);

    // All should be goal_upsert
    expect(events.every(e => e.type === 'goal_upsert')).toBe(true);

    // Verify positions match new order
    const byGoalId = Object.fromEntries(
      events.map(e => [e.payload.id, e.payload])
    );
    expect(byGoalId['g-c'].position).toBe(1);
    expect(byGoalId['g-a'].position).toBe(2);
    expect(byGoalId['g-b'].position).toBe(3);
  }, 10000);

  it('should handle empty queue without errors', async () => {
    await initStorage();

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(0);
  }, 10000);

  it('should delete old operations after migration', async () => {
    await initStorage();

    await saveLocalGoal({
      id: 'g-1',
      name: 'Exercise',
      color: '#5B8C5A',
      position: 1,
      created_at: '2026-01-01T00:00:00Z',
    });

    await saveQueuedOperation(makeOp({
      id: 'op-a',
      type: 'create_goal',
      entityId: 'g-1',
      timestamp: '2026-04-01T10:00:00Z',
    }));

    await saveQueuedOperation(makeOp({
      id: 'op-b',
      type: 'delete_goal',
      entityId: 'g-del',
      timestamp: '2026-04-01T11:00:00Z',
    }));

    await saveQueuedOperation(makeOp({
      id: 'op-c',
      type: 'create_completion',
      entityId: 'c-1',
      payload: { goal_id: 'g-1', date: '2026-04-05' },
      timestamp: '2026-04-01T12:00:00Z',
    }));

    // Verify operations exist before migration
    const opsBefore = await getQueuedOperations();
    expect(opsBefore).toHaveLength(3);

    await migrateOldQueue();

    // All old operations should be deleted
    const opsAfter = await getQueuedOperations();
    expect(opsAfter).toHaveLength(0);

    // Sync events should have been created
    const events = await getUnsyncedEvents();
    expect(events.length).toBeGreaterThanOrEqual(2);
  }, 10000);

  it('should include archived goals in migration lookup', async () => {
    await initStorage();

    // Save an archived goal
    await saveLocalGoal({
      id: 'g-archived',
      name: 'Old Habit',
      color: '#999',
      position: 5,
      created_at: '2025-01-01T00:00:00Z',
      archived_at: '2026-03-01T00:00:00Z',
    });

    await saveQueuedOperation(makeOp({
      id: 'op-archived',
      type: 'update_goal',
      entityId: 'g-archived',
      payload: {},
      timestamp: '2026-04-01T10:00:00Z',
    }));

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('goal_upsert');
    expect(events[0].payload).toMatchObject({
      id: 'g-archived',
      name: 'Old Habit',
    });
  }, 10000);

  it('should skip creating event when referenced goal is not found locally', async () => {
    await initStorage();

    // No local goal exists for this operation
    await saveQueuedOperation(makeOp({
      id: 'op-orphan',
      type: 'create_goal',
      entityId: 'g-missing',
      payload: {},
      timestamp: '2026-04-01T10:00:00Z',
    }));

    await migrateOldQueue();

    // No event should be created, but operation should still be deleted
    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(0);

    const ops = await getQueuedOperations();
    expect(ops).toHaveLength(0);
  }, 10000);
});
