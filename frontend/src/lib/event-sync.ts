import {
  getUnsyncedEvents,
  markEventsSynced,
  getQueuedOperations,
  deleteQueuedOperation,
  getAllLocalGoals,
  saveSyncEvent,
} from './storage';
import type { SyncEvent } from './events';
import { getToken } from './token-storage';
import { get } from 'svelte/store';
import { authStore } from './stores';
import { getApiBase } from './config';

const FLUSH_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes safety net

let isFlushing = false;

/**
 * Send a single event to the server immediately (fire-and-forget).
 * If offline or the request fails, the event stays unsynced in IndexedDB
 * and will be retried by the next flushPendingEvents call.
 */
export async function sendEvent(event: SyncEvent): Promise<void> {
  if (typeof navigator !== 'undefined' && !navigator.onLine) return;

  const auth = get(authStore);
  if (auth.type !== 'authenticated') return;

  try {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = await getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const res = await fetch(`${getApiBase()}/events/`, {
      method: 'POST',
      credentials: 'include',
      headers,
      body: JSON.stringify({
        events: [{
          id: event.id,
          type: event.type,
          timestamp: event.timestamp,
          payload: event.payload,
        }],
      }),
    });

    if (res.ok) {
      const data = await res.json();
      if (data.processed?.includes(event.id)) {
        await markEventsSynced([event.id]);
      }
    }
    // On failure: event stays unsynced, flush will retry
  } catch {
    // Network error: event stays unsynced
  }
}

/**
 * Flush all pending (unsynced) events to the server in a single batch.
 * Called on reconnect, on a timer, and at startup.
 */
export async function flushPendingEvents(): Promise<void> {
  if (isFlushing) return;
  if (typeof navigator !== 'undefined' && !navigator.onLine) return;

  const auth = get(authStore);
  if (auth.type !== 'authenticated') return;

  isFlushing = true;
  try {
    const events = await getUnsyncedEvents();
    if (events.length === 0) return;

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = await getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const res = await fetch(`${getApiBase()}/events/`, {
      method: 'POST',
      credentials: 'include',
      headers,
      body: JSON.stringify({
        events: events.map(e => ({
          id: e.id,
          type: e.type,
          timestamp: e.timestamp,
          payload: e.payload,
        })),
      }),
    });

    if (res.ok) {
      const data = await res.json();
      if (data.processed?.length > 0) {
        await markEventsSynced(data.processed);
      }
    }
  } catch {
    // Will retry on next flush
  } finally {
    isFlushing = false;
  }
}

// --- Migration: convert old QueuedOperation entries to SyncEvents ---

export async function migrateOldQueue(): Promise<void> {
  const oldOps = await getQueuedOperations();
  if (oldOps.length === 0) return;

  for (const op of oldOps) {
    try {
      let event: SyncEvent | null = null;

      switch (op.type) {
        case 'create_goal':
        case 'update_goal': {
          const goals = await getAllLocalGoals();
          const goal = goals.find(g => g.id === op.entityId);
          if (goal) {
            event = {
              id: crypto.randomUUID(),
              type: 'goal_upsert',
              timestamp: op.timestamp,
              synced: false,
              payload: {
                id: goal.id,
                name: goal.name,
                color: goal.color,
                position: goal.position,
                target_count: goal.target_count,
                target_period: goal.target_period,
              },
            };
          }
          break;
        }
        case 'delete_goal': {
          event = {
            id: crypto.randomUUID(),
            type: 'goal_delete',
            timestamp: op.timestamp,
            synced: false,
            payload: { id: op.entityId },
          };
          break;
        }
        case 'create_completion': {
          event = {
            id: crypto.randomUUID(),
            type: 'completion_set',
            timestamp: op.timestamp,
            synced: false,
            payload: { goal_id: op.payload.goal_id, date: op.payload.date },
          };
          break;
        }
        case 'delete_completion': {
          event = {
            id: crypto.randomUUID(),
            type: 'completion_unset',
            timestamp: op.timestamp,
            synced: false,
            payload: { goal_id: op.payload.goal_id, date: op.payload.date },
          };
          break;
        }
        case 'reorder_goals': {
          const goals = await getAllLocalGoals();
          const orderedIds: string[] = op.payload.goal_ids;
          for (let i = 0; i < orderedIds.length; i++) {
            const goal = goals.find(g => g.id === orderedIds[i]);
            if (goal) {
              const reorderEvent: SyncEvent = {
                id: crypto.randomUUID(),
                type: 'goal_upsert',
                timestamp: op.timestamp,
                synced: false,
                payload: {
                  id: goal.id,
                  name: goal.name,
                  color: goal.color,
                  position: i + 1,
                  target_count: goal.target_count,
                  target_period: goal.target_period,
                },
              };
              await saveSyncEvent(reorderEvent);
            }
          }
          break;
        }
      }

      if (event) {
        await saveSyncEvent(event);
      }

      await deleteQueuedOperation(op.id);
    } catch (error) {
      console.error(`Failed to migrate operation ${op.id}:`, error);
      // Continue with remaining operations — don't let one failure block the rest
      try {
        await deleteQueuedOperation(op.id);
      } catch {
        // Best effort cleanup
      }
    }
  }

  console.log(`Migrated ${oldOps.length} old queue operations to sync events`);
}

// --- Lifecycle: online listener + periodic flush ---

let flushIntervalId: ReturnType<typeof setInterval> | null = null;

function onOnline(): void {
  flushPendingEvents().catch(console.error);
}

/**
 * Start the event sync system: migrate old queue entries, flush immediately,
 * listen for online events, and set up a periodic flush interval as a safety net.
 */
export async function startEventSync(): Promise<void> {
  // One-time migration of old QueuedOperation entries to SyncEvents
  try {
    await migrateOldQueue();
  } catch (err) {
    console.error('Migration of old queue failed:', err);
  }

  // Flush any events that accumulated while offline / between sessions
  flushPendingEvents().catch(console.error);

  // Listen for reconnect
  if (typeof window !== 'undefined') {
    window.addEventListener('online', onOnline);
  }

  // Periodic flush as a safety net (e.g. if a sendEvent silently failed)
  if (flushIntervalId === null) {
    flushIntervalId = setInterval(() => {
      flushPendingEvents().catch(console.error);
    }, FLUSH_INTERVAL_MS);
  }
}

/**
 * Tear down the event sync system: remove listener and clear interval.
 */
export function stopEventSync(): void {
  if (typeof window !== 'undefined') {
    window.removeEventListener('online', onOnline);
  }

  if (flushIntervalId !== null) {
    clearInterval(flushIntervalId);
    flushIntervalId = null;
  }
}
