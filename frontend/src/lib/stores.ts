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
  | { type: 'guest' }
  | { type: 'authenticated'; user: User };

export const authStore = writable<AuthState>({ type: 'loading' });
export const isOnline = writable<boolean>(typeof navigator !== 'undefined' ? navigator.onLine : true);

// Listen for online/offline events
if (typeof window !== 'undefined') {
  window.addEventListener('online', () => isOnline.set(true));
  window.addEventListener('offline', () => isOnline.set(false));
}

// Local storage key for guest mode
const GUEST_MODE_KEY = 'goal-tracker-guest-mode';

// Check if user has opted into guest mode previously
export function hasLocalData(): boolean {
  if (typeof window === 'undefined') return false;
  // Check if there's any data in IndexedDB by checking the guest mode flag
  return localStorage.getItem(GUEST_MODE_KEY) === 'true';
}

// Set guest mode in local storage
export function setGuestMode(enabled: boolean): void {
  if (typeof window === 'undefined') return;
  if (enabled) {
    localStorage.setItem(GUEST_MODE_KEY, 'true');
  } else {
    localStorage.removeItem(GUEST_MODE_KEY);
  }
}

// Check if user is in guest mode
export function isInGuestMode(): boolean {
  return hasLocalData();
}
