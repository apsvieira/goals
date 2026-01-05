import { get } from 'svelte/store';
import { authStore } from './stores';
import {
  initStorage,
  getAllLocalGoals,
  saveLocalGoal,
  getLastSyncedAt,
  setLastSyncedAt,
  saveLocalCompletion,
} from './storage';
import type { Goal, Completion } from './api';

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

    try {
      // 1. Get pending local changes (all local goals and completions)
      const localGoals = await getAllLocalGoals();

      // Convert local goals to GoalChanges
      const goalChanges: GoalChange[] = localGoals.map(goal => ({
        id: goal.id,
        name: goal.name,
        color: goal.color,
        position: goal.position,
        updated_at: goal.created_at, // Use created_at as updated_at for local-only goals
        deleted: !!goal.archived_at,
      }));

      // For now, we don't track completion updates in local storage
      // We'll send empty completions and just receive server state
      const completionChanges: CompletionChange[] = [];

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
    } catch (error) {
      console.error('Sync failed:', error);
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
      }
      // For deleted completions, we'd need a deleteLocalCompletion by goal+date
      // For now, we'll let the next full refresh handle it
    }
  }

  async linkAccount(): Promise<void> {
    // Upload all local data to server on first sync after login
    // This is essentially a full sync with no last_synced_at
    this.lastSyncedAt = null;
    await this.sync();
  }

  getLastSyncedAt(): Date | null {
    return this.lastSyncedAt;
  }
}

export const syncManager = new SyncManager();
