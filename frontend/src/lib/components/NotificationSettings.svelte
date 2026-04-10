<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { Capacitor, type PluginListenerHandle } from '@capacitor/core';
  import { App as CapApp } from '@capacitor/app';
  import {
    notificationSettings,
    updateNotificationSettings,
    DEFAULT_NOTIFICATION_SETTINGS,
    type NotificationSettings,
    type NotificationFrequency,
  } from '../notification-settings';
  import { applySettings, checkPermissionGranted } from '../local-notifications';

  let settings: NotificationSettings = { ...DEFAULT_NOTIFICATION_SETTINGS };
  const unsubscribe = notificationSettings.subscribe((s) => {
    settings = s;
  });

  const WEEKDAY_KEYS = [
    'weekday.sun_long',
    'weekday.mon_long',
    'weekday.tue_long',
    'weekday.wed_long',
    'weekday.thu_long',
    'weekday.fri_long',
    'weekday.sat_long',
  ];

  let timeDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  async function trySchedule(next: NotificationSettings): Promise<boolean> {
    try {
      return await applySettings(next);
    } catch (err) {
      console.error('[NotificationSettings] applySettings failed:', err);
      return false;
    }
  }

  async function handleFrequencyClick(next: NotificationFrequency) {
    if (next === settings.frequency) return;

    if (next === 'off') {
      const updated = await updateNotificationSettings({ frequency: 'off' });
      await trySchedule(updated);
      return;
    }

    // Switching to daily or weekly — request permission through applySettings.
    const wasOff = settings.frequency === 'off';
    const updated = await updateNotificationSettings({ frequency: next });
    const ok = await trySchedule(updated);
    if (!ok && wasOff) {
      // Permission was denied on the Off → enabled transition. Snap back to Off
      // so the frequency selector reflects reality (nothing is scheduled).
      // applySettings has already recorded permissionDeniedAt, so the banner
      // still shows via the store's permissionDeniedAt field.
      await updateNotificationSettings({ frequency: 'off' });
    }
  }

  async function handleTimeChange(event: Event) {
    const target = event.target as HTMLInputElement;
    const newTime = target.value;
    if (!newTime || newTime === settings.time) return;

    if (timeDebounceTimer) clearTimeout(timeDebounceTimer);
    timeDebounceTimer = setTimeout(async () => {
      const updated = await updateNotificationSettings({ time: newTime });
      await trySchedule(updated);
    }, 300);
  }

  async function handleWeekdayChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    const next = parseInt(target.value, 10);
    if (!Number.isFinite(next) || next === settings.weekday) return;
    const updated = await updateNotificationSettings({ weekday: next });
    await trySchedule(updated);
  }

  // When the user grants permission in OS settings and returns to the app,
  // we want the banner to clear and a schedule to be (re)installed without
  // requiring them to toggle the control again. Listen for App 'resume' on
  // native platforms and re-check permission state.
  let resumeListener: PluginListenerHandle | null = null;

  onMount(async () => {
    if (!Capacitor.isNativePlatform()) return;
    try {
      resumeListener = await CapApp.addListener('resume', async () => {
        if (!settings.permissionDeniedAt) return;
        const granted = await checkPermissionGranted();
        if (!granted) return;
        // Permission was restored — clear the denial marker and re-schedule.
        const updated = await updateNotificationSettings({ permissionDeniedAt: undefined });
        await trySchedule(updated);
      });
    } catch (err) {
      console.error('[NotificationSettings] Failed to register resume listener:', err);
    }
  });

  onDestroy(() => {
    unsubscribe();
    if (timeDebounceTimer) clearTimeout(timeDebounceTimer);
    if (resumeListener) {
      void resumeListener.remove();
      resumeListener = null;
    }
  });
</script>

<div class="notifications-section">
    <h2 class="section-title">{$_('notifications.title')}</h2>
    <p class="description">{$_('notifications.description')}</p>

    <div class="frequency-group">
      <span class="group-label">{$_('notifications.frequencyLabel')}</span>
      <div class="option-list" role="radiogroup" aria-label={$_('notifications.frequencyLabel')}>
        <button
          type="button"
          class="option"
          class:selected={settings.frequency === 'off'}
          role="radio"
          aria-checked={settings.frequency === 'off'}
          on:click={() => handleFrequencyClick('off')}
        >
          {$_('notifications.frequency.off')}
        </button>
        <button
          type="button"
          class="option"
          class:selected={settings.frequency === 'daily'}
          role="radio"
          aria-checked={settings.frequency === 'daily'}
          on:click={() => handleFrequencyClick('daily')}
        >
          {$_('notifications.frequency.daily')}
        </button>
        <button
          type="button"
          class="option"
          class:selected={settings.frequency === 'weekly'}
          role="radio"
          aria-checked={settings.frequency === 'weekly'}
          on:click={() => handleFrequencyClick('weekly')}
        >
          {$_('notifications.frequency.weekly')}
        </button>
      </div>
    </div>

    {#if settings.frequency !== 'off'}
      <div class="row">
        <label class="row-label" for="notification-time">{$_('notifications.time')}</label>
        <input
          id="notification-time"
          type="time"
          class="time-input"
          value={settings.time}
          on:change={handleTimeChange}
        />
      </div>
    {/if}

    {#if settings.frequency === 'weekly'}
      <div class="row">
        <label class="row-label" for="notification-weekday">{$_('notifications.dayOfWeek')}</label>
        <select
          id="notification-weekday"
          class="weekday-select"
          value={settings.weekday}
          on:change={handleWeekdayChange}
        >
          {#each WEEKDAY_KEYS as key, idx}
            <option value={idx}>{$_(key)}</option>
          {/each}
        </select>
      </div>
    {/if}

    {#if settings.permissionDeniedAt}
      <p class="permission-banner">{$_('notifications.permissionDenied')}</p>
    {/if}
</div>

<style>
  .notifications-section {
    padding: 0.5rem 0;
  }

  .section-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.5rem 0;
  }

  .description {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin: 0 0 1rem 0;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .row-label {
    font-size: 0.875rem;
    color: var(--text-secondary);
  }

  .frequency-group {
    margin-bottom: 1rem;
  }

  .group-label {
    display: block;
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin-bottom: 0.5rem;
  }

  .option-list {
    display: flex;
    flex-direction: column;
    border-radius: 0.5rem;
    overflow: hidden;
    background: var(--bg-secondary);
  }

  .option {
    display: block;
    width: 100%;
    padding: 0.625rem 0.875rem;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.875rem;
    text-align: left;
    cursor: pointer;
    transition: background-color 0.15s, color 0.15s;
  }

  .option + .option {
    border-top: 1px solid var(--border);
  }

  .option.selected,
  .option.selected + .option {
    border-top-color: transparent;
  }

  .option:hover {
    background: var(--bg-tertiary);
  }

  .option.selected {
    background: var(--accent);
    color: white;
  }

  .time-input,
  .weekday-select {
    padding: 0.375rem 0.5rem;
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    background: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 0.875rem;
  }

  .permission-banner {
    margin: 0.75rem 0 0 0;
    padding: 0.625rem 0.75rem;
    border-radius: 0.375rem;
    /* TODO: move to CSS variables when dark mode lands — no warning-themed
       CSS custom properties exist in the current palette (only --accent,
       --error, --error-bg). */
    background: rgba(217, 119, 6, 0.12);
    border: 1px solid rgba(217, 119, 6, 0.4);
    color: #b45309;
    font-size: 0.8125rem;
  }

  @media (max-width: 480px) {
    .section-title {
      font-size: 1rem;
    }
  }
</style>
