// Phase 5: user-initiated debug report transport.
//
// Builds a payload, POSTs it to the backend, and falls back to an IndexedDB
// queue when offline. The breadcrumb snapshot is included at build time so
// reports always carry the most recent logs even if the queue drains later.
//
// This module is intentionally independent of Svelte/UI code so the same
// transport can be driven from the DebugReportModal today and from Sentry
// hooks (Phase 7) later.

import { openDB, type DBSchema, type IDBPDatabase } from 'idb';
import { Capacitor } from '@capacitor/core';
import { get } from 'svelte/store';
import { authStore, isOnline } from '../stores';
import { getApiBase } from '../config';
import { getToken } from '../token-storage';
import { snapshot, clear } from './breadcrumbs';
import { getUnsyncedEvents } from '../storage';

// ---------- Types ----------

export type DebugReportTrigger = 'shake' | 'auto';
export type DebugReportPlatform = 'android' | 'ios' | 'web';

export interface DebugReportInput {
  description: string;
  trigger: DebugReportTrigger;
}

export interface DebugReportOutcome {
  outcome: 'sent' | 'queued' | 'rate_limited' | 'client_rate_limited';
  message?: string;
}

// Shape of the JSON body we POST. Mirrors the backend handler.
interface DebugReportPayload {
  client_id: string;
  app_version: string;
  platform: DebugReportPlatform;
  device: { model: string; os: string; webview: string };
  state: {
    route: string;
    online: boolean;
    pending_events: number;
    goal_count: number;
    auth_state: string;
    notif_permission: string;
  };
  description: string;
  breadcrumbs: unknown[];
  trigger: DebugReportTrigger;
  client_ts: number;
}

// ---------- Constants ----------

const CLIENT_ID_STORAGE_KEY = 'debug_client_id';
const LAST_TS_STORAGE_KEY = 'last_debug_report_ts';
const CLIENT_RATE_LIMIT_MS = 60_000;

const QUEUE_DB_NAME = 'goal-tracker-debug-queue';
const QUEUE_DB_VERSION = 1;
const QUEUE_STORE = 'debug_report_queue';
const QUEUE_MAX = 10;

// ---------- IDB schema ----------

interface DebugQueueDB extends DBSchema {
  debug_report_queue: {
    key: number;
    value: { id: number; payload: DebugReportPayload };
  };
}

async function openQueueDB(): Promise<IDBPDatabase<DebugQueueDB> | null> {
  if (typeof indexedDB === 'undefined') return null;
  try {
    return await openDB<DebugQueueDB>(QUEUE_DB_NAME, QUEUE_DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains(QUEUE_STORE)) {
          db.createObjectStore(QUEUE_STORE, {
            keyPath: 'id',
            autoIncrement: true,
          });
        }
      },
    });
  } catch {
    return null;
  }
}

// ---------- Route reader ----------
//
// App.svelte owns the `currentRoute` local; we expose a setter the app can
// call when the route changes so this module can quote it in state.route.
// Default 'home' matches App.svelte's initial value.
let currentRoute = 'home';

export function setDebugReportRoute(route: string): void {
  currentRoute = route;
}

// ---------- Client id ----------

const UUID_V4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function randomUuidV4(): string {
  // xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx where y ∈ {8,9,a,b}
  const hex = (n: number) => n.toString(16).padStart(2, '0');
  const rand = new Uint8Array(16);
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(rand);
  } else {
    for (let i = 0; i < 16; i++) rand[i] = Math.floor(Math.random() * 256);
  }
  rand[6] = (rand[6] & 0x0f) | 0x40; // version 4
  rand[8] = (rand[8] & 0x3f) | 0x80; // variant 10x
  const b = Array.from(rand, hex);
  return `${b[0]}${b[1]}${b[2]}${b[3]}-${b[4]}${b[5]}-${b[6]}${b[7]}-${b[8]}${b[9]}-${b[10]}${b[11]}${b[12]}${b[13]}${b[14]}${b[15]}`;
}

function getClientId(): string {
  try {
    const existing = localStorage.getItem(CLIENT_ID_STORAGE_KEY);
    if (existing && UUID_V4_REGEX.test(existing)) return existing;
    // If the stored value is missing or malformed (e.g. poisoned by a prior
    // build that used a non-UUID fallback), fall through to regenerate and
    // overwrite it. The backend calls uuid.Parse and will 400 on bad ids.
  } catch {
    // localStorage unavailable; fall through to generate a fresh one
  }
  const generated =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : randomUuidV4();
  try {
    localStorage.setItem(CLIENT_ID_STORAGE_KEY, generated);
  } catch {
    // ignore
  }
  return generated;
}

// ---------- Platform / device ----------

function getPlatform(): DebugReportPlatform {
  try {
    if (typeof Capacitor !== 'undefined' && Capacitor.isNativePlatform?.()) {
      const p = Capacitor.getPlatform?.();
      if (p === 'android' || p === 'ios') return p;
    }
  } catch {
    // ignore
  }
  return 'web';
}

function getDevice(): { model: string; os: string; webview: string } {
  // @capacitor/device is not in the dependency list; Phase 5 ships with a
  // navigator-based fallback. A future phase can add the plugin and enrich
  // this with Device.getInfo() on native.
  let os = 'unknown';
  try {
    const nav = navigator as Navigator & {
      userAgentData?: { platform?: string };
    };
    os = nav?.userAgentData?.platform || navigator.platform || 'unknown';
  } catch {
    // ignore
  }
  const webview =
    typeof navigator !== 'undefined' && navigator.userAgent
      ? navigator.userAgent
      : 'unknown';
  const model = getPlatform() === 'web' ? 'web' : 'unknown';
  return { model, os, webview };
}

// ---------- State reader ----------

async function getPendingEventsCount(): Promise<number> {
  try {
    const events = await getUnsyncedEvents();
    return events.length;
  } catch {
    return 0;
  }
}

function getAuthStateTag(): string {
  try {
    const s = get(authStore);
    return s.type;
  } catch {
    return 'unknown';
  }
}

function getNotifPermission(): string {
  try {
    if (typeof Notification !== 'undefined' && Notification.permission) {
      return Notification.permission;
    }
  } catch {
    // Notification API can throw on cross-origin iframes etc.
  }
  return 'unavailable';
}

function getGoalCount(): number {
  // Goals are held in App.svelte local state; no global store exists. A
  // future phase can hoist it if this becomes useful. Returning 0 keeps the
  // field typed without leaking data.
  return 0;
}

async function buildPayload(input: DebugReportInput): Promise<DebugReportPayload> {
  return {
    client_id: getClientId(),
    app_version: (import.meta as { env?: Record<string, string> }).env?.VITE_APP_VERSION || 'dev',
    platform: getPlatform(),
    device: getDevice(),
    state: {
      route: currentRoute,
      online: get(isOnline),
      pending_events: await getPendingEventsCount(),
      goal_count: getGoalCount(),
      auth_state: getAuthStateTag(),
      notif_permission: getNotifPermission(),
    },
    description: (input.description ?? '').trim(),
    breadcrumbs: snapshot(),
    trigger: input.trigger,
    client_ts: Date.now(),
  };
}

// ---------- Rate limit ----------

export function isClientRateLimited(now: number = Date.now()): boolean {
  try {
    const raw = localStorage.getItem(LAST_TS_STORAGE_KEY);
    if (!raw) return false;
    const last = Number.parseInt(raw, 10);
    if (!Number.isFinite(last)) return false;
    return now - last < CLIENT_RATE_LIMIT_MS;
  } catch {
    return false;
  }
}

function recordSendTimestamp(now: number): void {
  try {
    localStorage.setItem(LAST_TS_STORAGE_KEY, String(now));
  } catch {
    // ignore
  }
}

// ---------- Transport ----------

async function postPayload(payload: DebugReportPayload): Promise<Response> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = await getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  return fetch(`${getApiBase()}/debug-reports`, {
    method: 'POST',
    credentials: 'include',
    headers,
    body: JSON.stringify(payload),
  });
}

async function safeReadRateLimitMessage(res: Response): Promise<string | undefined> {
  try {
    const data = await res.clone().json();
    if (data && typeof data === 'object') {
      const candidate =
        (data as Record<string, unknown>).error ??
        (data as Record<string, unknown>).message;
      if (typeof candidate === 'string' && candidate.length > 0) return candidate;
    }
  } catch {
    // not JSON; try text
  }
  try {
    const text = await res.clone().text();
    if (text) return text;
  } catch {
    // ignore
  }
  return undefined;
}

// ---------- Queue ----------

async function pushToQueue(payload: DebugReportPayload): Promise<void> {
  const db = await openQueueDB();
  if (!db) return;
  try {
    const tx = db.transaction(QUEUE_STORE, 'readwrite');
    const store = tx.objectStore(QUEUE_STORE);
    const count = await store.count();
    if (count >= QUEUE_MAX) {
      // Drop oldest (lowest auto-increment id) to make room.
      const cursor = await store.openCursor();
      if (cursor) {
        await cursor.delete();
      }
    }
    await store.add({ payload } as unknown as { id: number; payload: DebugReportPayload });
    await tx.done;
  } catch {
    // ignore — queue is best effort
  } finally {
    db.close();
  }
}

/**
 * Drain all queued reports in FIFO order. Stops on the first failure so
 * transient outages don't burn through the whole queue against an unhealthy
 * backend. The next `drainQueue()` call will pick up where this one stopped.
 *
 * A module-level in-flight guard prevents concurrent drains from posting the
 * same entry twice (e.g. if an online flip fires while an auto-triggered
 * report is already draining).
 */
let isDraining = false;

export async function drainQueue(): Promise<void> {
  if (isDraining) return;
  isDraining = true;
  try {
    const db = await openQueueDB();
    if (!db) return;
    try {
      // Read all keys in natural (ascending) order — FIFO.
      const keys = (await db.getAllKeys(QUEUE_STORE)) as number[];
      for (const key of keys) {
        const entry = await db.get(QUEUE_STORE, key);
        if (!entry) continue;
        let ok = false;
        try {
          const res = await postPayload(entry.payload);
          ok = res.ok;
        } catch {
          ok = false;
        }
        if (!ok) return; // stop; try again next time
        try {
          await db.delete(QUEUE_STORE, key);
        } catch {
          // if we can't delete, bail to avoid infinite retry of same entry
          return;
        }
      }
    } finally {
      db.close();
    }
  } finally {
    isDraining = false;
  }
}

// ---------- Public send ----------

export async function sendDebugReport(
  input: DebugReportInput,
): Promise<DebugReportOutcome> {
  const now = Date.now();
  if (isClientRateLimited(now)) {
    return {
      outcome: 'client_rate_limited',
      message: 'recent report just sent',
    };
  }

  const payload = await buildPayload(input);

  // Transport
  let res: Response | undefined;
  try {
    res = await postPayload(payload);
  } catch {
    // Network error — queue for later.
    await pushToQueue(payload);
    recordSendTimestamp(now);
    return { outcome: 'queued' };
  }

  if (res.ok) {
    recordSendTimestamp(now);
    try {
      clear();
    } catch {
      // ignore
    }
    return { outcome: 'sent' };
  }

  if (res.status === 429) {
    const message = await safeReadRateLimitMessage(res);
    return {
      outcome: 'rate_limited',
      ...(message ? { message } : {}),
    };
  }

  // Other 4xx/5xx: treat as retryable to avoid dropping user-reported
  // issues over a transient backend bug.
  await pushToQueue(payload);
  recordSendTimestamp(now);
  return { outcome: 'queued', message: `HTTP ${res.status}` };
}

// ---------- Test helpers ----------

export function __resetForTest(): void {
  currentRoute = 'home';
  isDraining = false;
  try {
    localStorage.removeItem(CLIENT_ID_STORAGE_KEY);
    localStorage.removeItem(LAST_TS_STORAGE_KEY);
  } catch {
    // ignore
  }
}
