import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock @capacitor/preferences with an in-memory store
const prefStore = vi.hoisted(() => new Map<string, string>());
vi.mock('@capacitor/preferences', () => ({
  Preferences: {
    get: vi.fn(async ({ key }: { key: string }) => ({
      value: prefStore.has(key) ? prefStore.get(key)! : null,
    })),
    set: vi.fn(async ({ key, value }: { key: string; value: string }) => {
      prefStore.set(key, value);
    }),
    remove: vi.fn(async ({ key }: { key: string }) => {
      prefStore.delete(key);
    }),
    clear: vi.fn(async () => {
      prefStore.clear();
    }),
  },
}));

async function freshImport() {
  vi.resetModules();
  return await import('../view-settings');
}

describe('view-settings', () => {
  beforeEach(() => {
    prefStore.clear();
    vi.clearAllMocks();
  });

  it('loadViewSettings returns defaults on first read', async () => {
    const mod = await freshImport();
    const loaded = await mod.loadViewSettings();
    expect(loaded.calendarView).toBe('month');
  });

  it('materializes defaults into storage on first read', async () => {
    const mod = await freshImport();
    await mod.loadViewSettings();
    expect(prefStore.has('view_settings')).toBe(true);
    const raw = prefStore.get('view_settings')!;
    const parsed = JSON.parse(raw);
    expect(parsed.calendarView).toBe('month');
  });

  it('saveViewSettings + loadViewSettings round-trips', async () => {
    const mod = await freshImport();
    const next: import('../view-settings').ViewSettings = {
      calendarView: 'week',
    };
    await mod.saveViewSettings(next);
    const loaded = await mod.loadViewSettings();
    expect(loaded).toEqual(next);
  });

  it('preserves unknown stored fields on load', async () => {
    prefStore.set(
      'view_settings',
      JSON.stringify({
        calendarView: 'week',
        futureFeature: 'xyz',
      }),
    );
    const mod = await freshImport();
    const loaded = (await mod.loadViewSettings()) as unknown as Record<string, unknown>;
    expect(loaded.calendarView).toBe('week');
    expect(loaded.futureFeature).toBe('xyz');
  });

  it('fills missing fields from defaults when stored JSON is partial', async () => {
    prefStore.set('view_settings', JSON.stringify({}));
    const mod = await freshImport();
    const loaded = await mod.loadViewSettings();
    expect(loaded.calendarView).toBe('month');
  });

  it('updateViewSettings patches and persists', async () => {
    const mod = await freshImport();
    await mod.loadViewSettings();
    const updated = await mod.updateViewSettings({ calendarView: 'week' });
    expect(updated.calendarView).toBe('week');

    const reloaded = await mod.loadViewSettings();
    expect(reloaded.calendarView).toBe('week');
  });

  it('updateViewSettings updates the svelte store', async () => {
    const mod = await freshImport();
    await mod.loadViewSettings();

    const seen: import('../view-settings').ViewSettings[] = [];
    const unsubscribe = mod.viewSettings.subscribe((s) => seen.push({ ...s }));

    await mod.updateViewSettings({ calendarView: 'week' });
    unsubscribe();

    const last = seen[seen.length - 1];
    expect(last.calendarView).toBe('week');
  });

  it('updateViewSettings round-trips back to month', async () => {
    const mod = await freshImport();
    await mod.updateViewSettings({ calendarView: 'week' });
    const updated = await mod.updateViewSettings({ calendarView: 'month' });
    expect(updated.calendarView).toBe('month');

    const reloaded = await mod.loadViewSettings();
    expect(reloaded.calendarView).toBe('month');
  });

  it('DEFAULT_VIEW_SETTINGS has calendarView month', async () => {
    const mod = await freshImport();
    expect(mod.DEFAULT_VIEW_SETTINGS.calendarView).toBe('month');
  });
});
