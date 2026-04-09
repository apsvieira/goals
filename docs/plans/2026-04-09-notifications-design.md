# Notifications — Local Reminders Design Plan

**Goal:** Give users an on-device "time to log your completions" reminder, configurable from a new **Notifications** section in the profile. Support `Off` / `Daily` / `Weekly` frequencies, a user-picked time (default 20:00), and a user-picked day of week for weekly mode.

**Problem:** The app currently has no reminders. Users who forget to open it miss days, breaking streaks and silently reducing engagement. The existing push-notifications scaffold (Firebase/FCM) is wired through a backend stub and doesn't actually send anything — it's not a viable delivery path for reminders, and the use case (a personal habit nudge) has no reason to leave the device.

**Approach:** On-device scheduling via `@capacitor/local-notifications`. The device schedules its own reminder, with a single "Already did it!" action button that dismisses the notification and records a local telemetry event. Server-side push is not involved. The existing FCM scaffold is left untouched for potential future server-driven notifications (announcements, social features, etc.); both code paths share the OS-level notification permission.

**Tech Stack:** Svelte 5, TypeScript, svelte-i18n, `@capacitor/local-notifications` (new dependency), `@capacitor/preferences` (already installed), existing IndexedDB layer. No backend changes.

---

## Design Decisions

### Scheduling: local only
On-device via the Capacitor LocalNotifications plugin. Works offline, no backend cron, no per-user timezone to track server-side, no dependency on Firebase being configured. The existing `push-notifications.ts` FCM flow is left alone.

### Time: user-picked, default 20:00
A native `<input type="time">` picker bound to a `HH:mm` string. Default `'20:00'` is materialized on first read, not baked into the UI, so future default changes are localized.

### Weekly day: user-picked, default Sunday
A weekday dropdown (Sun–Sat), localized via existing i18n weekday strings. Default `0` (Sunday). Only shown in Weekly mode.

### Behavior: always fire; no smart skip in v1
The reminder always fires on schedule, regardless of whether the user has already logged their completions. This is intentional — it's dramatically simpler than reactive rescheduling, it still nudges users who *think* they logged but didn't, and smart skip can be added later with no schema breakage. Section 6 sketches the future path.

### Action button: "Already did it!" → dismiss + telemetry
The notification carries a single custom action. Tapping it dismisses the notification and writes a `reminder_events` entry with `action: 'already_done'`. The button does **not** mark goals complete (risks phantom completions for users with multiple goals). The telemetry is the signal we'll use later to decide whether smart skip is worth building.

### Web: out of scope
The Capacitor LocalNotifications plugin on web only supports immediate notifications (no scheduling). The Notifications section is hidden when `!Capacitor.isNativePlatform()`. No fallback UI.

---

## Architecture

```
ProfilePage.svelte
  └── NotificationSettings.svelte          (new — UI section)
         ├── reads/writes → notificationSettings store  (new)
         └── calls       → local-notifications.ts        (new — engine)
                              │
                              ├── @capacitor/local-notifications  (new dep)
                              └── reminder_events store in IDB    (new object store)
```

**New files:**
- `frontend/src/lib/local-notifications.ts` — scheduling engine wrapping the Capacitor plugin.
- `frontend/src/lib/notification-settings.ts` — settings store + persistence via `@capacitor/preferences`.
- `frontend/src/lib/components/NotificationSettings.svelte` — UI section.
- `frontend/src/lib/__tests__/local-notifications.test.ts` — engine unit tests.
- `frontend/src/lib/__tests__/notification-settings.test.ts` — store unit tests.

**Modified files:**
- `frontend/package.json` — add `@capacitor/local-notifications`.
- `frontend/src/App.svelte` — call `initLocalNotifications()` at startup alongside `initPushNotifications()`.
- `frontend/src/lib/storage.ts` — add `reminder_events` object store + version bump + migration.
- `frontend/src/lib/components/ProfilePage.svelte` — mount `<NotificationSettings />` between Goal Stats and Data Export.
- `frontend/src/lib/i18n/en.json` and `pt-BR.json` — new `notifications.*` namespace.

---

## Data Model

### Settings (persisted via `@capacitor/preferences`, key `notification_settings`)

```typescript
type NotificationFrequency = 'off' | 'daily' | 'weekly';

interface NotificationSettings {
  frequency: NotificationFrequency;   // default: 'off'
  time: string;                        // 'HH:mm' 24-hour, default: '20:00'
  weekday: number;                     // 0=Sunday..6=Saturday, default: 0, used only when frequency='weekly'
  permissionDeniedAt?: string;         // ISO timestamp — set if OS permission denied, used to show a helper banner instead of re-prompting
}
```

Defaults materialized once on first read so adding a new field later is a simple migration.

### Telemetry (new IDB object store `reminder_events`)

```typescript
interface ReminderEvent {
  id: string;           // uuid
  timestamp: string;    // ISO — when the action happened
  action: 'already_done' | 'opened_app';
  mode: 'daily' | 'weekly';
  fired_at: string;     // ISO — last-scheduled fire time of the notification that triggered this event
}
```

Stored in the existing IDB alongside `goals`, `completions`, etc. Local-only in v1 — no sync wiring. Schema is compatible with the existing `event-sync.ts` pipeline when we later want to sync to the backend.

`fired_at` is the last-scheduled time stamped into the notification's `extra` payload at schedule time, not the precise OS fire time. Acceptable for v1 — smart-skip logic cares about "was the user already done around a recent fire", not millisecond precision.

`dismissed_os` (swipe-away) is **not** capturable reliably via Capacitor and is omitted.

---

## UI

Lives inside `ProfilePage.svelte` between the Goal Stats and Data Export sections. Styled to match existing sections (title + divider + content).

```
────────────────────
Notifications                                    (h2 section-title)
Get reminded to log your completions.             (description)

Frequency      [ Off  |  Daily  |  Weekly ]      (segmented: three styled buttons)

Time           [  20:00  ]                        (native time input — hidden when Off)

Day of week    [ Sunday ▼ ]                       (dropdown — only shown in Weekly)

⚠ Notifications are blocked in system settings.   (banner — only if permissionDeniedAt set)
────────────────────
```

### Interaction rules
- **Off → Daily/Weekly:** trigger permission request *before* saving. If denied, snap back to `Off`, set `permissionDeniedAt`, show banner.
- **Daily/Weekly → Off:** cancel all scheduled notifications, clear state. No confirmation.
- **Any change:** call `localNotifications.applySettings(newSettings)`, which cancels the old schedule and creates the new one. Idempotent.
- **Time input:** debounce-rescheduled on change.

Hidden entirely on web (`!Capacitor.isNativePlatform()`).

---

## Scheduling Engine

### Public surface (`local-notifications.ts`)

```typescript
export async function initLocalNotifications(): Promise<void>;
export async function applySettings(s: NotificationSettings): Promise<boolean>;
export async function requestPermission(): Promise<boolean>;
```

### `applySettings` algorithm
1. Cancel any previously scheduled reminder (stable fixed notification ID, e.g. `1001`).
2. If `frequency === 'off'` → return `true`.
3. Ensure permission granted; if not, request it. If still denied, set `permissionDeniedAt` and return `false`.
4. Build the schedule options:
   - **Daily:** `schedule: { on: { hour, minute } }` — fires infinitely on each matching local time.
   - **Weekly:** `schedule: { on: { weekday, hour, minute } }`. Convert our `0..6` (Sun..Sat) to the plugin's iOS-style `1..7` (Sun..Sat) numbering.
5. Attach `actionTypeId: 'REMINDER_ACTIONS'` and `extra: { mode, firedAt }`.
6. Call `LocalNotifications.schedule(...)`.

### Cross-platform caveats
- **Android recurrence:** historically flaky in some Capacitor versions for `on`-based weekday recurrence. **Mitigation if testing reveals issues:** fall back to scheduling a single `at:` occurrence and rescheduling on app resume + on `localNotificationActionPerformed`. Tested pattern, survives reboots. The public API in this design doesn't need to change — this is an internal implementation swap.
- **DST / timezone:** `on` matches wall-clock local time; OS handles DST. No manual code.

### Startup (`initLocalNotifications`)
Called once in `App.svelte` after auth, alongside `initPushNotifications()`:
1. Skip if `!Capacitor.isNativePlatform()`.
2. Register action types (needed for "Already did it!" button).
3. Attach `localNotificationActionPerformed` listener.
4. Read persisted settings and call `applySettings()` to re-sync OS state (defensive — the OS may have cleared schedules after a reinstall).

### Locale changes
Action type titles and notification title/body strings are frozen at registration/schedule time. The app re-registers action types and reschedules the current reminder when the locale store changes.

---

## Action Handling

### Registration (at startup)

```typescript
await LocalNotifications.registerActionTypes({
  types: [{
    id: 'REMINDER_ACTIONS',
    actions: [{ id: 'already_done', title: t('notifications.actionAlreadyDone') }],
  }],
});
```

### Listener (at startup, once)

```typescript
LocalNotifications.addListener('localNotificationActionPerformed', async (event) => {
  const mode = event.notification.extra?.mode;
  const firedAt = event.notification.extra?.firedAt;

  if (event.actionId === 'already_done') {
    await logReminderEvent({
      action: 'already_done',
      mode, fired_at: firedAt,
      timestamp: new Date().toISOString(),
    });
    // No navigation — user said they're done, leave them alone.
  } else if (event.actionId === 'tap') {
    await logReminderEvent({
      action: 'opened_app',
      mode, fired_at: firedAt,
      timestamp: new Date().toISOString(),
    });
  }
});
```

### Notification copy (localized at schedule time)
- **Daily** — title: "Time to track your day". Body: "Don't forget to log your completions."
- **Weekly** — title: "Weekly check-in". Body: "How did your habits go this week?"

Exact strings and Portuguese equivalents refined during implementation.

---

## Future Smart-Skip Exploration (Option B — not v1)

The core constraint: Capacitor LocalNotifications fire in native code; JavaScript cannot run at fire time to decide whether to show the notification. So "smart skip" must *pre-cancel* notifications based on user state.

### Approach 1: Reactive rescheduling (recommended future path)
Every time completion state changes (toggle, add/remove goal), a debounced job runs:
1. Compute: "is the user done for the current period?" (Daily: all active goals completed today. Weekly: all active goals have hit their weekly targets, or a simpler heuristic.)
2. If done → cancel the next scheduled notification and schedule the one after it (tomorrow / next week).
3. If not done → ensure the normal next notification is scheduled.

Edge cases: user completes → we cancel → they un-check → we must reschedule → window may have passed. Needs a robust "next fire time" calculator tightly coupled with the `stores.ts` reactivity.

### Approach 2: Silent background fetch
Use Capacitor Background Runner (or a native plugin) to wake up N minutes before the scheduled fire and decide whether to post a local notification. More "correct" but adds a background-task dependency, and iOS background execution is unreliable.

### What v1 enables
- `already_done` telemetry gives us the distribution of "how often are users self-reporting done before the reminder fires?" If it's high, smart skip is worth building.
- The settings schema has room to grow (e.g., a `smartSkip: boolean` field).
- `applySettings()` owns all scheduling logic — introducing a smart mode is a localized change.

**Recommendation:** ship v1 as designed, collect 2–4 weeks of `already_done` telemetry, then decide on Approach 1.

---

## Testing Strategy

### Unit (Vitest)
Engine tests with `@capacitor/local-notifications` mocked. Verify:
- `applySettings` cancels then schedules (correct order).
- Weekday conversion `0..6` → `1..7`.
- `off` cancels without scheduling.
- Permission-denied path sets `permissionDeniedAt` and returns `false`.
- Idempotency: two consecutive calls with identical settings produce the same OS state.

Settings store tests with `@capacitor/preferences` mocked. Verify:
- Round-trip persistence.
- Default materialization on first read.
- Field-level migrations (unknown fields preserved).

### E2E (Playwright)
UI flow only — the actual OS scheduling cannot be exercised in Playwright.
- Segmented control switches frequency.
- Time input shows/hides with frequency.
- Weekday dropdown shows only in weekly mode.
- Permission-denied banner appears when permission mocked as denied.

### Manual on Android (via CI-built APK)
- Daily at 20:00 — verify fires at scheduled time.
- Weekly on Sunday — verify.
- `Off` — verify no notifications.
- Tap "Already did it!" — verify telemetry event logged in IDB.
- Tap notification body — verify app opens and telemetry logged.
- Rapid frequency toggling — verify no orphaned scheduled notifications.

---

## i18n Keys

New `notifications.*` namespace in `en.json` and `pt-BR.json`:

- `notifications.title`
- `notifications.description`
- `notifications.frequency.off` / `.daily` / `.weekly`
- `notifications.time`
- `notifications.dayOfWeek`
- `notifications.permissionDenied`
- `notifications.reminderTitle.daily` / `.weekly`
- `notifications.reminderBody.daily` / `.weekly`
- `notifications.actionAlreadyDone`

Weekday option labels reuse the existing calendar weekday strings.

---

## Out of Scope for v1

Flagged explicitly so they don't scope-creep during implementation:

- Web support (no reliable scheduling via Web Notifications API).
- Smart skip (Approach 1 or 2 from section above).
- Per-goal reminders.
- Multiple daily reminders.
- Sound / vibration customization.
- Telemetry sync to backend.
- iOS support (Android-only for now; the Capacitor plugin is cross-platform but iOS testing/certificates aren't part of this scope).
