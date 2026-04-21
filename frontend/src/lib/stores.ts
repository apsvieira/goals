import { writable } from 'svelte/store';

export interface User {
  id: string;
  email: string;
  name?: string;
  avatar_url?: string;
  created_at: string;
}

export type AuthState =
  | { type: 'loading' }
  | { type: 'unauthenticated' }
  | { type: 'authenticated'; user: User };

export const authStore = writable<AuthState>({ type: 'loading' });
export const isOnline = writable<boolean>(typeof navigator !== 'undefined' ? navigator.onLine : true);

// Listen for online/offline events
if (typeof window !== 'undefined') {
  window.addEventListener('online', () => isOnline.set(true));
  window.addEventListener('offline', () => isOnline.set(false));
}

// Debug-report modal visibility. Phase 5 exposes only the user-initiated
// entry point; Phase 6 (shake) and Phase 7 (Sentry) will call
// `openDebugReport()` from their own triggers.
export const debugReportModalOpen = writable<boolean>(false);

export function openDebugReport(): void {
  debugReportModalOpen.set(true);
}

export function closeDebugReport(): void {
  debugReportModalOpen.set(false);
}
