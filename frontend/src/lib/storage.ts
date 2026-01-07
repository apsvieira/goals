import { openDB, type DBSchema, type IDBPDatabase } from 'idb';
import type { Goal, Completion } from './api';

export interface QueuedOperation {
  id: string;
  type: 'create_goal' | 'update_goal' | 'delete_goal' |
        'create_completion' | 'delete_completion' | 'reorder_goals';
  entityId: string;
  payload: any;
  timestamp: string;
  retryCount: number;
}

interface GoalTrackerDB extends DBSchema {
  goals: {
    key: string;
    value: Goal;
    indexes: { 'by-position': number };
  };
  completions: {
    key: string;
    value: Completion;
    indexes: { 'by-goal': string; 'by-date': string };
  };
  meta: {
    key: string;
    value: { key: string; value: string | null };
  };
  operations: {
    key: string;
    value: QueuedOperation;
    indexes: { 'by-timestamp': string };
  };
}

const DB_NAME = 'goal-tracker';
const DB_VERSION = 2;

let db: IDBPDatabase<GoalTrackerDB> | null = null;

export async function initStorage(): Promise<void> {
  if (db) return;

  db = await openDB<GoalTrackerDB>(DB_NAME, DB_VERSION, {
    upgrade(database, oldVersion) {
      // Create stores only if they don't exist
      if (oldVersion < 1) {
        // Goals store
        const goalsStore = database.createObjectStore('goals', { keyPath: 'id' });
        goalsStore.createIndex('by-position', 'position');

        // Completions store
        const completionsStore = database.createObjectStore('completions', { keyPath: 'id' });
        completionsStore.createIndex('by-goal', 'goal_id');
        completionsStore.createIndex('by-date', 'date');

        // Meta store for sync info
        database.createObjectStore('meta', { keyPath: 'key' });
      }

      // Add operations store in version 2
      if (oldVersion < 2) {
        const operationsStore = database.createObjectStore('operations', { keyPath: 'id' });
        operationsStore.createIndex('by-timestamp', 'timestamp');
      }
    },
  });
}

function getDB(): IDBPDatabase<GoalTrackerDB> {
  if (!db) {
    throw new Error('Storage not initialized. Call initStorage() first.');
  }
  return db;
}

// Goals operations
export async function getLocalGoals(): Promise<Goal[]> {
  const database = getDB();
  const goals = await database.getAllFromIndex('goals', 'by-position');
  // Filter out archived goals for normal display
  return goals.filter(g => !g.archived_at);
}

export async function saveLocalGoal(goal: Goal): Promise<void> {
  const database = getDB();
  await database.put('goals', goal);
}

export async function deleteLocalGoal(id: string): Promise<void> {
  const database = getDB();
  await database.delete('goals', id);
}

export async function getAllLocalGoals(): Promise<Goal[]> {
  const database = getDB();
  return database.getAllFromIndex('goals', 'by-position');
}

// Completions operations
export async function getLocalCompletions(month: string): Promise<Completion[]> {
  const database = getDB();
  const allCompletions = await database.getAll('completions');
  // Filter completions by month (date format: YYYY-MM-DD)
  return allCompletions.filter(c => c.date.startsWith(month));
}

export async function saveLocalCompletion(completion: Completion): Promise<void> {
  const database = getDB();
  await database.put('completions', completion);
}

export async function deleteLocalCompletion(id: string): Promise<void> {
  const database = getDB();
  await database.delete('completions', id);
}

export async function getLocalCompletionByGoalAndDate(goalId: string, date: string): Promise<Completion | undefined> {
  const database = getDB();
  const allCompletions = await database.getAllFromIndex('completions', 'by-goal', goalId);
  return allCompletions.find(c => c.date === date);
}

// Meta operations
export async function getLastSyncedAt(): Promise<string | null> {
  const database = getDB();
  const meta = await database.get('meta', 'lastSyncedAt');
  return meta?.value ?? null;
}

export async function setLastSyncedAt(timestamp: string): Promise<void> {
  const database = getDB();
  await database.put('meta', { key: 'lastSyncedAt', value: timestamp });
}

// Clear all local data
export async function clearLocalData(): Promise<void> {
  const database = getDB();
  await database.clear('goals');
  await database.clear('completions');
  await database.clear('meta');
}

// Get max position for new goals
export async function getMaxPosition(): Promise<number> {
  const goals = await getLocalGoals();
  if (goals.length === 0) return 0;
  return Math.max(...goals.map(g => g.position));
}

// Get ALL local completions (for sync)
export async function getAllLocalCompletions(): Promise<Completion[]> {
  const database = getDB();
  return database.getAll('completions');
}

// Delete completion by goal and date
export async function deleteLocalCompletionByGoalAndDate(goalId: string, date: string): Promise<void> {
  const database = getDB();
  const completion = await getLocalCompletionByGoalAndDate(goalId, date);
  if (completion) {
    await database.delete('completions', completion.id);
  }
}

// Operation queue operations
export async function saveQueuedOperation(operation: QueuedOperation): Promise<void> {
  const database = getDB();
  await database.put('operations', operation);
}

export async function getQueuedOperations(): Promise<QueuedOperation[]> {
  const database = getDB();
  const operations = await database.getAllFromIndex('operations', 'by-timestamp');
  return operations.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
}

export async function deleteQueuedOperation(id: string): Promise<void> {
  const database = getDB();
  await database.delete('operations', id);
}

export async function clearQueuedOperations(): Promise<void> {
  const database = getDB();
  await database.clear('operations');
}
