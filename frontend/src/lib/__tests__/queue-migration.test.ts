import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { deleteDB } from 'idb';
import {
  initStorage,
  resetDB,
  getUnsyncedEvents,
} from '../storage';
import { migrateOldQueue } from '../event-sync';

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

  it('should be a no-op and not create any events', async () => {
    await initStorage();

    await migrateOldQueue();

    const events = await getUnsyncedEvents();
    expect(events).toHaveLength(0);
  }, 10000);
});
