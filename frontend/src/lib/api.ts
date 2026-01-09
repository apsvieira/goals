import { get } from 'svelte/store';
import { Capacitor } from '@capacitor/core';
import { authStore, type User } from './stores';
import {
  initStorage,
  getLocalGoals,
  saveLocalGoal,
  deleteLocalGoal,
  getLocalCompletions,
  saveLocalCompletion,
  deleteLocalCompletion,
  getLocalCompletionByGoalAndDate,
  getMaxPosition,
  getAllLocalCompletions,
  type QueuedOperation,
  saveQueuedOperation,
} from './storage';
import { getToken } from './token-storage';

const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

// Use production URL for native platforms, relative URL in web production, absolute in dev
function getApiBase(): string {
  if (Capacitor.isNativePlatform()) {
    return `${PRODUCTION_API_URL}/api/v1`;
  }
  return typeof window !== 'undefined' && window.location.hostname !== 'localhost'
    ? '/api/v1'
    : 'http://localhost:8080/api/v1';
}

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
function generateId(): string {
  return `local-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
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
    return await request<CalendarResponse>(`/calendar?month=${month}`);
  } catch (e) {
    // Offline: return cached data
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

  // Queue operation for sync
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'create_goal',
    entityId: goal.id,
    payload: { name, color, target_count: targetCount, target_period: targetPeriod },
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);

  // Attempt immediate sync (will be handled by sync manager)
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

  // Queue operation
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'update_goal',
    entityId: id,
    payload: updates,
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);

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

  // Queue operation
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'delete_goal',
    entityId: id,
    payload: {},
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);
}

export async function createCompletion(goalId: string, date: string): Promise<Completion> {
  await ensureStorageInitialized();

  const completion: Completion = {
    id: generateId(),
    goal_id: goalId,
    date,
    created_at: new Date().toISOString(),
  };
  await saveLocalCompletion(completion);

  // Queue operation
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'create_completion',
    entityId: completion.id,
    payload: { goal_id: goalId, date },
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);

  return completion;
}

export async function deleteCompletion(id: string): Promise<void> {
  await ensureStorageInitialized();

  // Get completion details before deleting (need for queue payload)
  const allCompletions = await getAllLocalCompletions();
  const completion = allCompletions.find(c => c.id === id);

  await deleteLocalCompletion(id);

  if (completion) {
    // Queue operation
    const operation: QueuedOperation = {
      id: generateId(),
      type: 'delete_completion',
      entityId: id,
      payload: { goal_id: completion.goal_id, date: completion.date },
      timestamp: new Date().toISOString(),
      retryCount: 0,
    };
    await saveQueuedOperation(operation);
  }
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

  // Queue operation
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'reorder_goals',
    entityId: 'reorder',
    payload: { goal_ids: goalIds },
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);

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
