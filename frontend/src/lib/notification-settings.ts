import { writable } from 'svelte/store';
import { Preferences } from '@capacitor/preferences';

export type NotificationFrequency = 'off' | 'daily' | 'weekly';

export interface NotificationSettings {
  frequency: NotificationFrequency;
  time: string;
  weekday: number;
  permissionDeniedAt?: string;
}

export const DEFAULT_NOTIFICATION_SETTINGS: NotificationSettings = {
  frequency: 'daily',
  time: '19:00',
  weekday: 0,
};

const STORAGE_KEY = 'notification_settings';

export async function loadNotificationSettings(): Promise<NotificationSettings> {
  try {
    const { value } = await Preferences.get({ key: STORAGE_KEY });
    if (!value) {
      // Materialize defaults on first read
      await Preferences.set({
        key: STORAGE_KEY,
        value: JSON.stringify(DEFAULT_NOTIFICATION_SETTINGS),
      });
      return { ...DEFAULT_NOTIFICATION_SETTINGS };
    }
    const parsed = JSON.parse(value) as Partial<NotificationSettings> & Record<string, unknown>;
    // Merge onto defaults, preserving unknown fields for forward-compat
    return { ...DEFAULT_NOTIFICATION_SETTINGS, ...parsed };
  } catch (err) {
    console.error('[Notifications] Failed to load settings:', err);
    return { ...DEFAULT_NOTIFICATION_SETTINGS };
  }
}

export async function saveNotificationSettings(settings: NotificationSettings): Promise<void> {
  await Preferences.set({
    key: STORAGE_KEY,
    value: JSON.stringify(settings),
  });
}

export const notificationSettings = writable<NotificationSettings>({ ...DEFAULT_NOTIFICATION_SETTINGS });

// Hydrate the store on module load
let hydrated = false;
export async function hydrateNotificationSettings(): Promise<NotificationSettings> {
  const loaded = await loadNotificationSettings();
  notificationSettings.set(loaded);
  hydrated = true;
  return loaded;
}

// Fire-and-forget hydration so web and native UIs see the persisted value ASAP.
// Skipped under Vitest (`MODE === 'test'`) to avoid implicit side effects on
// module import in tests; test files invoke `hydrateNotificationSettings()`
// explicitly when they need it.
if (import.meta.env.MODE !== 'test') {
  void hydrateNotificationSettings();
}

const PROMPT_SEEN_KEY = 'notification_prompt_seen';

export async function isNotificationPromptSeen(): Promise<boolean> {
  const { value } = await Preferences.get({ key: PROMPT_SEEN_KEY });
  return value === 'true';
}

export async function markNotificationPromptSeen(): Promise<void> {
  await Preferences.set({ key: PROMPT_SEEN_KEY, value: 'true' });
}

export async function updateNotificationSettings(
  patch: Partial<NotificationSettings>,
): Promise<NotificationSettings> {
  const current = hydrated ? await loadNotificationSettings() : await hydrateNotificationSettings();
  const next: NotificationSettings = { ...current, ...patch };
  await saveNotificationSettings(next);
  notificationSettings.set(next);
  return next;
}
