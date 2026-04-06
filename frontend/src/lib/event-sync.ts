import {
  getUnsyncedEvents,
  markEventsSynced,
  pruneSyncedEvents,
} from './storage';
import type { SyncEvent } from './events';
import { getToken } from './token-storage';
import { get, writable } from 'svelte/store';
import { authStore } from './stores';
import { getApiBase } from './config';

export type SyncStatus =
  | { state: 'idle' }
  | { state: 'syncing'; message: string }
  | { state: 'error'; message: string; canRetry: boolean };

export const syncStatus = writable<SyncStatus>({ state: 'idle' });

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

    syncStatus.set({ state: 'syncing', message: 'Syncing...' });

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
      // Best-effort pruning of old synced events
      try {
        await pruneSyncedEvents(7);
      } catch {
        // Prune failure should not affect sync status
      }
    } else {
      syncStatus.set({ state: 'error', message: 'Sync failed', canRetry: true });
      return;
    }
  } catch {
    syncStatus.set({ state: 'error', message: 'Sync failed', canRetry: true });
    return;
  } finally {
    isFlushing = false;
  }
  syncStatus.set({ state: 'idle' });
}

// --- Lifecycle: online listener + periodic flush ---

let flushIntervalId: ReturnType<typeof setInterval> | null = null;

function onOnline(): void {
  flushPendingEvents().catch(console.error);
}

/**
 * Start the event sync system: flush immediately,
 * listen for online events, and set up a periodic flush interval as a safety net.
 */
export async function startEventSync(): Promise<void> {
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

  syncStatus.set({ state: 'idle' });
}
