import { describe, it, expect, beforeEach, vi } from 'vitest';
import { syncManager } from '../sync';
import { saveLocalGoal, getAllLocalGoals, saveQueuedOperation, getQueuedOperations, clearQueuedOperations, initStorage } from '../storage';
import { authStore } from '../stores';

describe('SyncManager', () => {
  beforeEach(async () => {
    // Initialize storage
    await initStorage();
    // Clear queue before each test
    await clearQueuedOperations();
  });

  it('should queue operations when offline', async () => {
    const operation = {
      id: 'test-op-1',
      type: 'create_goal' as const,
      entityId: 'goal-1',
      payload: { name: 'Test Goal', color: '#5B8C5A' },
      timestamp: new Date().toISOString(),
      retryCount: 0,
    };

    await saveQueuedOperation(operation);
    const queued = await getQueuedOperations();

    expect(queued).toHaveLength(1);
    expect(queued[0].id).toBe('test-op-1');
  });

  it('should process operations in order', async () => {
    const op1 = {
      id: 'op-1',
      type: 'create_goal' as const,
      entityId: 'goal-1',
      payload: {},
      timestamp: '2026-01-01T00:00:00Z',
      retryCount: 0,
    };

    const op2 = {
      id: 'op-2',
      type: 'update_goal' as const,
      entityId: 'goal-1',
      payload: {},
      timestamp: '2026-01-01T00:00:01Z',
      retryCount: 0,
    };

    await saveQueuedOperation(op2);
    await saveQueuedOperation(op1);

    const queued = await getQueuedOperations();
    expect(queued[0].id).toBe('op-1'); // Earlier timestamp first
    expect(queued[1].id).toBe('op-2');
  });
});

describe('SyncManager - reorder_goals', () => {
  beforeEach(async () => {
    await initStorage();
    await clearQueuedOperations();
  });

  it('should convert reorder_goals operations to goal changes with updated positions', async () => {
    // Save local goals with positions
    await saveLocalGoal({
      id: 'goal-a', name: 'A', color: '#FF0000', position: 1,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });
    await saveLocalGoal({
      id: 'goal-b', name: 'B', color: '#00FF00', position: 2,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });
    await saveLocalGoal({
      id: 'goal-c', name: 'C', color: '#0000FF', position: 3,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });

    // Queue a reorder that moves C to position 1: C, A, B
    await saveQueuedOperation({
      id: 'reorder-1',
      type: 'reorder_goals',
      entityId: 'reorder',
      payload: { goal_ids: ['goal-c', 'goal-a', 'goal-b'] },
      timestamp: '2026-01-02T00:00:00Z',
      retryCount: 0,
    });

    // Set auth state so sync proceeds
    authStore.set({ type: 'authenticated', user: { id: 'u1', email: 'test@test.com' } });

    // Ensure SyncManager storage is initialized
    await syncManager.init();

    // Trigger sync — intercept the fetch call to capture the request body
    let capturedBody: any = null;
    globalThis.fetch = vi.fn().mockImplementation(async (_url: string, init: any) => {
      capturedBody = JSON.parse(init.body);
      return new Response(JSON.stringify({
        server_time: '2026-01-02T00:00:01Z',
        goals: [],
        completions: [],
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });

    await syncManager.sync();

    expect(capturedBody).not.toBeNull();
    expect(capturedBody.goals).toHaveLength(3);

    // Verify positions match the reorder: C=1, A=2, B=3
    const goalById = Object.fromEntries(capturedBody.goals.map((g: any) => [g.id, g]));
    expect(goalById['goal-c'].position).toBe(1);
    expect(goalById['goal-a'].position).toBe(2);
    expect(goalById['goal-b'].position).toBe(3);
  });
});
