import { writable, get } from 'svelte/store';
import { authStore } from './stores';
import {
  initStorage,
  getAllLocalGoals,
  saveLocalGoal,
  getLastSyncedAt,
  setLastSyncedAt,
  saveLocalCompletion,
  getAllLocalCompletions,
  deleteLocalCompletionByGoalAndDate,
  getQueuedOperations,
  deleteQueuedOperation,
  type QueuedOperation,
} from './storage';
import { generateCompletionId, type Goal, type Completion } from './api';
import { Capacitor } from '@capacitor/core';
import { getToken } from './token-storage';

// Sync status store for UI feedback
export type SyncStatus =
  | { state: 'idle' }
  | { state: 'syncing'; message: string }
  | { state: 'success'; message: string }
  | { state: 'error'; message: string; canRetry: boolean };

export const syncStatus = writable<SyncStatus>({ state: 'idle' });

const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

function getApiBase(): string {
  if (Capacitor.isNativePlatform()) {
    return `${PRODUCTION_API_URL}/api/v1`;
  }
  return '/api/v1';
}

const API_BASE = getApiBase();

interface GoalChange {
  id: string;
  name: string;
  color: string;
  position: number;
  target_count?: number;
  target_period?: 'week' | 'month';
  updated_at: string;
  deleted: boolean;
  archived: boolean;
}

interface CompletionChange {
  goal_id: string;
  date: string;
  completed: boolean;
  updated_at: string;
}

interface SyncRequest {
  last_synced_at: string | null;
  goals: GoalChange[];
  completions: CompletionChange[];
}

interface SyncResponse {
  server_time: string;
  goals: GoalChange[];
  completions: CompletionChange[];
}

class SyncManager {
  private lastSyncedAt: Date | null = null;
  private isSyncing = false;
  private syncIntervalId: number | null = null;
  private readonly SYNC_INTERVAL_MS = 2 * 60 * 1000; // 2 minutes
  private storageInitialized = false;

  async init(): Promise<void> {
    try {
      await initStorage();
      this.storageInitialized = true;

      const lastSynced = await getLastSyncedAt();
      if (lastSynced) {
        this.lastSyncedAt = new Date(lastSynced);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      console.error('SyncManager: Failed to initialize storage:', errorMessage);
      console.error('SyncManager: Sync functionality will be disabled. Error details:', error);

      // Mark storage as not initialized, which will disable sync
      this.storageInitialized = false;

      // Don't throw - allow app to continue in degraded mode
      // Sync will be skipped when storage is not initialized
    }
  }

  startAutoSync(): void {
    if (this.syncIntervalId !== null) {
      return; // Already running
    }

    // Sync immediately
    this.sync().catch(console.error);

    // Then sync every 2 minutes
    this.syncIntervalId = window.setInterval(() => {
      this.sync().catch(console.error);
    }, this.SYNC_INTERVAL_MS);
  }

  stopAutoSync(): void {
    if (this.syncIntervalId !== null) {
      clearInterval(this.syncIntervalId);
      this.syncIntervalId = null;
    }
  }

  isAuthenticated(): boolean {
    const auth = get(authStore);
    return auth.type === 'authenticated';
  }

  async sync(): Promise<void> {
    if (this.isSyncing) {
      console.log('Sync already in progress, skipping');
      return;
    }

    if (!this.storageInitialized) {
      console.log('Storage not initialized, skipping sync');
      return;
    }

    if (!this.isAuthenticated()) {
      console.log('Not authenticated, skipping sync');
      return;
    }

    // Check if online
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      console.log('Offline, skipping sync');
      return;
    }

    this.isSyncing = true;
    syncStatus.set({ state: 'syncing', message: 'Syncing your data...' });

    try {
      // Get queued operations
      const operations = await getQueuedOperations();

      // Convert operations to sync format
      const goalChanges: GoalChange[] = [];
      const completionChanges: CompletionChange[] = [];

      for (const op of operations) {
        if (op.type === 'create_goal' || op.type === 'update_goal') {
          const goals = await getAllLocalGoals();
          const goal = goals.find(g => g.id === op.entityId);
          if (goal) {
            goalChanges.push({
              id: goal.id,
              name: goal.name,
              color: goal.color,
              position: goal.position,
              target_count: goal.target_count,
              target_period: goal.target_period,
              updated_at: op.timestamp,
              deleted: false,
              archived: !!goal.archived_at,
            });
          }
        } else if (op.type === 'delete_goal') {
          // Send as deleted
          const goals = await getAllLocalGoals();
          const goal = goals.find(g => g.id === op.entityId);
          if (goal) {
            goalChanges.push({
              id: goal.id,
              name: goal.name,
              color: goal.color,
              position: goal.position,
              target_count: goal.target_count,
              target_period: goal.target_period,
              updated_at: op.timestamp,
              deleted: true,
              archived: false,
            });
          }
        } else if (op.type === 'create_completion') {
          const completions = await getAllLocalCompletions();
          const completion = completions.find(c => c.id === op.entityId);
          if (completion) {
            completionChanges.push({
              goal_id: completion.goal_id,
              date: completion.date,
              completed: true,
              updated_at: op.timestamp,
            });
          }
        } else if (op.type === 'delete_completion') {
          // Parse goal_id and date from payload
          const { goal_id, date } = op.payload;
          completionChanges.push({
            goal_id,
            date,
            completed: false,
            updated_at: op.timestamp,
          });
        } else if (op.type === 'reorder_goals') {
          // Convert reorder to individual goal changes with updated positions
          const goals = await getAllLocalGoals();
          const orderedIds: string[] = op.payload.goal_ids;
          for (let i = 0; i < orderedIds.length; i++) {
            const goal = goals.find(g => g.id === orderedIds[i]);
            if (goal) {
              goalChanges.push({
                id: goal.id,
                name: goal.name,
                color: goal.color,
                position: i + 1,
                target_count: goal.target_count,
                target_period: goal.target_period,
                updated_at: op.timestamp,
                deleted: false,
                archived: !!goal.archived_at,
              });
            }
          }
        }
      }

      // Deduplicate goal changes — keep last entry per ID (later ops win)
      const goalChangeMap = new Map<string, GoalChange>();
      for (const change of goalChanges) {
        goalChangeMap.set(change.id, change);
      }
      const deduplicatedGoalChanges = Array.from(goalChangeMap.values());

      // Send to server
      const req: SyncRequest = {
        last_synced_at: this.lastSyncedAt?.toISOString() ?? null,
        goals: deduplicatedGoalChanges,
        completions: completionChanges,
      };

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      const token = await getToken();
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const res = await fetch(`${API_BASE}/sync/`, {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify(req),
      });

      if (!res.ok) {
        const errorText = await res.text().catch(() => 'Unable to read error response');
        throw new Error(`Sync failed (${res.status}): ${errorText}`);
      }

      const response: SyncResponse = await res.json();

      // Apply server changes
      await this.applyServerChanges(response);

      // Clear synced operations
      for (const op of operations) {
        await deleteQueuedOperation(op.id);
      }

      // Update lastSyncedAt
      this.lastSyncedAt = new Date(response.server_time);
      await setLastSyncedAt(response.server_time);

      console.log('Sync completed successfully');
      syncStatus.set({ state: 'idle' });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      console.error('Sync failed:', errorMessage);
      syncStatus.set({
        state: 'error',
        message: `Sync failed: ${errorMessage}`,
        canRetry: true
      });
    } finally {
      this.isSyncing = false;
    }
  }

  private async applyServerChanges(response: SyncResponse): Promise<void> {
    // Apply goal changes from server
    for (const goalChange of response.goals ?? []) {
      if (goalChange.deleted) {
        // For deleted goals, we mark them as archived locally
        const goal: Goal = {
          id: goalChange.id,
          name: goalChange.name,
          color: goalChange.color,
          position: goalChange.position,
          target_count: goalChange.target_count,
          target_period: goalChange.target_period,
          created_at: goalChange.updated_at,
          archived_at: goalChange.updated_at,
        };
        await saveLocalGoal(goal);
      } else {
        const goal: Goal = {
          id: goalChange.id,
          name: goalChange.name,
          color: goalChange.color,
          position: goalChange.position,
          target_count: goalChange.target_count,
          target_period: goalChange.target_period,
          created_at: goalChange.updated_at,
          archived_at: goalChange.archived ? goalChange.updated_at : undefined,
        };
        await saveLocalGoal(goal);
      }
    }

    // Apply completion changes from server
    for (const compChange of response.completions ?? []) {
      if (compChange.completed) {
        // Create or update completion
        const completion: Completion = {
          id: generateCompletionId(compChange.goal_id, compChange.date),
          goal_id: compChange.goal_id,
          date: compChange.date,
          created_at: compChange.updated_at,
        };
        await saveLocalCompletion(completion);
      } else {
        // Server says this completion was deleted, remove from local
        await deleteLocalCompletionByGoalAndDate(compChange.goal_id, compChange.date);
      }
    }
  }

  async linkAccount(): Promise<void> {
    // Upload all local data to server on first sync after login
    // This is essentially a full sync with no last_synced_at
    syncStatus.set({ state: 'syncing', message: 'Migrating your guest data...' });
    this.lastSyncedAt = null;
    await this.sync();
  }

  async retry(): Promise<void> {
    syncStatus.set({ state: 'idle' });
    try {
      await this.sync();
    } catch {
      // Error is already handled in sync()
    }
  }

  dismissError(): void {
    syncStatus.set({ state: 'idle' });
  }

  getLastSyncedAt(): Date | null {
    return this.lastSyncedAt;
  }
}

export const syncManager = new SyncManager();
