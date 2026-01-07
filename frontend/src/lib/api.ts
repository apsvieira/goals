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

// Check if we're in guest mode
export function isGuestMode(): boolean {
  const auth = get(authStore);
  return auth.type === 'guest';
}

// Initialize storage for guest mode
let storageInitialized = false;
async function ensureStorageInitialized(): Promise<void> {
  if (!storageInitialized) {
    await initStorage();
    storageInitialized = true;
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
  if (isGuestMode()) {
    await ensureStorageInitialized();
    const goals = await getLocalGoals();
    const completions = await getLocalCompletions(month);
    return { goals, completions };
  }
  return request<CalendarResponse>(`/calendar?month=${month}`);
}

export async function createGoal(
  name: string,
  color: string,
  targetCount?: number,
  targetPeriod?: 'week' | 'month'
): Promise<Goal> {
  if (isGuestMode()) {
    await ensureStorageInitialized();
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
    await saveLocalGoal(goal);
    return goal;
  }
  return request<Goal>('/goals', {
    method: 'POST',
    body: JSON.stringify({ name, color, target_count: targetCount, target_period: targetPeriod }),
  });
}

export async function updateGoal(
  id: string,
  updates: { name?: string; color?: string; target_count?: number; target_period?: 'week' | 'month' }
): Promise<Goal> {
  if (isGuestMode()) {
    await ensureStorageInitialized();
    const goals = await getLocalGoals();
    const existingGoal = goals.find(g => g.id === id);
    if (!existingGoal) {
      throw new Error('Goal not found');
    }
    const updatedGoal: Goal = {
      ...existingGoal,
      ...updates,
    };
    await saveLocalGoal(updatedGoal);
    return updatedGoal;
  }
  return request<Goal>(`/goals/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(updates),
  });
}

export async function archiveGoal(id: string): Promise<void> {
  if (isGuestMode()) {
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
    return;
  }
  return request<void>(`/goals/${id}`, {
    method: 'DELETE',
  });
}

export async function createCompletion(goalId: string, date: string): Promise<Completion> {
  if (isGuestMode()) {
    await ensureStorageInitialized();
    const completion: Completion = {
      id: generateId(),
      goal_id: goalId,
      date,
      created_at: new Date().toISOString(),
    };
    await saveLocalCompletion(completion);
    return completion;
  }
  return request<Completion>('/completions', {
    method: 'POST',
    body: JSON.stringify({ goal_id: goalId, date }),
  });
}

export async function deleteCompletion(id: string): Promise<void> {
  if (isGuestMode()) {
    await ensureStorageInitialized();
    await deleteLocalCompletion(id);
    return;
  }
  return request<void>(`/completions/${id}`, {
    method: 'DELETE',
  });
}

export async function reorderGoals(goalIds: string[]): Promise<Goal[]> {
  if (isGuestMode()) {
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

    return updatedGoals.sort((a, b) => a.position - b.position);
  }
  return request<Goal[]>('/goals/reorder', {
    method: 'PUT',
    body: JSON.stringify({ goal_ids: goalIds }),
  });
}

// Helper function to find and delete completion by goal and date (for toggle functionality)
export async function findCompletionByGoalAndDate(goalId: string, date: string): Promise<Completion | undefined> {
  if (isGuestMode()) {
    await ensureStorageInitialized();
    return getLocalCompletionByGoalAndDate(goalId, date);
  }
  // For server mode, this would need to be handled differently
  // The App.svelte currently tracks completions in memory
  return undefined;
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
  if (isGuestMode()) {
    await ensureStorageInitialized();
    // Get all completions by passing a very wide date range
    const goals = await getLocalGoals();
    const allCompletions: Completion[] = [];
    // Get completions for each month from 2020 to 2030
    for (let year = 2020; year <= 2030; year++) {
      for (let month = 1; month <= 12; month++) {
        const monthStr = `${year}-${month.toString().padStart(2, '0')}`;
        const monthCompletions = await getLocalCompletions(monthStr);
        allCompletions.push(...monthCompletions);
      }
    }
    // Deduplicate by id
    const seen = new Set<string>();
    return allCompletions.filter(c => {
      if (seen.has(c.id)) return false;
      seen.add(c.id);
      return true;
    });
  }
  // For server mode, fetch all completions without date filter
  // Using a very wide date range
  return request<Completion[]>('/completions?from=2020-01-01&to=2099-12-31');
}

// Get completions for the current period (week and month) for progress bar calculations
export async function getCurrentPeriodCompletions(): Promise<Completion[]> {
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

  if (isGuestMode()) {
    await ensureStorageInitialized();
    return getLocalCompletionsForRange(from, to);
  }
  return request<Completion[]>(`/completions?from=${from}&to=${to}`);
}

// Helper to get local completions for a date range (guest mode)
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
