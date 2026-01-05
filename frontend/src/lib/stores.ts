import { writable } from 'svelte/store';

export interface User {
  id: string;
  email: string;
  name?: string;
}

export type AuthState =
  | { type: 'guest' }
  | { type: 'authenticated'; user: User };

export const authStore = writable<AuthState>({ type: 'guest' });
export const isOnline = writable<boolean>(typeof navigator !== 'undefined' ? navigator.onLine : true);

// Listen for online/offline events
if (typeof window !== 'undefined') {
  window.addEventListener('online', () => isOnline.set(true));
  window.addEventListener('offline', () => isOnline.set(false));
}
