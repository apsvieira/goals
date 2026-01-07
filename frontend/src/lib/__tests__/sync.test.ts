import { describe, it, expect, beforeEach, vi } from 'vitest';
import { syncManager } from '../sync';
import { saveQueuedOperation, getQueuedOperations, clearQueuedOperations, initStorage } from '../storage';

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
