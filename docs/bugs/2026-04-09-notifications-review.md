# Notifications implementation review — 2026-04-09

## Verdict
**CHANGES REQUIRED**

Plan conformance is largely good — the data model, storage migration, scheduling engine,
listener, startup wiring, i18n keys, and unit tests all match the specification. However
two interaction-rule deviations from the plan and one store-fallback omission need to be
addressed before merge.

## Test run results
- Test suite: **88 / 88 passed** (10 test files). Both new test files pass:
  - `src/lib/__tests__/local-notifications.test.ts` — 7/7
  - `src/lib/__tests__/notification-settings.test.ts` — 7/7
  - `src/lib/__tests__/storage.test.ts` — 9/9 (includes the new `reminder_events` cases)
- svelte-check: **10 errors, 7 warnings**. All 10 errors are pre-existing — confirmed by
  running `npm run check` on `git stash`ed working tree: identical error set, same files
  (`storage.ts` migration block referencing the dropped `operations` store, and two test
  files with `SyncEvent` union-narrowing issues in `event-sync.test.ts` and
  `events-storage.test.ts`). **Zero new errors** introduced by `local-notifications.ts`,
  `notification-settings.ts`, or `NotificationSettings.svelte`.

## Critical findings (must fix before merge)

### 1. Permission-denied path does not "snap back to Off" (plan §"UI — Interaction rules")
**File:** `frontend/src/lib/components/NotificationSettings.svelte:41-59`
**Plan text:** *"Off → Daily/Weekly: trigger permission request before saving. If denied,
snap back to Off, set permissionDeniedAt, show banner."*

The implementation persists the new frequency *before* permission is checked:
```ts
const updated = await updateNotificationSettings({ frequency: next });
const ok = await trySchedule(updated);
if (!ok) {
  // applySettings already recorded permissionDeniedAt and updated the store.
  // No need to revert — the store reflects the rejected state (banner shows).
  void candidate;
}
```
Result: when the user taps "Daily" (or "Weekly") and the OS denies permission, the
segmented control remains highlighted on the denied option, but no schedule exists. The
plan explicitly requires snapping the persisted `frequency` back to `'off'` in this case.
The implementer's inline comment acknowledges the drift but rationalises away the spec.
This is a direct spec violation and will confuse users: the UI will show "Daily" active
while nothing is actually scheduled.

**Fix:** On `!ok`, call
`await updateNotificationSettings({ frequency: 'off', permissionDeniedAt: <iso> })` (or
simply revert `frequency` to the captured previous value). Additionally, the candidate
object stored in `candidate` is never used — dead code.

### 2. "Frequency" row label reuses `notifications.title` ("Notifications")
**File:** `frontend/src/lib/components/NotificationSettings.svelte:88`

```svelte
<h2 class="section-title">{$_('notifications.title')}</h2>
...
<span class="row-label">{$_('notifications.title')}</span>
```
The plan mock-up shows the row label as **"Frequency"**. The implementation repeats the
section title ("Notifications") as the row label, so the word "Notifications" appears
twice and the user is not told what the segmented control controls. There is no
`notifications.frequencyLabel` (or equivalent) key in either `en.json` or `pt-BR.json`.

**Fix:** Add a new key (e.g. `notifications.frequencyLabel` → "Frequency" / "Frequência")
to both locales and use it here.

### 3. Fallback-retry IDB upgrade block is missing the v5 migration
**File:** `frontend/src/lib/storage.ts:111-147`

The catch branch that handles `VersionError` (used when the user has a *newer* DB from a
previous install) calls `openDB` with `DB_VERSION = 5` but its inline `upgrade` function
only handles `oldVersion < 1..4`. It does **not** create the `reminder_events` object
store for `oldVersion < 5`. After a version-mismatch delete-and-recreate, any call to
`saveReminderEvent` or `getReminderEvents` will throw "No object store named
reminder_events found".

Note the plan explicitly calls out the retry block: this is a forgotten copy.

**Fix:** Append the same `oldVersion < 5` block to the fallback `upgrade` function:
```ts
if (oldVersion < 5) {
  const reminderEventsStore = database.createObjectStore('reminder_events', { keyPath: 'id' });
  reminderEventsStore.createIndex('by-timestamp', 'timestamp');
}
```

## Nits (minor — optional cleanup)

1. `NotificationSettings.svelte:51` — `const candidate: NotificationSettings = { ...settings, frequency: next };`
   is dead code (only `void candidate` references it). Remove when implementing fix #1.

2. `local-notifications.ts:15-17` — Three module-level singletons (`listenersRegistered`,
   `localeSubscribed`, `initialLocaleEmission`) cannot be reset in tests. Not a bug
   today because each test calls `vi.resetModules() + freshImport()`, but worth a
   comment so future contributors understand why.

3. `local-notifications.ts:123` — `as { mode?: 'daily' | 'weekly'; firedAt?: string }` is
   a targeted cast, not a loose `any`. Fine, just calling it out.

4. `local-notifications.ts:22-24` — `parseTime` silently clamps invalid `HH:mm` strings to
   `20:00`/`:00`. Defensive and matches the "materialize defaults" philosophy.

5. `notification-settings.ts:60` — `void hydrateNotificationSettings()` runs at module
   import time, which means importing this file from any Vitest test will fire a
   Preferences read. The tests already mock `@capacitor/preferences`, so no leak, but
   some test authors find implicit side-effect-on-import surprising.

6. The locale store subscription in `initLocalNotifications` registers a one-time
   "initial emission" skip guard (`initialLocaleEmission`). This is correct for
   `svelte-i18n` (which fires once on subscribe with the current value), but means the
   re-register logic only kicks in on *subsequent* locale changes — verified by reading
   the code, not by test. A regression test for this would be nice-to-have.

7. `NotificationSettings.svelte` has no smooth handling for the "denied → user re-enables
   permission in OS → returns to app" case. `applySettings` *does* clear
   `permissionDeniedAt` on successful schedule (lines 104-106 of `local-notifications.ts`),
   but only after the user touches the control again. Arguably fine for v1.

8. The `permission-banner` uses hardcoded HSL-ish colors (`#b45309`, etc.) instead of CSS
   custom properties. Consistent with the rest of the file but worth noting for
   dark-mode review.

9. `handleTimeChange` debounces with `setTimeout(..., 300)` but has no cleanup in
   `onDestroy`. If the user navigates away within 300ms of a time change, the
   `updateNotificationSettings` call still fires. Low-impact but technically a dangling
   handle.

## What was done well / verified OK

- **Data model** matches the plan exactly: `NotificationSettings` (frequency/time/weekday/
  permissionDeniedAt), default materialization on first read, `ReminderEvent`
  (id/timestamp/action/mode/fired_at).
- **IDB migration (v5) in the primary upgrade block** creates `reminder_events` with
  `keyPath: 'id'` + `by-timestamp` index. `clearLocalData` now clears it.
- **File layout** exactly matches the plan's new/modified file lists.
- **UI placement** — `<NotificationSettings />` is correctly mounted in
  `ProfilePage.svelte:357`, between Goal Stats and the Data Export section, with a
  divider on each side.
- **UI hide-on-web** — `{#if Capacitor.isNativePlatform()}` wraps the whole section.
- **Time input visibility** uses `frequency !== 'off'` (shows in both daily and weekly).
- **Weekday dropdown** uses `frequency === 'weekly'` (correctly hidden in daily).
- **`applySettings` algorithm** follows all six plan steps in the correct order. Cancel
  is always called before returning for the `off` case (which is stricter than the plan,
  and correct for cleanup). Weekday conversion `s.weekday + 1` for `0..6 → 1..7` is
  right. Fixed notification ID `1001`. `actionTypeId: 'REMINDER_ACTIONS'`. `extra: { mode,
  firedAt }` payload present.
- **`initLocalNotifications`** — correctly skips non-native, registers action types,
  attaches the `localNotificationActionPerformed` listener, reads persisted settings and
  calls `applySettings` to re-sync. Listener registration is idempotent via
  `listenersRegistered` guard.
- **Locale change re-registration** — `locale.subscribe` handler re-registers action
  types and re-applies settings, with the initial-emission skip guard.
- **Action handling** — both `already_done` and `tap` (body tap → `opened_app`) are
  logged to `reminder_events`, matching the plan pseudocode. Correctly pulls `mode` and
  `firedAt` from `event.notification.extra` with sensible fallbacks.
- **App.svelte wiring** — `initLocalNotifications()` is called inside `checkAuth`,
  directly after `initPushNotifications()` (line 435). Correct placement.
- **i18n keys** — all 13 required keys present in both `en.json` and `pt-BR.json`:
  `title`, `description`, `frequency.{off,daily,weekly}`, `time`, `dayOfWeek`,
  `permissionDenied`, `reminderTitle.{daily,weekly}`, `reminderBody.{daily,weekly}`,
  `actionAlreadyDone`. Portuguese translations are idiomatic and not machine-garbled
  ("Desligado / Diário / Semanal", "Receba lembretes para registrar suas conclusões",
  "Já fiz!").
- **Unit tests — engine** — cancel-before-schedule verified via `invocationCallOrder`,
  weekday 0→1 and 6→7 both tested, `off` cancels without scheduling, permission-denied
  returns `false` and sets `permissionDeniedAt`, idempotency covered, web no-op
  additionally covered.
- **Unit tests — settings** — defaults-on-first-read, persistence materialization on
  disk, round-trip, unknown-field preservation (`smartSkip`, `futureField`), partial
  JSON field-fill, `updateNotificationSettings` store notification — all present.
- **Unit tests — storage** — added `reminder_events` save/get, overwrite via `put`,
  multi-record list, and `clearLocalData` clearing.
- **Package manifest** — `@capacitor/local-notifications@^8.0.2` added, matches the
  rest of the `@capacitor/*` v8 ecosystem.
- **TypeScript hygiene** — no `any` leaks in the new files; one targeted cast for the
  `extra` payload.

## Plan flaws discovered

1. **`tap` actionId is unverified.** The plan pseudocode branches on
   `event.actionId === 'tap'` to log an `opened_app` telemetry event. The Capacitor
   `@capacitor/local-notifications` API documents `actionId` as `'tap'` for a body tap,
   but the behavior is platform-dependent (older plugin versions and some Android OEMs
   deliver a different id, or none at all). The implementer faithfully copied the plan
   but neither plan nor implementation adds a defensive fallback (e.g., also accept
   `undefined` / default action). Worth manual verification on device before relying on
   the `opened_app` telemetry for smart-skip decisions.

2. **"Frequency" row label has no i18n key in the plan.** The mock-up shows a row label
   "Frequency" but the plan's i18n key list only includes `frequency.off`, `frequency.daily`,
   `frequency.weekly`, with no plain "Frequency" label. Implementer had to improvise and
   chose to reuse `notifications.title` (see finding #2). A plan ambiguity, but the
   implementer should have flagged it rather than silently reusing the wrong key.

3. **Plan does not specify what to do with `permissionDeniedAt` on subsequent successful
   applies.** The implementation clears it on any successful schedule, which is
   reasonable, but the plan is silent. Not a drift, just an unspecified behavior that
   the implementer had to decide.
