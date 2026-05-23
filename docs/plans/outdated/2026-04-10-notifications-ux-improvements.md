# Notifications UX Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make notifications a first-class, transparent feature: default to daily at 19:00, prompt on first launch, dedicated settings page accessible from the menu and empty state.

**Architecture:** Add a `'notifications'` route to the app view state. Extract notification settings from ProfilePage into a standalone NotificationsPage. Add a first-launch in-app pre-prompt dialog before requesting OS permission. Gate all new UI behind `Capacitor.isNativePlatform()`.

**Tech Stack:** Svelte, TypeScript, Capacitor (local notifications, preferences), svelte-i18n, Vitest

---

### Task 1: Change default notification settings to daily at 19:00

**Files:**
- Modify: `frontend/src/lib/notification-settings.ts:13-17` (DEFAULT_NOTIFICATION_SETTINGS)
- Modify: `frontend/src/lib/__tests__/notification-settings.test.ts:36-51` (update assertions)

**Step 1: Update the default constant**

In `frontend/src/lib/notification-settings.ts`, change DEFAULT_NOTIFICATION_SETTINGS:

```ts
export const DEFAULT_NOTIFICATION_SETTINGS: NotificationSettings = {
  frequency: 'daily',
  time: '19:00',
  weekday: 0,
};
```

**Step 2: Update tests to match new defaults**

In `frontend/src/lib/__tests__/notification-settings.test.ts`:

- Test "returns defaults on first read": change `'off'` → `'daily'`, `'20:00'` → `'19:00'`
- Test "materializes defaults into storage": change `'off'` → `'daily'`, `'20:00'` → `'19:00'`
- Test "fills missing fields": change `'20:00'` → `'19:00'`
- Test "updateNotificationSettings patches and persists": change `'20:00'` → `'19:00'`

**Step 3: Run tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/notification-settings.test.ts`
Expected: all pass

**Step 4: Commit**

```
feat(notifications): change defaults to daily at 19:00
```

---

### Task 2: Add first-launch detection helpers

**Files:**
- Modify: `frontend/src/lib/notification-settings.ts` (add `isNotificationPromptSeen` and `markNotificationPromptSeen`)
- Modify: `frontend/src/lib/__tests__/notification-settings.test.ts` (add tests for new helpers)

**Step 1: Write tests for the new helpers**

Add to `frontend/src/lib/__tests__/notification-settings.test.ts`:

```ts
it('isNotificationPromptSeen returns false when no key exists', async () => {
  const mod = await freshImport();
  expect(await mod.isNotificationPromptSeen()).toBe(false);
});

it('isNotificationPromptSeen returns true after markNotificationPromptSeen', async () => {
  const mod = await freshImport();
  await mod.markNotificationPromptSeen();
  expect(await mod.isNotificationPromptSeen()).toBe(true);
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/__tests__/notification-settings.test.ts`
Expected: FAIL — functions not found

**Step 3: Implement the helpers**

In `frontend/src/lib/notification-settings.ts`, add at the end:

```ts
const PROMPT_SEEN_KEY = 'notification_prompt_seen';

export async function isNotificationPromptSeen(): Promise<boolean> {
  const { value } = await Preferences.get({ key: PROMPT_SEEN_KEY });
  return value === 'true';
}

export async function markNotificationPromptSeen(): Promise<void> {
  await Preferences.set({ key: PROMPT_SEEN_KEY, value: 'true' });
}
```

**Step 4: Run tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/notification-settings.test.ts`
Expected: all pass

**Step 5: Commit**

```
feat(notifications): add first-launch prompt detection helpers
```

---

### Task 3: Modify initLocalNotifications to signal when pre-prompt is needed

**Files:**
- Modify: `frontend/src/lib/local-notifications.ts:127-188` (change return type, add first-launch check)
- Modify: `frontend/src/lib/__tests__/local-notifications.test.ts` (add test for prompt-needed case)

**Step 1: Write tests for the new behavior**

Add to `frontend/src/lib/__tests__/local-notifications.test.ts`:

```ts
it('initLocalNotifications returns needsPrompt=true on first launch', async () => {
  // No stored settings and no prompt-seen flag → first launch
  const { localNotifications } = await freshImport();
  const result = await localNotifications.initLocalNotifications();
  expect(result).toEqual({ needsPrompt: true });
  // Should NOT call applySettings/schedule when prompt is needed
  expect(localNotificationsMock.schedule).not.toHaveBeenCalled();
});

it('initLocalNotifications returns needsPrompt=false when prompt already seen', async () => {
  prefStore.set('notification_prompt_seen', 'true');
  prefStore.set(
    'notification_settings',
    JSON.stringify({ frequency: 'daily', time: '19:00', weekday: 0 }),
  );
  const { localNotifications } = await freshImport();
  const result = await localNotifications.initLocalNotifications();
  expect(result).toEqual({ needsPrompt: false });
  expect(localNotificationsMock.schedule).toHaveBeenCalledTimes(1);
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/__tests__/local-notifications.test.ts`
Expected: FAIL

**Step 3: Implement the change**

In `frontend/src/lib/local-notifications.ts`:

1. Add import: `import { isNotificationPromptSeen } from './notification-settings';`
2. Change signature: `export async function initLocalNotifications(): Promise<{ needsPrompt: boolean }>`
3. At the start, after the `isNativePlatform` guard (which returns early for web), add:

```ts
if (!Capacitor.isNativePlatform()) {
  return { needsPrompt: false };
}

const promptSeen = await isNotificationPromptSeen();

try {
  await registerReminderActionTypes();
  // ... existing listener registration ...

  if (!promptSeen) {
    // First launch — don't schedule anything; caller will show pre-prompt
    return { needsPrompt: true };
  }

  const current = await loadNotificationSettings();
  await applySettings(current);

  // ... existing locale subscription ...
```

4. At the end of the try block, return `{ needsPrompt: false }`.
5. In the catch block, return `{ needsPrompt: false }`.

**Step 4: Update existing initLocalNotifications test**

The test "skips the initial locale emission and re-registers on later changes" now needs the prompt-seen flag set. Add `prefStore.set('notification_prompt_seen', 'true');` before `freshImport()` in that test.

**Step 5: Run tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/local-notifications.test.ts`
Expected: all pass

**Step 6: Commit**

```
feat(notifications): signal first-launch prompt need from init
```

---

### Task 4: Add i18n keys for pre-prompt, menu entry, and empty-state link

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**Step 1: Add keys to en.json**

Add inside the `"notifications"` object:

```json
"prompt": {
  "title": "Stay on track",
  "body": "We'll send you a daily reminder at 19:00 to check in on your goals.",
  "enable": "Enable",
  "notNow": "Not now"
},
"pageTitle": "Notification Settings"
```

Add to the `"menu"` object:

```json
"notifications": "Notifications"
```

Add to the `"welcome"` object:

```json
"notificationLink": "Configure your reminder settings"
```

**Step 2: Add keys to pt-BR.json**

Same structure with Portuguese translations:

```json
"prompt": {
  "title": "Mantenha o foco",
  "body": "Enviaremos um lembrete diário às 19:00 para registrar seus objetivos.",
  "enable": "Ativar",
  "notNow": "Agora não"
},
"pageTitle": "Configurações de Notificação"
```

Menu: `"notifications": "Notificações"`

Welcome: `"notificationLink": "Configure seus lembretes"`

**Step 3: Run i18n tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/i18n.test.ts`
Expected: all pass (the i18n tests likely check key parity between locales)

**Step 4: Commit**

```
feat(i18n): add notification prompt, menu, and empty-state keys
```

---

### Task 5: Create the NotificationPrompt dialog component

**Files:**
- Create: `frontend/src/lib/components/NotificationPrompt.svelte`

**Step 1: Create the component**

```svelte
<script lang="ts">
  import { _ } from 'svelte-i18n';

  export let onEnable: () => void;
  export let onDismiss: () => void;

  function handleBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) {
      onDismiss();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      onDismiss();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="backdrop" on:click={handleBackdropClick} role="presentation">
  <div class="dialog" role="alertdialog" aria-labelledby="prompt-title" aria-describedby="prompt-body">
    <h2 id="prompt-title" class="title">{$_('notifications.prompt.title')}</h2>
    <p id="prompt-body" class="body">{$_('notifications.prompt.body')}</p>
    <div class="actions">
      <button class="btn btn-secondary" on:click={onDismiss}>
        {$_('notifications.prompt.notNow')}
      </button>
      <button class="btn btn-primary" on:click={onEnable}>
        {$_('notifications.prompt.enable')}
      </button>
    </div>
  </div>
</div>
```

Style: centered modal with backdrop overlay, using existing CSS variable conventions (`--bg-primary`, `--text-primary`, `--accent`, `--border`, etc.). The dialog should be compact (max-width: 20rem), with rounded corners, padding, and box-shadow consistent with existing components.

**Step 2: Commit**

```
feat(notifications): add first-launch pre-prompt dialog component
```

---

### Task 6: Create the NotificationsPage component

**Files:**
- Create: `frontend/src/lib/components/NotificationsPage.svelte`

**Step 1: Create the component**

Follow the same layout pattern as `PrivacyPolicy.svelte` — a full-screen page with a back button and content area. Embed the existing `NotificationSettings` component.

```svelte
<script lang="ts">
  import { _ } from 'svelte-i18n';
  import NotificationSettings from './NotificationSettings.svelte';

  export let onBack: () => void;
</script>

<div class="page-container">
  <div class="page-card">
    <button class="back-btn" on:click={onBack}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M19 12H5M12 19l-7-7 7-7"/>
      </svg>
      {$_('notifications.pageTitle')}
    </button>

    <NotificationSettings />
  </div>
</div>
```

Style: reuse the same layout styling from PrivacyPolicy (`.legal-container`/`.legal-card` pattern) adapted for this page. Use consistent padding, max-width, and back button styling.

**Step 2: Commit**

```
feat(notifications): add dedicated NotificationsPage component
```

---

### Task 7: Remove NotificationSettings from ProfilePage

**Files:**
- Modify: `frontend/src/lib/components/ProfilePage.svelte:1-10` (remove import)
- Modify: `frontend/src/lib/components/ProfilePage.svelte:355-359` (remove usage + surrounding divider)

**Step 1: Remove import**

Remove line 5: `import NotificationSettings from './NotificationSettings.svelte';`

**Step 2: Remove usage**

Remove the divider + `<NotificationSettings />` block (around lines 355-358):

```svelte
<!-- REMOVE THIS -->
<div class="divider"></div>
<NotificationSettings />
```

Keep the divider before the export section.

**Step 3: Run build to verify no errors**

Run: `cd frontend && npx vite build`
Expected: builds cleanly

**Step 4: Commit**

```
refactor(profile): remove notification settings section (moved to dedicated page)
```

---

### Task 8: Remove isNativePlatform guard from NotificationSettings component

**Files:**
- Modify: `frontend/src/lib/components/NotificationSettings.svelte:1-5` (remove Capacitor import)
- Modify: `frontend/src/lib/components/NotificationSettings.svelte:115,183` (remove if/endif wrapper)

**Step 1: Remove the platform guard**

The parent (NotificationsPage / App.svelte) now gates native-only rendering. Remove:
- The `Capacitor` import (line 4): `import { Capacitor, type PluginListenerHandle } from '@capacitor/core';`
- Replace with just: `import { type PluginListenerHandle } from '@capacitor/core';`
- Remove the `{#if Capacitor.isNativePlatform()}` on line 115 and its closing `{/if}` on line 183

Note: keep the `Capacitor.isNativePlatform()` check in `onMount` (line 89) — that guards the resume listener registration which should still be native-only.

But we need `Capacitor` for that onMount check. So actually: keep the import as-is, only remove the template-level `{#if}` / `{/if}` wrapper.

**Step 2: Commit**

```
refactor(notifications): remove template-level native platform guard

The parent page now handles native-only gating.
```

---

### Task 9: Add Notifications entry to UserDropdown menu

**Files:**
- Modify: `frontend/src/lib/components/UserDropdown.svelte` (add prop, add menu entry)
- Modify: `frontend/src/lib/components/Header.svelte` (add prop, pass through)

**Step 1: Modify UserDropdown**

In `UserDropdown.svelte`:

1. Add import: `import { Capacitor } from '@capacitor/core';`
2. Add prop: `export let onNotificationsClick: () => void = () => {};`
3. After the Profile button (line 40) and before the Language divider (line 42), add:

```svelte
  {#if Capacitor.isNativePlatform()}
    <button class="menu-item" on:click={handleNotificationsClick} role="menuitem">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.63-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.64 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2zm-2 1H8v-6c0-2.48 1.51-4.5 4-4.5s4 2.02 4 4.5v6z"/>
      </svg>
      <span>{$_('menu.notifications')}</span>
    </button>
  {/if}
```

4. Add handler function:
```ts
function handleNotificationsClick() {
  closeDropdown();
  onNotificationsClick();
}
```

Where `closeDropdown()` calls `onClose()`.

**Step 2: Modify Header**

In `Header.svelte`:

1. Add prop: `export let onNotificationsClick: () => void = () => {};`
2. Pass it to UserDropdown: `onNotificationsClick={onNotificationsClick}`

**Step 3: Commit**

```
feat(notifications): add Notifications entry to user dropdown menu
```

---

### Task 10: Wire everything together in App.svelte

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: Add imports**

```ts
import NotificationsPage from './lib/components/NotificationsPage.svelte';
import NotificationPrompt from './lib/components/NotificationPrompt.svelte';
import { markNotificationPromptSeen } from './lib/notification-settings';
import { requestPermission, applySettings } from './lib/local-notifications';
import { updateNotificationSettings, loadNotificationSettings } from './lib/notification-settings';
```

Note: `requestPermission` and `applySettings` are already imported from `local-notifications` if needed; check existing imports. `loadNotificationSettings` and `updateNotificationSettings` may already be imported. Only add what's missing.

**Step 2: Extend Route type**

Change: `type Route = 'home' | 'privacy';`
To: `type Route = 'home' | 'privacy' | 'notifications';`

Update `getRouteFromPath`:
```ts
function getRouteFromPath(): Route {
  const path = window.location.pathname;
  if (path === '/privacy') return 'privacy';
  if (path === '/notifications') return 'notifications';
  return 'home';
}
```

**Step 3: Add pre-prompt state**

```ts
let showNotificationPrompt = false;
```

**Step 4: Modify checkAuth to handle initLocalNotifications result**

Where `initLocalNotifications()` is called (around line 435), change:

```ts
await initLocalNotifications();
```

To:

```ts
const { needsPrompt } = await initLocalNotifications();
if (needsPrompt) {
  showNotificationPrompt = true;
}
```

**Step 5: Add pre-prompt handlers**

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
}

async function handleNotificationPromptDismiss() {
  showNotificationPrompt = false;
  await markNotificationPromptSeen();
  await updateNotificationSettings({ frequency: 'off' });
}
```

**Step 6: Add notifications navigation handler**

```ts
function handleNotificationsClick() {
  navigateTo('notifications');
}
```

**Step 7: Update template**

1. Add notifications route to the template (alongside privacy):

```svelte
{#if currentRoute === 'privacy'}
  <PrivacyPolicy onBack={() => navigateTo('home')} />
{:else if currentRoute === 'notifications'}
  <NotificationsPage onBack={() => navigateTo('home')} />
{:else if authState.type === 'loading'}
  ...
```

2. Pass `onNotificationsClick` to Header:

```svelte
<Header
  ...existing props...
  onNotificationsClick={handleNotificationsClick}
/>
```

3. Add notification link in welcome card (after the CTA button, native-only):

```svelte
{#if Capacitor.isNativePlatform()}
  <button class="welcome-notification-link" on:click={handleNotificationsClick}>
    {$_('welcome.notificationLink')}
  </button>
{/if}
```

4. Add the pre-prompt dialog (rendered at the bottom of the authenticated view, before the closing tags):

```svelte
{#if showNotificationPrompt}
  <NotificationPrompt
    onEnable={handleNotificationPromptEnable}
    onDismiss={handleNotificationPromptDismiss}
  />
{/if}
```

**Step 8: Add minimal styling for the welcome notification link**

```css
.welcome-notification-link {
  display: block;
  margin: 0.75rem auto 0;
  background: none;
  border: none;
  color: var(--accent);
  font-size: 0.875rem;
  cursor: pointer;
  text-decoration: underline;
}
```

**Step 9: Commit**

```
feat(notifications): wire pre-prompt, dedicated page, menu and empty-state link
```

---

### Task 11: Run full test suite and fix any breakage

**Step 1: Run all tests**

Run: `cd frontend && npx vitest run`
Expected: all pass

**Step 2: Fix any failures**

Common things to check:
- Tests that assert default frequency `'off'` or default time `'20:00'` need updating
- Tests that call `initLocalNotifications()` without setting `notification_prompt_seen` may get `needsPrompt: true` and no scheduling

**Step 3: Commit fixes if needed**

```
test: fix notification test assertions for new defaults
```

---

### Task 12: Manual smoke test checklist

These are manual checks to do on the built app (or via Playwright if available):

- [ ] Web: no notification UI appears (no menu entry, no welcome link, no prompt)
- [ ] Native first launch: pre-prompt dialog appears after auth
- [ ] Pre-prompt "Enable" → OS permission dialog → notifications scheduled as daily at 19:00
- [ ] Pre-prompt "Not now" → dialog closes, settings saved as off, prompt never reappears
- [ ] Menu → "Notifications" → navigates to dedicated page with all controls
- [ ] Empty state (0 goals) → "Configure your reminder settings" link → navigates to notifications page
- [ ] Profile page no longer shows notification settings section
- [ ] Changing settings on notifications page works (frequency, time, weekday)
- [ ] Back button on notifications page returns to home
