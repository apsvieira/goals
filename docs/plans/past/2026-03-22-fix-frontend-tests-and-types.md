# Fix Frontend Tests and Type Errors — Implementation Plan

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all failing frontend unit tests, type checking errors, and vitest/playwright misconfiguration.

**Architecture:** Four independent fixes: (1) close IndexedDB connections in test cleanup, (2) remove dead guest-mode code from ProfilePage, (3) fix TypeScript narrowing and onMount signature in App.svelte, (4) exclude Playwright specs from Vitest.

**Tech Stack:** Svelte 5, TypeScript, Vitest, fake-indexeddb, Playwright

---

### Task 1: Fix `storage.test.ts` — close DB connection before deletion

All 9 storage tests time out because `resetDB()` nulls the `db` reference without calling `db.close()`. The `afterEach` then calls `deleteDB()`, which hangs forever under `fake-indexeddb` waiting for the open connection to close.

**Files:**
- Modify: `frontend/src/lib/storage.ts:42-44`

**Step 1: Fix `resetDB()` to close the connection**

In `frontend/src/lib/storage.ts`, change `resetDB()` from:

```typescript
export function resetDB(): void {
  db = null;
}
```

to:

```typescript
export function resetDB(): void {
  if (db) {
    db.close();
  }
  db = null;
}
```

**Step 2: Run storage tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/storage.test.ts`
Expected: All 9 tests PASS, run completes in <5 seconds instead of 170s.

**Step 3: Run full unit test suite**

Run: `cd frontend && npx vitest run`
Expected: `sync.test.ts`, `sync-error-handling.test.ts`, `api-error-handling.test.ts` still pass. `storage.test.ts` now passes. E2E suite failures remain (fixed in Task 4).

**Step 4: Commit**

```bash
git add frontend/src/lib/storage.ts
git commit -m "fix(frontend): close IndexedDB connection in resetDB() to prevent test hangs"
```

---

### Task 2: Remove dead `isGuest` references from `ProfilePage.svelte`

Guest mode was removed in commit `7cb17e9` but 6 `isGuest` references and associated CSS were left behind. The component is only rendered for authenticated users (guarded in `App.svelte` line 628), so `isGuest` is always false — the guest branches are dead code.

**Files:**
- Modify: `frontend/src/lib/components/ProfilePage.svelte:217,264-265,277,279,283,438,646`

**Step 1: Remove the guest `memberSince` reactive block**

At line 216-222, remove the entire `isGuest`-gated reactive block:

```svelte
  // Find the earliest goal creation date as a proxy for member since date for guests
  $: memberSince = isGuest && goals?.length > 0
    ? goals.reduce((earliest, goal) => {
        const goalDate = new Date(goal.created_at);
        return goalDate < earliest ? goalDate : earliest;
      }, new Date(goals[0].created_at)).toISOString()
    : null;
```

This variable `memberSince` is never used in the template (the template reads `user.created_at` directly), so it can be deleted entirely.

**Step 2: Simplify the avatar block**

Replace lines 264-274:

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
```

with:

```svelte
      <div class="avatar">
        {#if user?.avatar_url}
          <img src={user.avatar_url} alt="User avatar" />
        {:else}
          <span class="avatar-initial">{user?.name?.[0] || user?.email?.[0] || '?'}</span>
        {/if}
      </div>
```

**Step 3: Simplify the user name line**

Replace line 277:

```svelte
      <h1 class="user-name">{isGuest ? 'Anonymous' : (user?.name || user?.email?.split('@')[0] || 'User')}</h1>
```

with:

```svelte
      <h1 class="user-name">{user?.name || user?.email?.split('@')[0] || 'User'}</h1>
```

**Step 4: Simplify the email conditional**

Replace lines 279-281:

```svelte
      {#if !isGuest && user?.email}
        <p class="user-email">{user.email}</p>
      {/if}
```

with:

```svelte
      {#if user?.email}
        <p class="user-email">{user.email}</p>
      {/if}
```

**Step 5: Simplify the member-since conditional**

Replace lines 283-285:

```svelte
      {#if !isGuest && user?.created_at}
        <p class="member-since">Member since {formatMemberSince(user.created_at)}</p>
      {/if}
```

with:

```svelte
      {#if user?.created_at}
        <p class="member-since">Member since {formatMemberSince(user.created_at)}</p>
      {/if}
```

**Step 6: Remove `.guest` CSS rules**

Delete the `.avatar.guest` block (around line 438) and the `.avatar.guest svg` block (around line 646). These style the guest avatar SVG that no longer exists.

**Step 7: Run type checking**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.app.json`
Expected: The 6 `Cannot find name 'isGuest'` errors are gone. The 4 `App.svelte` errors remain (fixed in Task 3).

**Step 8: Commit**

```bash
git add frontend/src/lib/components/ProfilePage.svelte
git commit -m "fix(frontend): remove dead isGuest references from ProfilePage"
```

---

### Task 3: Fix `App.svelte` type errors

Four type errors: discriminated union narrowing (2), null safety in template (1), async onMount returning cleanup (1).

**Files:**
- Modify: `frontend/src/App.svelte:296,551,638`

**Step 1: Fix line 296 — remove non-null assertion that breaks narrowing**

The function has an early return `if (!editorState || editorState.mode !== 'edit') return;` which narrows `editorState` to `{ mode: 'edit'; goal: Goal }`. But `editorState!` re-widens the type. Replace line 296:

```typescript
      goals = goals.filter(g => g.id !== editorState!.goal.id);
```

with:

```typescript
      goals = goals.filter(g => g.id !== editorState.goal.id);
```

Note: TypeScript may still not narrow across `await` boundaries (the `await archiveGoal(...)` on line 295 is a suspension point where `editorState` could theoretically be reassigned). If the error persists after removing `!`, use a local variable instead:

```typescript
  async function handleEditorDelete() {
    if (!editorState || editorState.mode !== 'edit') return;
    const goalId = editorState.goal.id;

    try {
      await archiveGoal(goalId);
      goals = goals.filter(g => g.id !== goalId);
      editorState = null;
```

**Step 2: Fix line 638 — null + union narrowing in template**

Replace:

```svelte
        goal={editorState.mode === 'edit' ? goalsWithColors.find(g => g.id === editorState.goal.id) ?? null : null}
```

with a local binding using Svelte's `{@const}` (available in Svelte 4+, inside `{#if}` blocks):

```svelte
    {:else if editorState}
      {@const editGoalId = editorState.mode === 'edit' ? editorState.goal.id : null}
      <GoalEditor
        mode={editorState.mode}
        goal={editGoalId ? goalsWithColors.find(g => g.id === editGoalId) ?? null : null}
```

This avoids the repeated `editorState` access that TypeScript can't narrow.

**Step 3: Fix line 551 — async onMount returning cleanup**

Svelte's `onMount` expects a sync function optionally returning a cleanup function. An async function returns `Promise<() => void>`, which Svelte silently ignores as cleanup. Restructure to keep the outer function synchronous:

```typescript
  onMount(() => {
    // Synchronous setup
    currentRoute = getRouteFromPath();
    window.addEventListener('popstate', handlePopState);
    window.addEventListener('keydown', handleKeyDown);

    window.addEventListener('online', () => {
      console.log('Online, triggering sync');
      syncManager.sync().catch(console.error);
    });

    window.addEventListener('offline', () => {
      console.log('Offline');
    });

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
    };
  });
```

**Step 4: Run type checking**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.app.json`
Expected: All 10 errors resolved. Only warnings remain.

**Step 5: Run unit tests to confirm nothing broke**

Run: `cd frontend && npx vitest run`
Expected: All unit tests pass.

**Step 6: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "fix(frontend): resolve type errors in App.svelte (union narrowing, async onMount)"
```

---

### Task 4: Exclude Playwright E2E specs from Vitest

Vitest's default include pattern (`**/*.{test,spec}.{ts,js}`) picks up `e2e/*.spec.ts` files, which import from `@playwright/test` and crash when run under Vitest.

**Files:**
- Modify: `frontend/vitest.config.ts`

**Step 1: Add exclude pattern**

Change `vitest.config.ts` from:

```typescript
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
  },
});
```

to:

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

**Step 2: Run Vitest**

Run: `cd frontend && npx vitest run`
Expected: Only `src/lib/__tests__/*.test.ts` files are collected. No Playwright suite errors. All tests pass.

**Step 3: Commit**

```bash
git add frontend/vitest.config.ts
git commit -m "fix(frontend): exclude e2e directory from Vitest test collection"
```

---

### Task 5: Run full type check + test suite

Run: `cd frontend && npm run check && npm run test -- --run`
Expected: 0 errors from svelte-check, 0 test failures, 0 Playwright false positives.

Then from the repo root:

Run: `make test`
Expected: Both backend and frontend tests pass.

---

## Outdated Test Assessment

Tests 2 and 3 in the "Storage Error Handling" group (`should have onversionchange handler set` and `should log version mismatch detection`) are essentially no-ops — they call `initStorage()` then `expect(true).toBe(true)`. They don't actually test any behavior. Similarly, tests 1-2 in "Storage Error Recovery" are no-ops.

**Recommendation:** These 4 tests should be flagged for removal or replacement in a future pass. They exist as placeholders "verifying the code compiles" but provide zero coverage value. For now, fixing the `resetDB()` connection leak unblocks them all, and removing them is out of scope for this fix.
