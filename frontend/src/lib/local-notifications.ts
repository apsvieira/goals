import { LocalNotifications } from '@capacitor/local-notifications';
import { Capacitor } from '@capacitor/core';
import { get } from 'svelte/store';
import { _, locale } from 'svelte-i18n';
import {
  loadNotificationSettings,
  updateNotificationSettings,
  type NotificationSettings,
} from './notification-settings';
import { saveReminderEvent } from './storage';

export const REMINDER_NOTIFICATION_ID = 1001;
export const ACTION_TYPE_ID = 'REMINDER_ACTIONS';

let listenersRegistered = false;
let localeSubscribed = false;
let initialLocaleEmission = true;

function parseTime(hhmm: string): { hour: number; minute: number } {
  const [h, m] = hhmm.split(':').map((x) => parseInt(x, 10));
  return {
    hour: Number.isFinite(h) ? h : 20,
    minute: Number.isFinite(m) ? m : 0,
  };
}

async function registerReminderActionTypes(): Promise<void> {
  const t = get(_);
  await LocalNotifications.registerActionTypes({
    types: [
      {
        id: ACTION_TYPE_ID,
        actions: [{ id: 'already_done', title: t('notifications.actionAlreadyDone') }],
      },
    ],
  });
}

export async function requestPermission(): Promise<boolean> {
  const result = await LocalNotifications.requestPermissions();
  return result.display === 'granted';
}

export async function applySettings(s: NotificationSettings): Promise<boolean> {
  if (!Capacitor.isNativePlatform()) {
    return true;
  }

  // Cancel any existing scheduled reminder (fixed ID).
  // Cancelling a non-existent ID may throw on some platforms — swallow.
  try {
    await LocalNotifications.cancel({
      notifications: [{ id: REMINDER_NOTIFICATION_ID }],
    });
  } catch (err) {
    console.warn('[Notifications] cancel failed (likely no pending):', err);
  }

  if (s.frequency === 'off') {
    return true;
  }

  // Ensure permission is granted.
  const check = await LocalNotifications.checkPermissions();
  if (check.display !== 'granted') {
    const granted = await requestPermission();
    if (!granted) {
      await updateNotificationSettings({ permissionDeniedAt: new Date().toISOString() });
      return false;
    }
  }

  const { hour, minute } = parseTime(s.time);
  const t = get(_);
  const firedAt = new Date().toISOString();

  const isDaily = s.frequency === 'daily';
  const title = isDaily
    ? t('notifications.reminderTitle.daily')
    : t('notifications.reminderTitle.weekly');
  const body = isDaily
    ? t('notifications.reminderBody.daily')
    : t('notifications.reminderBody.weekly');

  const on = isDaily
    ? { hour, minute }
    : { weekday: s.weekday + 1, hour, minute };

  const notification = {
    id: REMINDER_NOTIFICATION_ID,
    title,
    body,
    schedule: {
      on,
      allowWhileIdle: true,
    },
    actionTypeId: ACTION_TYPE_ID,
    extra: { mode: s.frequency, firedAt },
  };

  await LocalNotifications.schedule({ notifications: [notification] });

  // Permission is now granted — clear any stale denial marker.
  if (s.permissionDeniedAt) {
    await updateNotificationSettings({ permissionDeniedAt: undefined });
  }

  return true;
}

export async function initLocalNotifications(): Promise<void> {
  if (!Capacitor.isNativePlatform()) {
    return;
  }

  try {
    await registerReminderActionTypes();

    if (!listenersRegistered) {
      listenersRegistered = true;
      await LocalNotifications.addListener('localNotificationActionPerformed', async (event) => {
        try {
          const extra = (event.notification.extra ?? {}) as { mode?: 'daily' | 'weekly'; firedAt?: string };
          const mode = extra.mode ?? 'daily';
          const firedAt = extra.firedAt ?? new Date().toISOString();

          if (event.actionId === 'already_done') {
            await saveReminderEvent({
              id: crypto.randomUUID(),
              timestamp: new Date().toISOString(),
              action: 'already_done',
              mode,
              fired_at: firedAt,
            });
          } else if (event.actionId === 'tap') {
            await saveReminderEvent({
              id: crypto.randomUUID(),
              timestamp: new Date().toISOString(),
              action: 'opened_app',
              mode,
              fired_at: firedAt,
            });
          }
        } catch (err) {
          console.error('[Notifications] Failed to handle action:', err);
        }
      });
    }

    const current = await loadNotificationSettings();
    await applySettings(current);

    if (!localeSubscribed) {
      localeSubscribed = true;
      locale.subscribe(async () => {
        if (initialLocaleEmission) {
          initialLocaleEmission = false;
          return;
        }
        try {
          await registerReminderActionTypes();
          const latest = await loadNotificationSettings();
          await applySettings(latest);
        } catch (err) {
          console.error('[Notifications] Failed to re-apply after locale change:', err);
        }
      });
    }
  } catch (err) {
    console.error('[Notifications] init failed:', err);
  }
}
