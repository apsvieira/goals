import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock @capacitor/preferences with an in-memory store backing Preferences.get/set.
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
  return await import('../notification-settings');
}

describe('notification-settings', () => {
  beforeEach(() => {
    prefStore.clear();
    vi.clearAllMocks();
  });

  it('loadNotificationSettings returns defaults on first read', async () => {
    const mod = await freshImport();
    const loaded = await mod.loadNotificationSettings();
    expect(loaded.frequency).toBe('off');
    expect(loaded.time).toBe('20:00');
    expect(loaded.weekday).toBe(0);
    expect(loaded.permissionDeniedAt).toBeUndefined();
  });

  it('materializes defaults into storage on first read', async () => {
    const mod = await freshImport();
    await mod.loadNotificationSettings();
    // Now the store should contain the serialized defaults.
    expect(prefStore.has('notification_settings')).toBe(true);
    const raw = prefStore.get('notification_settings')!;
    const parsed = JSON.parse(raw);
    expect(parsed.frequency).toBe('off');
    expect(parsed.time).toBe('20:00');
    expect(parsed.weekday).toBe(0);
  });

  it('saveNotificationSettings + loadNotificationSettings round-trips', async () => {
    const mod = await freshImport();
    const next: import('../notification-settings').NotificationSettings = {
      frequency: 'daily',
      time: '07:30',
      weekday: 3,
      permissionDeniedAt: '2026-04-09T12:00:00Z',
    };
    await mod.saveNotificationSettings(next);

    const loaded = await mod.loadNotificationSettings();
    expect(loaded).toEqual(next);
  });

  it('preserves unknown stored fields on load', async () => {
    prefStore.set(
      'notification_settings',
      JSON.stringify({
        frequency: 'weekly',
        time: '08:00',
        weekday: 2,
        smartSkip: true,
        futureField: 'xyz',
      }),
    );
    const mod = await freshImport();
    const loaded = (await mod.loadNotificationSettings()) as unknown as Record<string, unknown>;
    expect(loaded.frequency).toBe('weekly');
    expect(loaded.time).toBe('08:00');
    expect(loaded.weekday).toBe(2);
    expect(loaded.smartSkip).toBe(true);
    expect(loaded.futureField).toBe('xyz');
  });

  it('fills missing fields from defaults when stored JSON is partial', async () => {
    prefStore.set('notification_settings', JSON.stringify({ frequency: 'daily' }));
    const mod = await freshImport();
    const loaded = await mod.loadNotificationSettings();
    expect(loaded.frequency).toBe('daily');
    expect(loaded.time).toBe('20:00');
    expect(loaded.weekday).toBe(0);
  });

  it('updateNotificationSettings patches and persists', async () => {
    const mod = await freshImport();
    await mod.loadNotificationSettings();
    const updated = await mod.updateNotificationSettings({
      frequency: 'weekly',
      weekday: 5,
    });
    expect(updated.frequency).toBe('weekly');
    expect(updated.weekday).toBe(5);
    expect(updated.time).toBe('20:00');

    const reloaded = await mod.loadNotificationSettings();
    expect(reloaded.frequency).toBe('weekly');
    expect(reloaded.weekday).toBe(5);
  });

  it('updateNotificationSettings updates the svelte store', async () => {
    const mod = await freshImport();
    await mod.loadNotificationSettings();

    const seen: import('../notification-settings').NotificationSettings[] = [];
    const unsubscribe = mod.notificationSettings.subscribe((s) => seen.push({ ...s }));

    await mod.updateNotificationSettings({ frequency: 'daily', time: '09:15' });
    unsubscribe();

    const last = seen[seen.length - 1];
    expect(last.frequency).toBe('daily');
    expect(last.time).toBe('09:15');
  });
});
