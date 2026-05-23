# Fix: Show custom notification prompt before OS permission dialog

**Problem:** On first launch, `initPushNotifications()` calls `PushNotifications.requestPermissions()` (push-notifications.ts:34), which triggers the OS notification permission dialog *before* our custom in-app prompt (`NotificationPrompt.svelte`) is shown.

**Root cause:** In App.svelte:440-445, `initPushNotifications()` runs before `initLocalNotifications()`, and push init unconditionally requests OS permission.

**Fix:** Reorder initialization so our custom prompt appears first. Only initialize push notifications after the user has either seen the prompt or on returning visits.

---

### Task 1: Reorder notification initialization in App.svelte

**Files:**
- Modify: `frontend/src/App.svelte` (lines ~440-445 and ~499-509)

**Step 1: Swap init order and gate push init**

Change lines 440-445 from:

```ts
await initPushNotifications();
const { needsPrompt } = await initLocalNotifications();
if (needsPrompt) {
  showNotificationPrompt = true;
}
```

To:

```ts
const { needsPrompt } = await initLocalNotifications();
if (needsPrompt) {
  showNotificationPrompt = true;
} else {
  await initPushNotifications();
}
```

This ensures:
- First launch: only `initLocalNotifications` runs, returns `needsPrompt: true`, shows our prompt. Push init is skipped entirely (no OS dialog).
- Returning visits: `initLocalNotifications` returns `needsPrompt: false`, then push init runs normally.

**Step 2: Initialize push notifications after prompt is acknowledged**

In `handleNotificationPromptEnable` (~line 499), add `initPushNotifications()` at the end so push registration happens once the user has responded to our prompt:

```ts
async function handleNotificationPromptEnable() {
  showNotificationPrompt = false;
  await markNotificationPromptSeen();
  const granted = await requestPermission();
  if (granted) {
    const settings = await loadNotificationSettings();
    await applySettings(settings);
  } else {
    await updateNotificationSettings({ frequency: 'off', permissionDeniedAt: new Date().toISOString() });
  }
  await initPushNotifications();
}
```

In `handleNotificationPromptDismiss` (~line 511), also add push init so push notifications still get set up even if the user dismisses local reminders:

```ts
async function handleNotificationPromptDismiss() {
  showNotificationPrompt = false;
  await markNotificationPromptSeen();
  await updateNotificationSettings({ frequency: 'off' });
  await initPushNotifications();
}
```

**Step 3: Run tests**

Run: `cd frontend && npx vitest run`
Expected: all pass (no test exercises the exact ordering of push vs local init)

**Step 4: Commit**

```
fix(notifications): show custom prompt before OS permission dialog

Reorder notification initialization so initLocalNotifications runs
first. Push notification init (which triggers OS permission) is
deferred until after the user responds to the in-app prompt on
first launch.
```
