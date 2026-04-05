import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';
import { resetDB, initStorage, saveLocalGoal, saveQueuedOperation, clearQueuedOperations } from '../storage';
import type { Goal, CalendarResponse } from '../api';

describe('getCalendar merges unsynced local goals', () => {
  beforeEach(async () => {
    resetDB();
    await initStorage();
    await clearQueuedOperations();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
    vi.restoreAllMocks();
  });

  it('should include locally-created goals that have not been synced yet', async () => {
    // Simulate a goal created locally but not yet synced to the server
    const unsyncedGoal: Goal = {
      id: 'local-123-abc',
      name: 'Exercise',
      color: '#5B8C5A',
      position: 2,
      created_at: '2026-04-05T10:00:00Z',
    };
    await saveLocalGoal(unsyncedGoal);

    // Queue the pending create operation (sync hasn't run yet)
    await saveQueuedOperation({
      id: 'op-1',
      type: 'create_goal',
      entityId: 'local-123-abc',
      payload: { name: 'Exercise', color: '#5B8C5A' },
      timestamp: '2026-04-05T10:00:00Z',
      retryCount: 0,
    });

    // Server only knows about a previously-synced goal
    const serverResponse: CalendarResponse = {
      goals: [
        { id: 'server-goal-1', name: 'Read', color: '#708090', position: 1, created_at: '2026-03-01T00:00:00Z' },
      ],
      completions: [],
    };

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(serverResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { getCalendar } = await import('../api');
    const result = await getCalendar('2026-03');

    // Should contain both the server goal AND the unsynced local goal
    expect(result.goals).toHaveLength(2);
    expect(result.goals.map(g => g.id)).toContain('server-goal-1');
    expect(result.goals.map(g => g.id)).toContain('local-123-abc');
  });

  it('should not duplicate a goal that has already been synced', async () => {
    // Goal exists both locally and on server (sync completed)
    const goal: Goal = {
      id: 'server-goal-1',
      name: 'Read',
      color: '#708090',
      position: 1,
      created_at: '2026-03-01T00:00:00Z',
    };
    await saveLocalGoal(goal);

    // A pending create for a DIFFERENT goal (shouldn't cause duplication)
    await saveQueuedOperation({
      id: 'op-1',
      type: 'create_goal',
      entityId: 'server-goal-1',
      payload: { name: 'Read', color: '#708090' },
      timestamp: '2026-03-01T00:00:00Z',
      retryCount: 0,
    });

    const serverResponse: CalendarResponse = {
      goals: [goal],
      completions: [],
    };

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(serverResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { getCalendar } = await import('../api');
    const result = await getCalendar('2026-03');

    // Should NOT duplicate the goal
    expect(result.goals).toHaveLength(1);
    expect(result.goals[0].id).toBe('server-goal-1');
  });

  it('should not include goals without pending create operations', async () => {
    // A goal exists locally but has NO pending create (it was synced and the op was cleared)
    await saveLocalGoal({
      id: 'synced-goal',
      name: 'Meditate',
      color: '#5B8C5A',
      position: 1,
      created_at: '2026-03-01T00:00:00Z',
    });

    // No pending operations

    const serverResponse: CalendarResponse = {
      goals: [
        { id: 'server-goal-1', name: 'Read', color: '#708090', position: 1, created_at: '2026-03-01T00:00:00Z' },
      ],
      completions: [],
    };

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(serverResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { getCalendar } = await import('../api');
    const result = await getCalendar('2026-03');

    // Should only return server data, not inject unrelated local goals
    expect(result.goals).toHaveLength(1);
    expect(result.goals[0].id).toBe('server-goal-1');
  });

  it('should exclude locally archived goals that server still returns', async () => {
    // Goal was archived locally but sync hasn't told the server yet
    await saveLocalGoal({
      id: 'goal-to-archive',
      name: 'Old Habit',
      color: '#708090',
      position: 2,
      created_at: '2026-02-01T00:00:00Z',
      archived_at: '2026-04-05T12:00:00Z',
    });

    // Pending delete_goal operation (sync hasn't run)
    await saveQueuedOperation({
      id: 'op-del-1',
      type: 'delete_goal',
      entityId: 'goal-to-archive',
      payload: {},
      timestamp: '2026-04-05T12:00:00Z',
      retryCount: 0,
    });

    // Server still returns the goal as active (it doesn't know about the archive)
    const serverResponse: CalendarResponse = {
      goals: [
        { id: 'goal-keep', name: 'Read', color: '#5B8C5A', position: 1, created_at: '2026-03-01T00:00:00Z' },
        { id: 'goal-to-archive', name: 'Old Habit', color: '#708090', position: 2, created_at: '2026-02-01T00:00:00Z' },
      ],
      completions: [],
    };

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(serverResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { getCalendar } = await import('../api');
    const result = await getCalendar('2026-03');

    // Archived goal should be filtered out
    expect(result.goals).toHaveLength(1);
    expect(result.goals[0].id).toBe('goal-keep');
  });

  it('should handle server returning null goals array', async () => {
    // Edge case: server returns null/undefined for goals
    await saveLocalGoal({
      id: 'local-goal',
      name: 'New Goal',
      color: '#5B8C5A',
      position: 1,
      created_at: '2026-04-05T10:00:00Z',
    });

    await saveQueuedOperation({
      id: 'op-create',
      type: 'create_goal',
      entityId: 'local-goal',
      payload: { name: 'New Goal', color: '#5B8C5A' },
      timestamp: '2026-04-05T10:00:00Z',
      retryCount: 0,
    });

    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ goals: null, completions: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const { getCalendar } = await import('../api');
    const result = await getCalendar('2026-04');

    // Should still include the unsynced goal despite null server goals
    expect(result.goals).toHaveLength(1);
    expect(result.goals[0].id).toBe('local-goal');
  });
});
