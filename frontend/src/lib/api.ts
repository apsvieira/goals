import { get } from 'svelte/store';
import { authStore, type User } from './stores';
import {
  initStorage,
  getLocalGoals,
  saveLocalGoal,
  deleteLocalGoal,
  getLocalCompletions,
  saveLocalCompletion,
  deleteLocalCompletion,
  deleteLocalCompletionByGoalAndDate,
  getLocalCompletionByGoalAndDate,
  getMaxPosition,
  getAllLocalCompletions,
  saveSyncEvent,
} from './storage';
import type { SyncEvent } from './events';
import { sendEvent, flushPendingEvents } from './event-sync';
import { getToken } from './token-storage';
import { getApiBase } from './config';

const API_BASE = getApiBase();

export interface Goal {
  id: string;
  name: string;
  color: string;
  position: number;
  target_count?: number;
  target_period?: 'week' | 'month';
  created_at: string;
  archived_at?: string;
}

export interface Completion {
  id: string;
  goal_id: string;
  date: string;
  created_at: string;
}

export interface CalendarResponse {
  goals: Goal[];
  completions: Completion[];
}

// Initialize storage
let storageInitialized = false;
let storageInitError: Error | null = null;

async function ensureStorageInitialized(): Promise<void> {
  if (!storageInitialized) {
    try {
      await initStorage();
      storageInitialized = true;
      storageInitError = null;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      storageInitError = error instanceof Error ? error : new Error(errorMessage);
      console.error('Failed to initialize storage:', errorMessage);
      console.error('Storage operations will not work. Error details:', error);

      // Allow retry on next call by not setting storageInitialized = true
      throw new Error(`Storage initialization failed: ${errorMessage}. The app may not function correctly.`);
    }
  }

  // If previous initialization failed, throw the cached error
  if (storageInitError) {
    throw storageInitError;
  }
}

// Generate a unique ID for local storage
export function generateId(): string {
  return crypto.randomUUID();
}

// Generate a deterministic completion ID from goal ID and date
export function generateCompletionId(goalId: string, date: string): string {
  return `${goalId}:${date}`;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  // Build headers with optional Authorization token for mobile
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options?.headers as Record<string, string>,
  };

  // Add Authorization header if we have a token (mobile auth)
  const token = await getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',
    headers,
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

export async function getCalendar(month: string): Promise<CalendarResponse> {
  await ensureStorageInitialized();
  try {
    const data = await request<CalendarResponse>(`/calendar?month=${month}`);

    // Persist server data to IndexedDB so local mutations (archive, update) can find goals
    for (const goal of data.goals ?? []) {
      await saveLocalGoal(goal);
    }
    for (const completion of data.completions ?? []) {
      await saveLocalCompletion(completion);
    }

    return data;
  } catch (e) {
    const goals = await getLocalGoals();
    const completions = await getLocalCompletions(month);
    return { goals, completions };
  }
}

export async function createGoal(
  name: string,
  color: string,
  targetCount?: number,
  targetPeriod?: 'week' | 'month'
): Promise<Goal> {
  await ensureStorageInitialized();

  // Generate ID for new goal
  const maxPosition = await getMaxPosition();
  const goal: Goal = {
    id: generateId(),
    name,
    color,
    position: maxPosition + 1,
    target_count: targetCount,
    target_period: targetPeriod,
    created_at: new Date().toISOString(),
  };

  // Save to local cache immediately
  await saveLocalGoal(goal);

  // Create and persist sync event, then fire-and-forget to server
  const event: SyncEvent = {
    id: generateId(),
    type: 'goal_upsert',
    timestamp: new Date().toISOString(),
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
  await saveSyncEvent(event);
  sendEvent(event).catch(console.error);

  return goal;
}

export async function updateGoal(
  id: string,
  updates: { name?: string; color?: string; target_count?: number; target_period?: 'week' | 'month' }
): Promise<Goal> {
  await ensureStorageInitialized();

  const goals = await getLocalGoals();
  const existingGoal = goals.find(g => g.id === id);
  if (!existingGoal) {
    throw new Error('Goal not found');
  }

  const updatedGoal: Goal = { ...existingGoal, ...updates };
  await saveLocalGoal(updatedGoal);

  // Create and persist sync event with FULL goal state (not just the delta)
  const event: SyncEvent = {
    id: generateId(),
    type: 'goal_upsert',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: {
      id: updatedGoal.id,
      name: updatedGoal.name,
      color: updatedGoal.color,
      position: updatedGoal.position,
      target_count: updatedGoal.target_count,
      target_period: updatedGoal.target_period,
    },
  };
  await saveSyncEvent(event);
  sendEvent(event).catch(console.error);

  return updatedGoal;
}

export async function archiveGoal(id: string): Promise<void> {
  await ensureStorageInitialized();

  const goals = await getLocalGoals();
  const existingGoal = goals.find(g => g.id === id);
  if (!existingGoal) {
    throw new Error('Goal not found');
  }

  const archivedGoal: Goal = {
    ...existingGoal,
    archived_at: new Date().toISOString(),
  };
  await saveLocalGoal(archivedGoal);

  // Create and persist sync event for goal deletion
  const event: SyncEvent = {
    id: generateId(),
    type: 'goal_delete',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: { id },
  };
  await saveSyncEvent(event);
  sendEvent(event).catch(console.error);
}

export async function createCompletion(goalId: string, date: string): Promise<Completion> {
  await ensureStorageInitialized();

  const completion: Completion = {
    id: generateCompletionId(goalId, date),
    goal_id: goalId,
    date,
    created_at: new Date().toISOString(),
  };
  await saveLocalCompletion(completion);

  // Create and persist sync event for completion
  const event: SyncEvent = {
    id: generateId(),
    type: 'completion_set',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: { goal_id: goalId, date },
  };
  await saveSyncEvent(event);
  sendEvent(event).catch(console.error);

  return completion;
}

export async function deleteCompletion(id: string, goalId: string, date: string): Promise<void> {
  await ensureStorageInitialized();

  // Delete by ID (covers local-created completions)
  await deleteLocalCompletion(id);

  // Also delete by goal_id + date (covers sync-created and server-ID mismatches)
  await deleteLocalCompletionByGoalAndDate(goalId, date);

  // Create and persist sync event for completion removal
  const event: SyncEvent = {
    id: generateId(),
    type: 'completion_unset',
    timestamp: new Date().toISOString(),
    synced: false,
    payload: { goal_id: goalId, date },
  };
  await saveSyncEvent(event);
  sendEvent(event).catch(console.error);
}

export async function reorderGoals(goalIds: string[]): Promise<Goal[]> {
  await ensureStorageInitialized();

  const goals = await getLocalGoals();
  const updatedGoals: Goal[] = [];

  for (let i = 0; i < goalIds.length; i++) {
    const goal = goals.find(g => g.id === goalIds[i]);
    if (goal) {
      const updatedGoal = { ...goal, position: i + 1 };
      await saveLocalGoal(updatedGoal);
      updatedGoals.push(updatedGoal);
    }
  }

  // Save all goal_upsert events first, then batch-flush once
  for (const goal of updatedGoals) {
    const event: SyncEvent = {
      id: generateId(),
      type: 'goal_upsert',
      timestamp: new Date().toISOString(),
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
    await saveSyncEvent(event);
  }
  flushPendingEvents().catch(console.error);

  return updatedGoals.sort((a, b) => a.position - b.position);
}

// Helper function to find and delete completion by goal and date (for toggle functionality)
export async function findCompletionByGoalAndDate(goalId: string, date: string): Promise<Completion | undefined> {
  await ensureStorageInitialized();
  return getLocalCompletionByGoalAndDate(goalId, date);
}

// Auth API

export async function getCurrentUser(): Promise<User | null> {
  try {
    // Build headers with optional Authorization token for mobile
    const headers: Record<string, string> = {};
    const token = await getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const res = await fetch(`${API_BASE}/auth/me`, {
      credentials: 'include',
      headers,
    });

    if (!res.ok) {
      return null;
    }

    return res.json();
  } catch {
    return null;
  }
}

export async function logout(): Promise<void> {
  // Build headers with optional Authorization token for mobile
  const headers: Record<string, string> = {};
  const token = await getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  await fetch(`${API_BASE}/auth/logout`, {
    method: 'POST',
    credentials: 'include',
    headers,
  });
}

// Get all completions (for statistics - no date filter)
export async function getAllCompletions(): Promise<Completion[]> {
  await ensureStorageInitialized();

  try {
    // Try to fetch from server first
    return await request<Completion[]>('/completions?from=2020-01-01&to=2099-12-31');
  } catch (e) {
    // Offline: use local cache
    return getAllLocalCompletions();
  }
}

// Get completions for the current period (week and month) for progress bar calculations
export async function getCurrentPeriodCompletions(): Promise<Completion[]> {
  await ensureStorageInitialized();

  const now = new Date();
  // Start of current week (Sunday)
  const weekStart = new Date(now);
  weekStart.setDate(now.getDate() - now.getDay());
  weekStart.setHours(0, 0, 0, 0);
  // Start of current month
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
  // Use earlier date to cover both weekly and monthly targets
  const from = (monthStart < weekStart ? monthStart : weekStart).toISOString().slice(0, 10);
  const to = now.toISOString().slice(0, 10);

  try {
    return await request<Completion[]>(`/completions?from=${from}&to=${to}`);
  } catch (e) {
    // Offline: use local cache
    return getLocalCompletionsForRange(from, to);
  }
}

// Helper to get local completions for a date range
async function getLocalCompletionsForRange(from: string, to: string): Promise<Completion[]> {
  const fromDate = new Date(from);
  const toDate = new Date(to);
  const allCompletions: Completion[] = [];

  // Fetch each month in the range
  const current = new Date(fromDate.getFullYear(), fromDate.getMonth(), 1);
  while (current <= toDate) {
    const monthStr = current.toISOString().slice(0, 7);
    const monthCompletions = await getLocalCompletions(monthStr);
    allCompletions.push(...monthCompletions);
    current.setMonth(current.getMonth() + 1);
  }

  // Filter to exact date range and deduplicate
  const seen = new Set<string>();
  return allCompletions.filter(c => {
    if (seen.has(c.id)) return false;
    if (c.date < from || c.date > to) return false;
    seen.add(c.id);
    return true;
  });
}

// Device registration API (for push notifications)

export interface Device {
  id: string;
  token: string;
  platform: string;
  created_at: string;
}

/**
 * Register a device token for push notifications
 * @param token - FCM/APNs token
 * @param platform - 'ios' or 'android'
 */
export async function registerDevice(token: string, platform: string): Promise<Device> {
  return request<Device>('/devices', {
    method: 'POST',
    body: JSON.stringify({ token, platform }),
  });
}

/**
 * Unregister a device from push notifications
 * @param id - Device ID returned from registerDevice
 */
export async function unregisterDevice(id: string): Promise<void> {
  return request<void>(`/devices/${id}`, {
    method: 'DELETE',
  });
}
