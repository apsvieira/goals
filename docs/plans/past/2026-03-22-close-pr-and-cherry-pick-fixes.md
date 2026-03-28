# Close PR #1 and Cherry-Pick Valid Fixes

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Close the stale PR `fix/frontend-tests-and-types` and apply its 4 still-valid fixes directly to main.

**Architecture:** The PR branch diverged from main months ago. Since then, 5 bug-fix commits landed on main. Merging the PR as-is would revert those fixes. Instead, we close the PR and manually apply the 4 changes that are still needed.

**Tech Stack:** Svelte 5, TypeScript, Vitest, IndexedDB (idb)

---

## Why Close PR #1

PR #1 (`fix/frontend-tests-and-types`, branch `fix/frontend-tests-and-types`) contains 18 commits, but only 4 are unique to the branch — the rest were already cherry-picked to main in prior sessions.

If merged, the PR would **revert 5 bug fixes** that landed on main after the branch diverged:

| What would break | Main commit it reverts | Why it matters |
|---|---|---|
| `currentMonth` reverted to UTC `.toISOString().slice(0,7)` | `caf89e6` | Wrong month near midnight in non-UTC timezones |
| Completion date parsing loses `.slice(0,10)` safety | `f32145a` | Progress bar breaks when dates arrive as full ISO datetimes |
| `deleteCompletion(id)` loses dual-delete by goal+date | `703e169` | Undo-completion fails when local/server IDs mismatch |
| `target_count`/`target_period` stripped from sync protocol | `fc82eb5` | Goals lose weekly/monthly targets after sync |
| `e2e/bug-fixes.spec.ts` deleted | `7027936` | Removes E2E test coverage for the above fixes |

## What Still Needs Fixing on Main

These 4 changes from the PR are valid and haven't landed yet:

1. **`resetDB()` must close DB before nulling** — Without `db.close()`, `deleteDB()` hangs on an open connection under `fake-indexeddb`, causing 9/14 unit tests to timeout (~170s).

2. **Vitest must exclude `e2e/`** — Without `exclude: ['e2e/**', 'node_modules/**']`, Vitest collects Playwright spec files and they fail.

3. **Dead `isGuest` code in ProfilePage** — 6 references to the removed guest mode remain: a `memberSince` reactive statement, guest avatar SVG branch, `class:guest` binding, conditional `isGuest` checks, and two `.avatar.guest` CSS rules.

4. **App.svelte type narrowing improvements** — `handleEditorDelete` accesses `editorState.goal.id` after `await` (where Svelte may have re-evaluated `editorState`), and the template accesses `editorState.goal.id` without narrowing the discriminated union first. Also `onMount` is `async` which means the cleanup function return is wrapped in a Promise and Svelte silently ignores it.

---

## Tasks

### Task 1: Close PR #1

**Step 1: Close the PR with a comment explaining why**

```bash
gh pr close 1 --comment "Closing: this branch diverged too far from main. Merging would revert 5 bug fixes (caf89e6, f32145a, 703e169, fc82eb5, 7027936). The 4 still-valid changes (resetDB close, vitest exclude, dead isGuest cleanup, App.svelte type narrowing) will be applied directly to main."
```

Expected: PR #1 status changes to CLOSED.

**Step 2: Delete the remote branch**

```bash
git push origin --delete fix/frontend-tests-and-types
```

Expected: Remote branch deleted. (Local branch may remain; that's fine.)

---

### Task 2: Fix `resetDB()` — close DB before nulling

**Files:**
- Modify: `frontend/src/lib/storage.ts:42-44`

**Step 1: Edit `resetDB()` to close the connection**

In `frontend/src/lib/storage.ts`, change:

```typescript
// For testing: reset the database connection
export function resetDB(): void {
  db = null;
}
```

To:

```typescript
// For testing: reset the database connection
export function resetDB(): void {
  if (db) {
    db.close();
  }
  db = null;
}
```

**Step 2: Run unit tests to verify the fix**

```bash
cd frontend && npx vitest run 2>&1 | tail -30
```

Expected: Tests still fail (because e2e files are collected), but storage tests should no longer timeout. If storage tests pass but e2e-related tests fail, the fix is working — Task 3 handles the e2e exclusion.

---

### Task 3: Exclude `e2e/` from Vitest

**Files:**
- Modify: `frontend/vitest.config.ts:4-8`

**Step 1: Add exclude to vitest config**

In `frontend/vitest.config.ts`, change:

```typescript
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
  },
});
```

To:

```typescript
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
});
```

**Step 2: Run unit tests — all should pass now**

```bash
cd frontend && npx vitest run 2>&1 | tail -20
```

Expected: All unit tests pass (no e2e collection, no storage timeouts).

**Step 3: Commit Tasks 2 and 3 together**

These are closely related (both fix the unit test suite):

```bash
git add frontend/src/lib/storage.ts frontend/vitest.config.ts
git commit -m "fix(frontend): fix unit test suite — close DB in resetDB(), exclude e2e from vitest

resetDB() now calls db.close() before nulling the reference, preventing
deleteDB() from hanging under fake-indexeddb (was causing 9/14 test timeouts).

Add exclude: ['e2e/**', 'node_modules/**'] to vitest.config.ts so Playwright
specs aren't collected by Vitest."
```

---

### Task 4: Remove dead `isGuest` code from ProfilePage

**Files:**
- Modify: `frontend/src/lib/components/ProfilePage.svelte`

There are 6 locations to clean up. All reference `isGuest` which no longer exists after guest mode was removed.

**Step 1: Remove the `memberSince` reactive statement (lines 216-222)**

Remove this block entirely:

```svelte
  // Find the earliest goal creation date as a proxy for member since date for guests
  $: memberSince = isGuest && goals?.length > 0
    ? goals.reduce((earliest, goal) => {
        const goalDate = new Date(goal.created_at);
        return goalDate < earliest ? goalDate : earliest;
      }, new Date(goals[0].created_at)).toISOString()
    : null;
```

**Step 2: Simplify the avatar/header section (lines 264-285)**

Replace:

```svelte
      <div class="avatar" class:guest={isGuest}>
        {#if isGuest}
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
            <circle cx="12" cy="7" r="4"/>
          </svg>
        {:else if user?.avatar_url}
          <img src={user.avatar_url} alt="User avatar" />
        {:else}
          <span class="avatar-initial">{user?.name?.[0] || user?.email?.[0] || '?'}</span>
        {/if}
      </div>

      <h1 class="user-name">{isGuest ? 'Anonymous' : (user?.name || user?.email?.split('@')[0] || 'User')}</h1>

      {#if !isGuest && user?.email}
        <p class="user-email">{user.email}</p>
      {/if}

      {#if !isGuest && user?.created_at}
        <p class="member-since">Member since {formatMemberSince(user.created_at)}</p>
      {/if}
```

With:

```svelte
      <div class="avatar">
        {#if user?.avatar_url}
          <img src={user.avatar_url} alt="User avatar" />
        {:else}
          <span class="avatar-initial">{user?.name?.[0] || user?.email?.[0] || '?'}</span>
        {/if}
      </div>

      <h1 class="user-name">{user?.name || user?.email?.split('@')[0] || 'User'}</h1>

      {#if user?.email}
        <p class="user-email">{user.email}</p>
      {/if}

      {#if user?.created_at}
        <p class="member-since">Member since {formatMemberSince(user.created_at)}</p>
      {/if}
```

**Step 3: Remove `.avatar.guest` CSS rule (lines 438-441)**

Remove:

```css
  .avatar.guest {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }
```

**Step 4: Remove `.avatar.guest svg` CSS rule (lines 646-649)**

Remove:

```css
    .avatar.guest svg {
      width: 2rem;
      height: 2rem;
    }
```

**Step 5: Verify no remaining `isGuest` references**

```bash
grep -n "isGuest\|guest" frontend/src/lib/components/ProfilePage.svelte
```

Expected: No matches (or only unrelated uses of the word "guest" in comments, if any).

**Step 6: Run type check**

```bash
cd frontend && npx svelte-check 2>&1 | grep -v node_modules
```

Expected: No errors in our code (the 2 `esrap` dependency errors are unrelated).

**Step 7: Commit**

```bash
git add frontend/src/lib/components/ProfilePage.svelte
git commit -m "fix(frontend): remove dead isGuest references from ProfilePage

Remove 6 leftover guest-mode references: memberSince reactive statement,
guest avatar SVG, class:guest binding, isGuest conditionals, and two
.avatar.guest CSS rules. Guest mode was removed in 7cb17e9."
```

---

### Task 5: Fix App.svelte type narrowing and onMount cleanup

**Files:**
- Modify: `frontend/src/App.svelte`

Three changes, all addressing type safety:

**Step 1: Extract `goalId` before async in `handleEditorDelete` (line 294-307)**

Replace:

```typescript
  async function handleEditorDelete() {
    if (!editorState || editorState.mode !== 'edit') return;

    try {
      await archiveGoal(editorState.goal.id);
      goals = goals.filter(g => g.id !== editorState!.goal.id);
      editorState = null;
```

With:

```typescript
  async function handleEditorDelete() {
    if (!editorState || editorState.mode !== 'edit') return;
    const goalId = editorState.goal.id;

    try {
      await archiveGoal(goalId);
      goals = goals.filter(g => g.id !== goalId);
      editorState = null;
```

This avoids accessing `editorState.goal.id` after `await` (where `editorState` may have been reactively re-set) and eliminates the non-null assertion (`!`).

**Step 2: Make `onMount` synchronous with async IIFE (lines 554-609)**

Replace:

```typescript
  onMount(async () => {
    // Initialize route from URL
    currentRoute = getRouteFromPath();
    window.addEventListener('popstate', handlePopState);
    window.addEventListener('keydown', handleKeyDown);

    // Sync on online/offline events
    window.addEventListener('online', () => {
      console.log('Online, triggering sync');
      syncManager.sync().catch(console.error);
    });

    window.addEventListener('offline', () => {
      console.log('Offline');
    });

    // Set up deep link handler for mobile OAuth callback
    let appUrlOpenListener: { remove: () => Promise<void> } | null = null;
    if (Capacitor.isNativePlatform()) {
      appUrlOpenListener = await CapApp.addListener('appUrlOpen', async (event) => {
        // Handle goaltracker://auth?token=xxx deep links
        const url = event.url;
        if (url.startsWith('goaltracker://auth')) {
          try {
            const urlObj = new URL(url);
            const token = urlObj.searchParams.get('token');
            if (token) {
              await saveToken(token);
              // Refresh auth state after saving token (which will also init push notifications)
              await checkAuth();
            }
          } catch (e) {
            console.error('Failed to handle auth deep link:', e);
          }
        }
      });

      // Add Capacitor app resume listener
      CapApp.addListener('resume', () => {
        console.log('App resumed, triggering sync');
        syncManager.sync().catch(console.error);
      });
    }

    await checkAuth();
    // Note: loadData() is called by the reactive statement when authState changes,
    // so we don't need to call it here explicitly

    return () => {
      window.removeEventListener('popstate', handlePopState);
      window.removeEventListener('keydown', handleKeyDown);
      syncManager.stopAutoSync();
      // Clean up the deep link listener
      if (appUrlOpenListener) {
        appUrlOpenListener.remove();
      }
```

With:

```typescript
  onMount(() => {
    // Initialize route from URL
    currentRoute = getRouteFromPath();
    window.addEventListener('popstate', handlePopState);
    window.addEventListener('keydown', handleKeyDown);

    // Sync on online/offline events
    window.addEventListener('online', () => {
      console.log('Online, triggering sync');
      syncManager.sync().catch(console.error);
    });

    window.addEventListener('offline', () => {
      console.log('Offline');
    });

    // Set up deep link handler for mobile OAuth callback
    let appUrlOpenListener: { remove: () => Promise<void> } | null = null;

    // Async initialization (fire-and-forget)
    (async () => {
      if (Capacitor.isNativePlatform()) {
        appUrlOpenListener = await CapApp.addListener('appUrlOpen', async (event) => {
          const url = event.url;
          if (url.startsWith('goaltracker://auth')) {
            try {
              const urlObj = new URL(url);
              const token = urlObj.searchParams.get('token');
              if (token) {
                await saveToken(token);
                await checkAuth();
              }
            } catch (e) {
              console.error('Failed to handle auth deep link:', e);
            }
          }
        });

        CapApp.addListener('resume', () => {
          console.log('App resumed, triggering sync');
          syncManager.sync().catch(console.error);
        });
      }

      await checkAuth();
    })();

    // Synchronous cleanup return
    return () => {
      window.removeEventListener('popstate', handlePopState);
      window.removeEventListener('keydown', handleKeyDown);
      syncManager.stopAutoSync();
      if (appUrlOpenListener) {
        appUrlOpenListener.remove();
      }
```

Why: Svelte's `onMount` expects a synchronous return of the cleanup function. When `onMount` is `async`, it returns `Promise<cleanup>` — Svelte silently ignores this and cleanup never runs (event listeners leak, auto-sync never stops).

**Step 3: Add `@const` for union narrowing in template (line 639-641)**

Replace:

```svelte
    {:else if editorState}
      <GoalEditor
        mode={editorState.mode}
        goal={editorState.mode === 'edit' ? goalsWithColors.find(g => g.id === editorState.goal.id) ?? null : null}
```

With:

```svelte
    {:else if editorState}
      {@const editGoalId = editorState.mode === 'edit' ? editorState.goal.id : null}
      <GoalEditor
        mode={editorState.mode}
        goal={editGoalId ? goalsWithColors.find(g => g.id === editGoalId) ?? null : null}
```

Why: TypeScript can't narrow `editorState.goal.id` inside a ternary that also checks `editorState.mode`. Extracting to `@const` lets the narrowing work.

**Step 4: Run type check**

```bash
cd frontend && npx svelte-check 2>&1 | grep -v node_modules
```

Expected: No errors in our code.

**Step 5: Run unit tests**

```bash
cd frontend && npx vitest run 2>&1 | tail -10
```

Expected: All pass.

**Step 6: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "fix(frontend): improve type safety in App.svelte

Extract goalId before await in handleEditorDelete to avoid accessing
reactive state after yield point. Make onMount synchronous with async
IIFE so cleanup function is returned synchronously (Svelte ignores
Promise<cleanup>). Add @const for discriminated union narrowing in
GoalEditor template binding."
```

---

### Task 6: Final verification

**Step 1: Run full frontend checks**

```bash
cd frontend && npm run check && npx vitest run
```

Expected: Type check passes (0 errors in our code), all unit tests pass.

**Step 2: Run backend tests**

```bash
cd /home/apsv/source/personal/goal-tracker/backend && go test ./...
```

Expected: All pass (no backend changes in this plan).

**Step 3: Verify PR is closed**

```bash
gh pr list --state closed | head -5
```

Expected: PR #1 shows as closed.
