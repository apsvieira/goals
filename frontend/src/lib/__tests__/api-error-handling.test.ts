import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { deleteDB } from 'idb';
import { resetDB } from '../storage';
import * as storage from '../storage';

// Import the module to test - we'll test the error handling indirectly
// by mocking initStorage to throw an error

describe('API Error Handling', () => {
  let consoleErrorSpy: any;

  beforeEach(() => {
    resetDB();
    // Mock console.error to verify error logging
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(async () => {
    consoleErrorSpy.mockRestore();
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  it('should log errors when storage initialization fails', async () => {
    // Mock initStorage to throw an error
    const originalInitStorage = storage.initStorage;
    const mockError = new Error('Mock storage initialization failure');
    vi.spyOn(storage, 'initStorage').mockRejectedValue(mockError);

    // Dynamic import to get fresh module with mocked storage
    const api = await import('../api');

    // Try to call a function that requires storage initialization
    try {
      await api.getCalendar('2026-01');
    } catch (error) {
      // Expected to throw
      expect(error).toBeDefined();
      expect(error instanceof Error).toBe(true);
      if (error instanceof Error) {
        expect(error.message).toContain('Storage initialization failed');
      }
    }

    // Verify error was logged
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'Failed to initialize storage:',
      'Mock storage initialization failure'
    );
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'Storage operations will not work. Error details:',
      mockError
    );

    // Restore original function
    vi.spyOn(storage, 'initStorage').mockResolvedValue(originalInitStorage as any);
  });
});
