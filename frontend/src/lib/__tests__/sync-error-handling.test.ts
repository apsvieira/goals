import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';
import { resetDB } from '../storage';
import * as storage from '../storage';

describe('SyncManager Error Handling', () => {
  let consoleErrorSpy: any;
  let consoleLogSpy: any;

  beforeEach(() => {
    resetDB();
    // Mock console methods to verify error logging
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    consoleLogSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
  });

  afterEach(async () => {
    consoleErrorSpy.mockRestore();
    consoleLogSpy.mockRestore();
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should handle storage initialization errors gracefully and continue in degraded mode', async () => {
    // Mock initStorage to throw an error
    const mockError = new Error('Mock storage initialization failure');
    vi.spyOn(storage, 'initStorage').mockRejectedValue(mockError);

    // Import SyncManager
    const { syncManager } = await import('../sync');

    // Try to initialize - should not throw
    await syncManager.init();

    // Verify error was logged with proper prefix
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'SyncManager: Failed to initialize storage:',
      'Mock storage initialization failure'
    );
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'SyncManager: Sync functionality will be disabled. Error details:',
      mockError
    );

    // Try to sync - should skip due to storage not initialized
    await syncManager.sync();

    // Verify sync was skipped
    expect(consoleLogSpy).toHaveBeenCalledWith(
      'Storage not initialized, skipping sync'
    );
  });

  it('should allow normal operation when storage initializes successfully', async () => {
    // Don't mock - let storage initialize normally
    const { syncManager } = await import('../sync');

    // Initialize - should succeed
    await syncManager.init();

    // Verify no errors were logged
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.stringContaining('Failed to initialize storage')
    );
  });
});
