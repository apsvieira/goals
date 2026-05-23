# Disable Future Month Navigation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent users from navigating to months after the current month, and remove all references to "future months" in the UI and code.

**Architecture:** Guard all month navigation paths (button, keyboard, swipe) with a current-month check. Visually disable the "next" button when already at the current month. Remove the dead `currentDay = -1` future-month branch since it's now unreachable.

**Tech Stack:** Svelte 5, TypeScript, Vitest (unit), Playwright (E2E)

---

### Task 1: Add `isCurrentMonth` helper and guard `nextMonth()`

**Files:**
- Modify: `frontend/src/App.svelte:55-58` (currentMonth init), `frontend/src/App.svelte:208-212` (nextMonth function)

**Step 1: Add a reactive `isCurrentMonth` derivation**

After line 58 in `App.svelte`, add:

```typescript
// Whether we're viewing the current calendar month
$: isCurrentMonth = (() => {
  const now = new Date();
  const todayMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  return currentMonth === todayMonth;
})();
```

**Step 2: Guard `nextMonth()` to be a no-op when at current month**

Replace the `nextMonth` function (lines 208-212):

```typescript
function nextMonth() {
  if (isCurrentMonth) return;
  const [year, month] = currentMonth.split('-').map(Number);
  const d = new Date(year, month, 1);
  currentMonth = d.toISOString().slice(0, 7);
}
```

**Step 3: Remove the future-month branch from `currentDay` computation**

Replace lines 113-127 (the `currentDay` reactive block):

```typescript
// Compute currentDay for disabling future dates
// If viewing current month: currentDay = today's day number
// If viewing past month: currentDay = 0 (no restriction, 7-day limit handled by DayGrid)
$: {
  const now = new Date();
  const todayMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  if (currentMonth === todayMonth) {
    currentDay = now.getDate();
  } else {
    // Past month: no day-level restriction
    currentDay = 0;
  }
}
```

**Step 4: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat: guard nextMonth() to prevent future month navigation"
```

---

### Task 2: Guard keyboard and swipe navigation

**Files:**
- Modify: `frontend/src/App.svelte:494-502` (keyboard handler), `frontend/src/App.svelte:225-239` (touch handler)

**Step 1: Guard keyboard ArrowRight**

Replace lines 499-502:

```typescript
if (e.key === 'ArrowRight') {
  if (!isCurrentMonth) {
    nextMonth();
  }
  e.preventDefault();
  return;
}
```

Note: The guard inside `nextMonth()` itself already prevents navigation, but adding the explicit `if` here keeps the intent clear and avoids any future refactoring surprises. Either approach is fine — the `nextMonth()` guard is the authoritative one.

**Step 2: Guard swipe-left (next month)**

Replace lines 232-238 in `handleTouchEnd`:

```typescript
if (Math.abs(deltaX) > SWIPE_THRESHOLD && Math.abs(deltaX) > Math.abs(deltaY) * SWIPE_RATIO) {
  if (deltaX > 0) {
    prevMonth();
  } else if (!isCurrentMonth) {
    nextMonth();
  }
}
```

**Step 3: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat: guard keyboard and swipe month navigation against future months"
```

---

### Task 3: Visually disable the "next month" button

**Files:**
- Modify: `frontend/src/lib/components/MonthNav.svelte`
- Modify: `frontend/src/lib/components/Header.svelte:74`
- Modify: `frontend/src/App.svelte` (pass prop to Header)

**Step 1: Add `disableNext` prop to MonthNav.svelte**

Update MonthNav's script section:

```typescript
export let month: string;
export let onPrev: () => void;
export let onNext: () => void;
export let disableNext: boolean = false;
```

Update the next-month button:

```svelte
<button class="nav-btn" on:click={onNext} aria-label="Next month" disabled={disableNext}>
```

Add a disabled style:

```css
.nav-btn:disabled {
  opacity: 0.25;
  cursor: default;
  pointer-events: none;
}
```

**Step 2: Thread the prop through Header.svelte**

Add prop to Header:

```typescript
export let disableNextMonth: boolean = false;
```

Pass it to MonthNav (line 74):

```svelte
<MonthNav {month} {onPrev} {onNext} disableNext={disableNextMonth} />
```

**Step 3: Pass `isCurrentMonth` from App.svelte to Header**

Update the Header usage (~line 701):

```svelte
<Header
  month={currentMonth}
  onPrev={prevMonth}
  onNext={nextMonth}
  disableNextMonth={isCurrentMonth}
  showAddForm={false}
  ...
/>
```

**Step 4: Commit**

```bash
git add frontend/src/App.svelte frontend/src/lib/components/MonthNav.svelte frontend/src/lib/components/Header.svelte
git commit -m "feat: visually disable next-month button when at current month"
```

---

### Task 4: Update welcome card text

**Files:**
- Modify: `frontend/src/App.svelte:738`

**Step 1: Remove the "future months" reference**

Replace line 738:

```svelte
<li><strong>Swipe to navigate</strong> - View past months</li>
```

(Changed from "View past or future months" to "View past months")

**Step 2: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "fix: update welcome text to remove future-month reference"
```

---

### Task 5: Add E2E test for future-month restriction

**Files:**
- Create: `frontend/e2e/month-navigation.spec.ts`

**Step 1: Write the E2E test**

```typescript
import { test, expect } from './fixtures/base';

test.describe('Month Navigation', () => {
  test('next-month button is disabled on current month', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    const nextBtn = page.locator('button[aria-label="Next month"]');
    await expect(nextBtn).toBeDisabled();
  });

  test('can navigate to previous month and back to current', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Navigate to previous month
    const prevBtn = page.locator('button[aria-label="Previous month"]');
    await prevBtn.click();

    // Next button should now be enabled
    const nextBtn = page.locator('button[aria-label="Next month"]');
    await expect(nextBtn).toBeEnabled();

    // Navigate back to current month
    await nextBtn.click();

    // Next button should be disabled again
    await expect(nextBtn).toBeDisabled();
  });

  test('cannot swipe past current month', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Get initial month display
    const monthDisplay = page.locator('.month-display');
    const initialMonth = await monthDisplay.textContent();

    // Swipe left (would navigate to next month)
    const main = page.locator('main');
    const box = await main.boundingBox();
    if (box) {
      await page.mouse.move(box.x + box.width - 50, box.y + box.height / 2);
      await page.mouse.down();
      await page.mouse.move(box.x + 50, box.y + box.height / 2, { steps: 10 });
      await page.mouse.up();
    }

    // Month should not have changed
    await expect(monthDisplay).toHaveText(initialMonth!);
  });
});
```

**Step 2: Run tests to verify**

Run: `cd frontend && npx playwright test e2e/month-navigation.spec.ts --project=chromium`

Expected: All 3 tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/month-navigation.spec.ts
git commit -m "test: add E2E tests for future-month navigation restriction"
```

---

### Task 6: Update HomePage page object

**Files:**
- Modify: `frontend/e2e/pages/HomePage.ts`

**Step 1: Update `navigateToMonth` to handle disabled state**

Add a helper method:

```typescript
async isNextMonthDisabled(): Promise<boolean> {
  return await this.nextMonthButton.isDisabled();
}
```

**Step 2: Commit**

```bash
git add frontend/e2e/pages/HomePage.ts
git commit -m "test: add isNextMonthDisabled helper to HomePage page object"
```
