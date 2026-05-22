import { writable } from 'svelte/store';
import { Preferences } from '@capacitor/preferences';

export type CalendarView = 'month' | 'week';

export interface ViewSettings {
  calendarView: CalendarView;
}

export const DEFAULT_VIEW_SETTINGS: ViewSettings = {
  calendarView: 'month',
};

const STORAGE_KEY = 'view_settings';

export async function loadViewSettings(): Promise<ViewSettings> {
  try {
    const { value } = await Preferences.get({ key: STORAGE_KEY });
    if (!value) {
      await Preferences.set({
        key: STORAGE_KEY,
        value: JSON.stringify(DEFAULT_VIEW_SETTINGS),
      });
      return { ...DEFAULT_VIEW_SETTINGS };
    }
    const parsed = JSON.parse(value) as Partial<ViewSettings> & Record<string, unknown>;
    return { ...DEFAULT_VIEW_SETTINGS, ...parsed };
  } catch (err) {
    console.error('[ViewSettings] Failed to load settings:', err);
    return { ...DEFAULT_VIEW_SETTINGS };
  }
}

export async function saveViewSettings(settings: ViewSettings): Promise<void> {
  await Preferences.set({
    key: STORAGE_KEY,
    value: JSON.stringify(settings),
  });
}

export const viewSettings = writable<ViewSettings>({ ...DEFAULT_VIEW_SETTINGS });

// Hydrate the store on module load
let hydrated = false;
export async function hydrateViewSettings(): Promise<ViewSettings> {
  const loaded = await loadViewSettings();
  viewSettings.set(loaded);
  hydrated = true;
  return loaded;
}

// Fire-and-forget hydration so web and native UIs see the persisted value ASAP.
// Skipped under Vitest (`MODE === 'test'`) to avoid implicit side effects on
// module import in tests; test files invoke `hydrateViewSettings()`
// explicitly when they need it.
if (import.meta.env.MODE !== 'test') {
  void hydrateViewSettings();
}

export async function updateViewSettings(
  patch: Partial<ViewSettings>,
): Promise<ViewSettings> {
  const current = hydrated ? await loadViewSettings() : await hydrateViewSettings();
  const next: ViewSettings = { ...current, ...patch };
  await saveViewSettings(next);
  viewSettings.set(next);
  return next;
}
