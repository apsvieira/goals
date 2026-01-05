const API_BASE = 'http://localhost:8080/api/v1';

export interface Goal {
  id: string;
  name: string;
  color: string;
  position: number;
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

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
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
  return request<CalendarResponse>(`/calendar?month=${month}`);
}

export async function createGoal(name: string, color: string): Promise<Goal> {
  return request<Goal>('/goals', {
    method: 'POST',
    body: JSON.stringify({ name, color }),
  });
}

export async function updateGoal(id: string, updates: { name?: string; color?: string }): Promise<Goal> {
  return request<Goal>(`/goals/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(updates),
  });
}

export async function archiveGoal(id: string): Promise<void> {
  return request<void>(`/goals/${id}`, {
    method: 'DELETE',
  });
}

export async function createCompletion(goalId: string, date: string): Promise<Completion> {
  return request<Completion>('/completions', {
    method: 'POST',
    body: JSON.stringify({ goal_id: goalId, date }),
  });
}

export async function deleteCompletion(id: string): Promise<void> {
  return request<void>(`/completions/${id}`, {
    method: 'DELETE',
  });
}

export async function reorderGoals(goalIds: string[]): Promise<Goal[]> {
  return request<Goal[]>('/goals/reorder', {
    method: 'PUT',
    body: JSON.stringify({ goal_ids: goalIds }),
  });
}
