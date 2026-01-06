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
} from './storage';
import type { Goal, Completion } from './api';

// Sync status store for UI feedback
export type SyncStatus =
  | { state: 'idle' }
  | { state: 'syncing'; message: string }
  | { state: 'success'; message: string }
  | { state: 'error'; message: string; canRetry: boolean };

export const syncStatus = writable<SyncStatus>({ state: 'idle' });

// Use relative URL in production, absolute in dev
const API_BASE = typeof window !== 'undefined' && window.location.hostname !== 'localhost'
  ? '/api/v1'
  : 'http://localhost:8080/api/v1';

interface GoalChange {
  id: string;
  name: string;
  color: string;
  position: number;
  updated_at: string;
  deleted: boolean;
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

  async init(): Promise<void> {
    await initStorage();
    const lastSynced = await getLastSyncedAt();
    if (lastSynced) {
      this.lastSyncedAt = new Date(lastSynced);
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

    if (!this.isAuthenticated()) {
      console.log('Not authenticated, skipping sync');
      return;
    }

    this.isSyncing = true;
    syncStatus.set({ state: 'syncing', message: 'Syncing your data...' });

    try {
      // 1. Get pending local changes (all local goals and completions)
      const localGoals = await getAllLocalGoals();
      const localCompletions = await getAllLocalCompletions();

      // Convert local goals to GoalChanges
      const goalChanges: GoalChange[] = localGoals.map(goal => ({
        id: goal.id,
        name: goal.name,
        color: goal.color,
        position: goal.position,
        updated_at: goal.created_at, // Use created_at as updated_at for local-only goals
        deleted: !!goal.archived_at,
      }));

      // Convert local completions to CompletionChanges
      const completionChanges: CompletionChange[] = localCompletions.map(completion => ({
        goal_id: completion.goal_id,
        date: completion.date,
        completed: true, // All completions in local storage are completed
        updated_at: completion.created_at,
      }));

      // 2. POST to /api/v1/sync
      const req: SyncRequest = {
        last_synced_at: this.lastSyncedAt?.toISOString() ?? null,
        goals: goalChanges,
        completions: completionChanges,
      };

      const res = await fetch(`${API_BASE}/sync/`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(req),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`Sync failed: ${text || res.statusText}`);
      }

      const response: SyncResponse = await res.json();

      // 3. Apply server changes to local storage
      await this.applyServerChanges(response);

      // 4. Update lastSyncedAt
      this.lastSyncedAt = new Date(response.server_time);
      await setLastSyncedAt(response.server_time);

      console.log('Sync completed successfully');

      const totalItems = goalChanges.length + completionChanges.length;
      const message = totalItems > 0
        ? `Synced ${goalChanges.length} goals and ${completionChanges.length} completions`
        : 'Sync complete';
      syncStatus.set({ state: 'success', message });

      // Clear success status after 3 seconds
      setTimeout(() => {
        syncStatus.update(current => {
          if (current.state === 'success') {
            return { state: 'idle' };
          }
          return current;
        });
      }, 3000);
    } catch (error) {
      console.error('Sync failed:', error);
      const errorMessage = error instanceof Error ? error.message : 'Sync failed';
      syncStatus.set({ state: 'error', message: errorMessage, canRetry: true });
      throw error;
    } finally {
      this.isSyncing = false;
    }
  }

  private async applyServerChanges(response: SyncResponse): Promise<void> {
    // Apply goal changes from server
    for (const goalChange of response.goals) {
      if (goalChange.deleted) {
        // For deleted goals, we mark them as archived locally
        const goal: Goal = {
          id: goalChange.id,
          name: goalChange.name,
          color: goalChange.color,
          position: goalChange.position,
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
          created_at: goalChange.updated_at,
        };
        await saveLocalGoal(goal);
      }
    }

    // Apply completion changes from server
    for (const compChange of response.completions) {
      if (compChange.completed) {
        // Create or update completion
        const completion: Completion = {
          id: `${compChange.goal_id}-${compChange.date}`,
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
