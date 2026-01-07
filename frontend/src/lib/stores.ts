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
