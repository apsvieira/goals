import { describe, it, expect, beforeEach, vi } from 'vitest';

// In-memory preferences backing store
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

// Capacitor.isNativePlatform flag controlled per-test
const capState = vi.hoisted(() => ({ isNative: true }));
vi.mock('@capacitor/core', () => ({
  Capacitor: {
    isNativePlatform: () => capState.isNative,
    getPlatform: () => (capState.isNative ? 'android' : 'web'),
  },
}));

// LocalNotifications plugin mock with spyable methods
const notifState = vi.hoisted(() => ({
  permission: 'granted' as 'granted' | 'denied' | 'prompt',
  requestPermission: 'granted' as 'granted' | 'denied' | 'prompt',
}));
const localNotificationsMock = vi.hoisted(() => ({
  schedule: vi.fn(async () => ({ notifications: [] })),
  cancel: vi.fn(async () => {}),
  checkPermissions: vi.fn(async () => ({ display: 'granted' as 'granted' | 'denied' | 'prompt' })),
  requestPermissions: vi.fn(async () => ({ display: 'granted' as 'granted' | 'denied' | 'prompt' })),
  registerActionTypes: vi.fn(async () => {}),
  addListener: vi.fn(async () => ({ remove: async () => {} })),
}));
vi.mock('@capacitor/local-notifications', () => ({
  LocalNotifications: localNotificationsMock,
}));

// svelte-i18n: make $_ a passthrough returning the translation key
vi.mock('svelte-i18n', () => {
  const _store = {
    subscribe: (fn: (t: (k: string) => string) => void) => {
      fn((k: string) => k);
      return () => {};
    },
  };
  const localeStore = {
    subscribe: (fn: (v: string) => void) => {
      fn('en');
      return () => {};
    },
  };
  return { _: _store, locale: localeStore };
});

// Mock storage.saveReminderEvent so listener callbacks (if any fire) don't touch IDB
vi.mock('../storage', () => ({
  saveReminderEvent: vi.fn(async () => {}),
}));

async function freshImport() {
  vi.resetModules();
  const localNotifications = await import('../local-notifications');
  const notificationSettings = await import('../notification-settings');
  return { localNotifications, notificationSettings };
}

describe('local-notifications applySettings', () => {
  beforeEach(() => {
    prefStore.clear();
    localNotificationsMock.schedule.mockClear();
    localNotificationsMock.cancel.mockClear();
    localNotificationsMock.checkPermissions.mockClear();
    localNotificationsMock.requestPermissions.mockClear();
    localNotificationsMock.registerActionTypes.mockClear();
    localNotificationsMock.addListener.mockClear();
    // Default: native + permission granted
    capState.isNative = true;
    notifState.permission = 'granted';
    notifState.requestPermission = 'granted';
    localNotificationsMock.checkPermissions.mockImplementation(async () => ({
      display: notifState.permission,
    }));
    localNotificationsMock.requestPermissions.mockImplementation(async () => ({
      display: notifState.requestPermission,
    }));
    localNotificationsMock.cancel.mockImplementation(async () => {});
    localNotificationsMock.schedule.mockImplementation(async () => ({ notifications: [] }));
  });

  it('frequency=off cancels and does not schedule', async () => {
    const { localNotifications } = await freshImport();
    const result = await localNotifications.applySettings({
      frequency: 'off',
      time: '20:00',
      weekday: 0,
    });
    expect(result).toBe(true);
    expect(localNotificationsMock.cancel).toHaveBeenCalledTimes(1);
    expect(localNotificationsMock.schedule).not.toHaveBeenCalled();
  });

  it('daily schedules with correct hour/minute and cancels first', async () => {
    const { localNotifications } = await freshImport();
    const result = await localNotifications.applySettings({
      frequency: 'daily',
      time: '07:30',
      weekday: 0,
    });
    expect(result).toBe(true);
    expect(localNotificationsMock.cancel).toHaveBeenCalledTimes(1);
    expect(localNotificationsMock.schedule).toHaveBeenCalledTimes(1);

    // Verify cancel called before schedule via invocation order
    const cancelOrder = localNotificationsMock.cancel.mock.invocationCallOrder[0];
    const scheduleOrder = localNotificationsMock.schedule.mock.invocationCallOrder[0];
    expect(cancelOrder).toBeLessThan(scheduleOrder);

    const scheduleCalls = localNotificationsMock.schedule.mock.calls as unknown as Array<[any]>;
    const scheduleArg = scheduleCalls[0]![0];
    expect(scheduleArg.notifications).toHaveLength(1);
    const n = scheduleArg.notifications[0];
    expect(n.schedule.on).toEqual({ hour: 7, minute: 30 });
    expect(n.schedule.allowWhileIdle).toBe(true);
    expect(n.extra.mode).toBe('daily');
    expect(n.extra.firedAt).toEqual(expect.any(String));
    expect(n.actionTypeId).toBe('REMINDER_ACTIONS');
    expect(n.id).toBe(1001);
    // Title/body are translation-key passthroughs from our mock
    expect(n.title).toBe('notifications.reminderTitle.daily');
    expect(n.body).toBe('notifications.reminderBody.daily');
  });

  it('weekly with weekday=0 (Sunday) schedules plugin weekday=1', async () => {
    const { localNotifications } = await freshImport();
    await localNotifications.applySettings({
      frequency: 'weekly',
      time: '20:00',
      weekday: 0,
    });
    const scheduleCalls = localNotificationsMock.schedule.mock.calls as unknown as Array<[any]>;
    const scheduleArg = scheduleCalls[0]![0];
    expect(scheduleArg.notifications[0].schedule.on).toEqual({
      weekday: 1,
      hour: 20,
      minute: 0,
    });
    expect(scheduleArg.notifications[0].extra.mode).toBe('weekly');
    expect(scheduleArg.notifications[0].title).toBe('notifications.reminderTitle.weekly');
  });

  it('weekly with weekday=6 (Saturday) schedules plugin weekday=7', async () => {
    const { localNotifications } = await freshImport();
    await localNotifications.applySettings({
      frequency: 'weekly',
      time: '09:15',
      weekday: 6,
    });
    const scheduleCalls = localNotificationsMock.schedule.mock.calls as unknown as Array<[any]>;
    const scheduleArg = scheduleCalls[0]![0];
    expect(scheduleArg.notifications[0].schedule.on).toEqual({
      weekday: 7,
      hour: 9,
      minute: 15,
    });
  });

  it('permission denied sets permissionDeniedAt and returns false', async () => {
    notifState.permission = 'denied';
    notifState.requestPermission = 'denied';
    const { localNotifications, notificationSettings } = await freshImport();
    // Ensure mocks reflect the denied state post-reset
    localNotificationsMock.checkPermissions.mockResolvedValue({ display: 'denied' });
    localNotificationsMock.requestPermissions.mockResolvedValue({ display: 'denied' });

    const result = await localNotifications.applySettings({
      frequency: 'daily',
      time: '20:00',
      weekday: 0,
    });

    expect(result).toBe(false);
    expect(localNotificationsMock.schedule).not.toHaveBeenCalled();

    const persisted = await notificationSettings.loadNotificationSettings();
    expect(persisted.permissionDeniedAt).toEqual(expect.any(String));
  });

  it('is idempotent: two applySettings calls each cancel + schedule once', async () => {
    const { localNotifications } = await freshImport();
    const settings = {
      frequency: 'daily' as const,
      time: '08:00',
      weekday: 0,
    };
    await localNotifications.applySettings(settings);
    await localNotifications.applySettings(settings);

    expect(localNotificationsMock.cancel).toHaveBeenCalledTimes(2);
    expect(localNotificationsMock.schedule).toHaveBeenCalledTimes(2);
  });

  it('is a no-op on web (isNativePlatform=false)', async () => {
    capState.isNative = false;
    const { localNotifications } = await freshImport();
    const result = await localNotifications.applySettings({
      frequency: 'daily',
      time: '20:00',
      weekday: 0,
    });
    expect(result).toBe(true);
    expect(localNotificationsMock.cancel).not.toHaveBeenCalled();
    expect(localNotificationsMock.schedule).not.toHaveBeenCalled();
  });
});
