# Fix Completion Undo & Progress Tracker Bugs

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix two user-reported bugs: (1) inability to "undo" (un-complete) a completion, and (2) progress trackers resetting to zero on page load.

**Architecture:** Both bugs stem from an ID mismatch between the server (UUIDs), local storage (composite or local-prefixed IDs), and the sync layer. The fix normalizes completion identity around `goal_id + date` pairs — the natural key — rather than relying on opaque IDs that diverge across layers. A secondary fix corrects a UTC-vs-local timezone inconsistency in the displayed month that causes progress miscounts.

**Tech Stack:** SvelteKit (frontend), Go (backend), Playwright (E2E tests), Vitest (unit tests), IndexedDB (local storage)

---

## Bug Analysis

### Bug 1: Can't undo a completion

**Root cause:** `deleteCompletion()` looks up the completion in IndexedDB by its `id`, but the IDs in IndexedDB never match the server UUIDs used by the UI.

Detailed flow:
1. `getCalendar()` returns completions from the server with **server-generated UUIDs** → stored in the `completions` state array.
2. `completionsByGoal` (derived from `completions`) maps `day → server-UUID`.
3. User clicks a filled day → `handleToggle` calls `deleteCompletion(server-UUID)`.
4. `deleteCompletion()` calls `getAllLocalCompletions().find(c => c.id === server-UUID)` — **fails** because IndexedDB stores completions under:
   - `local-{timestamp}-{random}` (from `createCompletion()`)
   - `{goal_id}-{date}` (from sync's `applyServerChanges()`)
5. Since the lookup fails: the IndexedDB record is not deleted, and **no sync operation is queued** (the `if (completion)` guard on line 260 of `api.ts` prevents it).
6. The UI array is filtered (appears to work), but on next `loadData()` the completion reappears from the server.

**Key files:**
- `frontend/src/App.svelte:236-259` — `handleToggle` passes server UUID to `deleteCompletion`
- `frontend/src/lib/api.ts:251-272` — `deleteCompletion` uses `id` for IndexedDB lookup
- `frontend/src/lib/sync.ts:274-283` — `applyServerChanges` creates completions with `${goal_id}-${date}` IDs

### Bug 2: Progress trackers reset to zero

**Root cause:** `currentMonth` on line 53 of `App.svelte` is computed using UTC:

```typescript
let currentMonth = new Date().toISOString().slice(0, 7);
```

For users in UTC-N timezones (Americas) after UTC midnight but before local midnight, `toISOString()` returns the next day — and at month boundaries, the **next month**. This causes:

1. Calendar displays the wrong month (e.g., April instead of March on March 31 evening).
2. `getCalendar('2026-04')` returns April completions, which are shown in the day grid.
3. `periodCompletionsMap` computes period boundaries using `new Date()` (local time = still March).
4. The monthly filter `completionDate <= now` excludes all April completions (they're "in the future" relative to local March).
5. Progress bar shows 0/N despite completions being visible in the grid.

**Secondary contributing factor:** Sync's `GoalChange` type (in `backend/internal/sync/types.go`) omits `target_count` and `target_period`. When `applyServerChanges()` saves goals to IndexedDB, these fields are lost. If the app subsequently loads offline (falling back to IndexedDB), goals lack target metadata → `hasTarget` is false → progress bars disappear entirely.

**Key files:**
- `frontend/src/App.svelte:53` — `currentMonth` UTC computation
- `frontend/src/App.svelte:141-172` — `periodCompletionsMap` uses local time boundaries
- `frontend/src/lib/sync.ts:247-289` — `applyServerChanges` creates goals without target fields
- `backend/internal/sync/types.go:20-27` — `GoalChange` struct missing target fields

---

## Task 1: Add E2E tests that reproduce both bugs

These tests should fail before the fix and pass after.

**Files:**
- Create: `frontend/e2e/bug-fixes.spec.ts`
- Reference: `frontend/e2e/fixtures/base.ts`, `frontend/e2e/pages/HomePage.ts`, `frontend/e2e/helpers/test-data.ts`

### Step 1: Write the E2E test file

```typescript
// frontend/e2e/bug-fixes.spec.ts
import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';
import { GoalEditorPage } from './pages/GoalEditorPage';
import { generateTestGoalName, getTodayDayNumber } from './helpers/test-data';

test.describe('Bug fixes: undo completion & progress tracking', () => {
  let homePage: HomePage;
  let editorPage: GoalEditorPage;
  const today = getTodayDayNumber();

  test.beforeEach(async ({ page }) => {
    homePage = new HomePage(page);
    editorPage = new GoalEditorPage(page);
    await homePage.goto();
  });

  test('undo completion persists after page reload', async ({ page }) => {
    const goalName = generateTestGoalName('Undo Bug');

    // Create goal and mark today complete
    await homePage.createGoal(goalName);
    await homePage.toggleCompletion(goalName, today);

    // Verify it's marked
    const goalRow = await homePage.getGoalRow(goalName);
    const dayButton = goalRow.locator(`button[aria-label="Day ${today}"]`);
    await expect(dayButton).toHaveAttribute('data-filled', 'true');

    // Undo: toggle off
    await homePage.toggleCompletion(goalName, today);
    await expect(dayButton).not.toHaveAttribute('data-filled', 'true');

    // Wait for sync to propagate
    await page.waitForTimeout(2000);

    // Reload and verify it's still unchecked
    await page.reload();
    await page.waitForSelector('.goal-row', { timeout: 10000 });
    const rowAfterReload = await homePage.getGoalRow(goalName);
    const btnAfterReload = rowAfterReload.locator(`button[aria-label="Day ${today}"]`);
    await expect(btnAfterReload).not.toHaveAttribute('data-filled', 'true');

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('progress bar reflects existing completions on load', async ({ page }) => {
    const goalName = generateTestGoalName('Progress Bug');

    // Create goal with weekly target of 3
    await homePage.newGoalButton.click();
    await editorPage.fillGoalDetails(goalName, 3, 'week');
    await editorPage.save();

    // Mark today complete
    await homePage.toggleCompletion(goalName, today);

    // Wait for sync
    await page.waitForTimeout(2000);

    // Reload
    await page.reload();
    await page.waitForSelector('.goal-row', { timeout: 10000 });

    // Verify progress bar shows 1/3, not 0/3
    const goalRow = await homePage.getGoalRow(goalName);
    const progressText = goalRow.locator('.progress-text');
    await expect(progressText).toContainText('1');

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });
});
```

### Step 2: Run the tests to verify they fail

Run: `cd frontend && npx playwright test e2e/bug-fixes.spec.ts --reporter=line`

Expected: Both tests FAIL — the undo test sees the completion reappear after reload; the progress test sees 0 instead of 1.

> **Note:** The `data-filled` attribute may not exist yet on `DaySquare`. Check the component; if it uses a CSS class instead, adapt the selector to match the actual DOM (e.g., check for a class like `.filled` or use `aria-pressed`). Update the test accordingly before proceeding.

### Step 3: Commit

```bash
git add frontend/e2e/bug-fixes.spec.ts
git commit -m "test: add failing E2E tests for undo-completion and progress-on-load bugs"
```

---

## Task 2: Fix `deleteCompletion` to use goal_id + date lookup

The core fix: make deletion work regardless of which ID format is stored locally.

**Files:**
- Modify: `frontend/src/lib/api.ts:251-272` — `deleteCompletion()`
- Modify: `frontend/src/App.svelte:244-247` — pass `goalId` and `date` to `deleteCompletion()`

### Step 1: Write a unit test for the fix

**File:** `frontend/src/lib/__tests__/api-delete.test.ts` (create if needed, or add to existing test file)

The test should verify that `deleteCompletion` correctly queues a sync operation even when the ID doesn't match what's in IndexedDB, by using goal_id + date as the fallback.

Since `deleteCompletion` depends on IndexedDB, this is best tested via the E2E test from Task 1. Skip a separate unit test here and rely on E2E coverage.

### Step 2: Update `deleteCompletion` signature and implementation

Change `deleteCompletion` in `frontend/src/lib/api.ts` to accept `goalId` and `date` as required parameters, so it can always build the sync payload and perform a correct local deletion:

```typescript
export async function deleteCompletion(id: string, goalId: string, date: string): Promise<void> {
  await ensureStorageInitialized();

  // Try to delete by ID first (covers local-created completions)
  await deleteLocalCompletion(id);

  // Also delete by goal_id + date (covers sync-created and server-ID mismatches)
  await deleteLocalCompletionByGoalAndDate(goalId, date);

  // Always queue the sync operation using goal_id + date (reliable payload)
  const operation: QueuedOperation = {
    id: generateId(),
    type: 'delete_completion',
    entityId: id,
    payload: { goal_id: goalId, date },
    timestamp: new Date().toISOString(),
    retryCount: 0,
  };
  await saveQueuedOperation(operation);
}
```

Key changes:
- Accepts `goalId` and `date` as parameters (no longer needs to look them up).
- Deletes from IndexedDB by both `id` AND `goal_id + date` (belt and suspenders).
- Always queues the sync operation (no conditional guard).

### Step 3: Update `handleToggle` in App.svelte to pass goalId and date

In `frontend/src/App.svelte`, update the delete branch in `handleToggle` (around line 244-247):

```typescript
// Before:
await deleteCompletion(existingId);

// After:
const [year, month] = currentMonth.split('-');
const dateStr = `${year}-${month}-${day.toString().padStart(2, '0')}`;
await deleteCompletion(existingId, goalId, dateStr);
```

Note: `date` is already computed on line 238 as `date`. Reuse it:

```typescript
async function handleToggle(goalId: string, day: number) {
  const [year, month] = currentMonth.split('-');
  const date = `${year}-${month}-${day.toString().padStart(2, '0')}`;

  const goalCompletions = completionsByGoal[goalId];
  const existingId = goalCompletions?.get(day);

  try {
    if (existingId) {
      await deleteCompletion(existingId, goalId, date);
      completions = completions.filter(c => c.id !== existingId);
      periodCompletions = periodCompletions.filter(c => c.id !== existingId);
    } else {
      // ... unchanged
    }
    // ... unchanged
  }
}
```

### Step 4: Run tests

Run: `cd frontend && npm run check` (type-check)
Run: `make test-frontend`

Expected: Type check passes; existing tests pass.

### Step 5: Run the E2E undo test

Run: `cd frontend && npx playwright test e2e/bug-fixes.spec.ts -g "undo completion" --reporter=line`

Expected: PASS (undo now persists after reload).

### Step 6: Commit

```bash
git add frontend/src/lib/api.ts frontend/src/App.svelte
git commit -m "fix: use goal_id+date for completion deletion instead of unreliable ID lookup"
```

---

## Task 3: Fix `currentMonth` to use local time instead of UTC

**Files:**
- Modify: `frontend/src/App.svelte:53` — `currentMonth` initialization

### Step 1: Fix the currentMonth computation

On line 53 of `frontend/src/App.svelte`, change:

```typescript
// Before (UTC — wrong month for users near midnight in UTC-N timezones):
let currentMonth = new Date().toISOString().slice(0, 7);

// After (local time — matches what the user expects):
let currentMonth = (() => {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
})();
```

This uses the same local-time approach already used in `helpers/test-data.ts:getCurrentMonth()`.

### Step 2: Run tests

Run: `make test-frontend`
Run: `cd frontend && npx playwright test e2e/bug-fixes.spec.ts -g "progress bar" --reporter=line`

Expected: All pass.

### Step 3: Commit

```bash
git add frontend/src/App.svelte
git commit -m "fix: compute currentMonth using local time instead of UTC"
```

---

## Task 4: Preserve target fields in sync GoalChange

When sync applies server goal changes, `target_count` and `target_period` are lost because `GoalChange` doesn't include them. This breaks progress bars when the app falls back to IndexedDB (offline).

**Files:**
- Modify: `backend/internal/sync/types.go:20-27` — add target fields to `GoalChange`
- Modify: `backend/internal/sync/merge.go:96-105` — include target fields in `GoalToChange`
- Modify: `frontend/src/lib/sync.ts:30-37` — add target fields to frontend `GoalChange` type
- Modify: `frontend/src/lib/sync.ts:247-270` — include target fields in `applyServerChanges`

### Step 1: Add target fields to backend GoalChange

In `backend/internal/sync/types.go`, update the `GoalChange` struct:

```go
type GoalChange struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Color        string    `json:"color"`
	Position     int       `json:"position"`
	TargetCount  *int      `json:"target_count,omitempty"`
	TargetPeriod *string   `json:"target_period,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	Deleted      bool      `json:"deleted"`
}
```

### Step 2: Update GoalToChange in merge.go

In `backend/internal/sync/merge.go`, update `GoalToChange`:

```go
func GoalToChange(goal *models.Goal) GoalChange {
	return GoalChange{
		ID:           goal.ID,
		Name:         goal.Name,
		Color:        goal.Color,
		Position:     goal.Position,
		TargetCount:  goal.TargetCount,
		TargetPeriod: goal.TargetPeriod,
		UpdatedAt:    goal.UpdatedAt,
		Deleted:      goal.DeletedAt != nil,
	}
}
```

### Step 3: Update MergeGoal to preserve target fields

In `backend/internal/sync/merge.go`, in the `MergeGoal` function, update the "no server goal" branch (new goal from client) and the "client wins" branch to include target fields:

For new goals (serverGoal == nil):
```go
goal := &models.Goal{
    ID:           clientChange.ID,
    Name:         clientChange.Name,
    Color:        clientChange.Color,
    Position:     clientChange.Position,
    TargetCount:  clientChange.TargetCount,
    TargetPeriod: clientChange.TargetPeriod,
    UpdatedAt:    clientChange.UpdatedAt,
    CreatedAt:    now,
}
```

For client-wins update:
```go
serverGoal.Name = clientChange.Name
serverGoal.Color = clientChange.Color
serverGoal.Position = clientChange.Position
serverGoal.TargetCount = clientChange.TargetCount
serverGoal.TargetPeriod = clientChange.TargetPeriod
serverGoal.UpdatedAt = clientChange.UpdatedAt
```

### Step 4: Update frontend GoalChange type

In `frontend/src/lib/sync.ts`, update the `GoalChange` interface:

```typescript
interface GoalChange {
  id: string;
  name: string;
  color: string;
  position: number;
  target_count?: number;
  target_period?: 'week' | 'month';
  updated_at: string;
  deleted: boolean;
}
```

### Step 5: Update applyServerChanges to include target fields

In `frontend/src/lib/sync.ts`, in `applyServerChanges`, update goal creation to include target fields:

```typescript
const goal: Goal = {
  id: goalChange.id,
  name: goalChange.name,
  color: goalChange.color,
  position: goalChange.position,
  target_count: goalChange.target_count,
  target_period: goalChange.target_period,
  created_at: goalChange.updated_at,
};
```

Apply this to both the deleted and non-deleted branches.

### Step 6: Also update the client-to-server goal sync to include target fields

In `frontend/src/lib/sync.ts`, in the `sync()` method, where `create_goal` and `update_goal` operations are converted to `goalChanges`, include target fields:

```typescript
if (goal) {
  goalChanges.push({
    id: goal.id,
    name: goal.name,
    color: goal.color,
    position: goal.position,
    target_count: goal.target_count,
    target_period: goal.target_period,
    updated_at: op.timestamp,
    deleted: !!goal.archived_at,
  });
}
```

Apply this to both the `create_goal`/`update_goal` branch and the `delete_goal` branch.

### Step 7: Run backend tests

Run: `make test-backend`

Expected: All pass.

### Step 8: Run frontend tests

Run: `make test-frontend`

Expected: All pass.

### Step 9: Commit

```bash
git add backend/internal/sync/types.go backend/internal/sync/merge.go frontend/src/lib/sync.ts
git commit -m "fix: preserve target_count and target_period through sync layer"
```

---

## Task 5: Run full E2E test suite and verify

### Step 1: Run all E2E tests

Run: `cd frontend && npx playwright test --reporter=line`

Expected: All tests pass, including the new bug-fix tests from Task 1.

### Step 2: Run full test suite

Run: `make test`

Expected: All backend and frontend tests pass.

### Step 3: Manual smoke test (optional)

1. Start dev servers: `make dev`
2. Log in, create a goal with weekly target of 3
3. Mark today and yesterday as complete
4. Verify progress bar shows 2/3
5. Refresh the page → progress bar should still show 2/3
6. Uncheck today → progress bar shows 1/3
7. Refresh → should still show 1/3

---

## Summary of changes

| File | Change |
|------|--------|
| `frontend/src/lib/api.ts` | `deleteCompletion` now takes `goalId` + `date`, deletes by both ID and goal+date, always queues sync |
| `frontend/src/App.svelte:53` | `currentMonth` uses local time instead of UTC |
| `frontend/src/App.svelte:handleToggle` | Passes `goalId` and `date` to `deleteCompletion` |
| `backend/internal/sync/types.go` | `GoalChange` includes `target_count` and `target_period` |
| `backend/internal/sync/merge.go` | `GoalToChange` and `MergeGoal` propagate target fields |
| `frontend/src/lib/sync.ts` | Frontend `GoalChange` type + `applyServerChanges` + `sync()` include target fields |
| `frontend/e2e/bug-fixes.spec.ts` | New E2E tests for both bugs |
