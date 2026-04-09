import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';
import { GoalEditorPage } from './pages/GoalEditorPage';
import { generateTestGoalName } from './helpers/test-data';

/** Format a Date as YYYY-MM-DD (local time, matching how the app keys cells). */
function isoDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

test.describe('Weekday-aligned calendar grid', () => {
  let homePage: HomePage;
  let editorPage: GoalEditorPage;

  test.beforeEach(async ({ page }) => {
    homePage = new HomePage(page);
    editorPage = new GoalEditorPage(page);
    await homePage.goto();
  });

  test('weekday header is visible and shows 7 localized labels', async ({ page }) => {
    const header = page.locator('.weekday-header');
    await expect(header).toBeVisible();

    const cells = header.locator('.weekday-cell');
    await expect(cells).toHaveCount(7);

    // English labels: S M T W T F S
    const labels = await cells.allTextContents();
    expect(labels.map(l => l.trim())).toEqual(['S', 'M', 'T', 'W', 'T', 'F', 'S']);
  });

  test('day cells align under their weekday columns (spot check)', async ({ page }) => {
    const goalName = generateTestGoalName('Align');
    await homePage.createGoal(goalName);

    // Pick a known date and verify its grid column matches its weekday.
    // Use the first current-month cell (the 1st of the month). Its
    // grid-column-start should equal (getDay() + 1).
    const now = new Date();
    const firstOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
    const expectedColumn = firstOfMonth.getDay() + 1; // CSS grid is 1-indexed

    const row = await homePage.getGoalRow(goalName);
    const cell = row.locator(`button[data-date="${isoDate(firstOfMonth)}"][data-outside-month="false"]`);
    await expect(cell).toBeVisible();

    const col = await cell.evaluate((el) => {
      const computed = window.getComputedStyle(el);
      return computed.gridColumnStart;
    });

    // Chrome reports gridColumnStart as either a number or "auto" — if auto,
    // the cell is placed by auto-flow; compare its DOM index within its grid
    // row instead.
    if (col === 'auto' || col === '') {
      // Fallback: the first-of-month cell's index within the grid
      // should equal the leading-cell count (which equals firstOfMonth.getDay()).
      const idx = await cell.evaluate((el) => {
        const parent = el.parentElement;
        if (!parent) return -1;
        return Array.from(parent.children).indexOf(el);
      });
      expect(idx).toBe(firstOfMonth.getDay());
    } else {
      expect(parseInt(col, 10)).toBe(expectedColumn);
    }

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('toggling a leading-adjacent M-1 cell within the 7-day window persists across reload', async ({ page }) => {
    // The interactive window in DayGrid.svelte is [today-6, today] (7 days
    // inclusive). A prev-month day only falls inside that window when
    // today-6 <= 0, i.e. todayDayOfMonth <= 6. On the 7th, today-6 is the
    // 1st of the current month, so no prev-month days are in the window
    // and this test is meaningless — skip it.
    const now = new Date();
    const todayDayOfMonth = now.getDate();
    test.skip(
      todayDayOfMonth > 6,
      'Only meaningful when today <= 6 so prev-month days fall inside the [today-6, today] 7-day window'
    );

    // Pick a date from the previous month that is still within the 7-day
    // lockout window (so it is interactive).
    const targetDate = new Date(now);
    targetDate.setDate(now.getDate() - (todayDayOfMonth - 1) - 1); // last day of prev month
    const targetIso = isoDate(targetDate);

    const goalName = generateTestGoalName('Leading Toggle');
    await homePage.createGoal(goalName);

    const row = await homePage.getGoalRow(goalName);
    const leadingCell = row.locator(
      `button[data-date="${targetIso}"][data-outside-month="true"]`
    );
    await expect(leadingCell).toBeVisible();
    await expect(leadingCell).toBeEnabled();

    await leadingCell.click();
    await expect(leadingCell).toHaveClass(/filled/);

    // Wait for sync, reload, verify persistence
    await page.waitForTimeout(5000);
    await page.reload();
    await homePage.header.waitFor({ state: 'visible', timeout: 10000 });

    const rowAfter = await homePage.getGoalRow(goalName);
    const leadingAfter = rowAfter.locator(
      `button[data-date="${targetIso}"][data-outside-month="true"]`
    );
    await expect(leadingAfter).toHaveClass(/filled/, { timeout: 10000 });

    // Clean up: toggle off and delete
    await leadingAfter.click();
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('leading-adjacent M-1 cell outside the 7-day window is disabled', async ({ page }) => {
    // Navigate to a past month so all its leading M-1 cells are out of the
    // 7-day window, then verify they are disabled.
    await homePage.navigateToMonth('prev');
    // Navigate back once more to guarantee the prev-month leading cells are
    // firmly in the past.
    await homePage.navigateToMonth('prev');

    const firstLeading = page.locator(
      '.goal-row button.day-square[data-outside-month="true"]'
    ).first();

    // If there are no goals yet, create one so the grid renders.
    const goalCount = await page.locator('.goal-row').count();
    let cleanupName: string | null = null;
    if (goalCount === 0) {
      const goalName = generateTestGoalName('Past Disabled');
      // Come back to current month to create the goal, then navigate back
      await homePage.navigateToMonth('next');
      await homePage.navigateToMonth('next');
      await homePage.createGoal(goalName);
      await homePage.navigateToMonth('prev');
      await homePage.navigateToMonth('prev');
      cleanupName = goalName;
    }

    const cell = page.locator(
      '.goal-row button.day-square[data-outside-month="true"]'
    ).first();
    await expect(cell).toBeVisible();
    // DaySquare renders its "not interactive" state as a `disabled` CSS class
    // rather than the HTML `disabled` attribute, so assert on the class.
    await expect(cell).toHaveClass(/disabled/);

    if (cleanupName) {
      await homePage.navigateToMonth('next');
      await homePage.navigateToMonth('next');
      await page.locator(`text=${cleanupName}`).click();
      await editorPage.delete();
    }
  });

  test('5-row months render exactly 35 day cells per goal', async ({ page }) => {
    // April 2026 is a 5-row month (leading=3, days=30 → sum 33).
    // The current date in these tests is 2026-04-08, so we're already on
    // April. buildMonthGrid should produce 35 cells.
    const goalName = generateTestGoalName('FiveRow');
    await homePage.createGoal(goalName);

    const row = await homePage.getGoalRow(goalName);
    await expect(row.locator('[data-date]')).toHaveCount(35);

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('6-row months render exactly 42 day cells per goal', async ({ page }) => {
    // Navigate backward five months to November 2025 (leading=6, days=30 →
    // sum 36). November 2025 is a 6-row month. We go backward rather than
    // forward because the app disables the next-month button on the current
    // month, which makes forward navigation hang in CI.
    const goalName = generateTestGoalName('SixRow');
    await homePage.createGoal(goalName);

    for (let i = 0; i < 5; i++) {
      await homePage.navigateToMonth('prev');
    }

    const row = await homePage.getGoalRow(goalName);
    await expect(row.locator('[data-date]')).toHaveCount(42);

    // Clean up: navigate forward back to the current month before deleting
    // so later tests start from a predictable state.
    for (let i = 0; i < 5; i++) {
      await homePage.navigateToMonth('next');
    }
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('root document has no scroll on a typical desktop viewport', async ({ page }) => {
    // Standard desktop viewport with a handful of goals — the app should lock
    // to 100dvh and never produce a document-level scrollbar.
    await page.setViewportSize({ width: 1280, height: 800 });

    const goalNames: string[] = [];
    for (let i = 0; i < 4; i++) {
      const name = generateTestGoalName(`Viewport ${i}`);
      await homePage.createGoal(name);
      goalNames.push(name);
    }

    const { scrollH, clientH } = await page.evaluate(() => ({
      scrollH: document.documentElement.scrollHeight,
      clientH: document.documentElement.clientHeight,
    }));
    expect(scrollH).toBe(clientH);

    // Clean up
    while (true) {
      const row = page.locator('.goal-row').filter({ hasText: 'Viewport ' }).first();
      if ((await row.count()) === 0) break;
      await row.locator('.goal-name').click();
      await editorPage.delete();
    }
  });

  test('many goals cause internal .goals scroll while weekday header stays visible', async ({ page }) => {
    // Force a smaller viewport so a dozen goals comfortably overflow .goals.
    await page.setViewportSize({ width: 1024, height: 600 });

    const goalNames: string[] = [];
    for (let i = 0; i < 12; i++) {
      const name = generateTestGoalName(`Many ${i}`);
      await homePage.createGoal(name);
      goalNames.push(name);
    }

    // .goals must have an internal scrollbar (scrollHeight > clientHeight).
    const { scrollH, clientH } = await page.evaluate(() => {
      const goals = document.querySelector('.goals') as HTMLElement | null;
      if (!goals) return { scrollH: 0, clientH: 0 };
      return { scrollH: goals.scrollHeight, clientH: goals.clientHeight };
    });
    expect(scrollH).toBeGreaterThan(clientH);

    // Scroll the goals list and confirm the sticky weekday header remains
    // in the viewport (it's inside .goals, so sticky should hold it pinned).
    await page.evaluate(() => {
      const goals = document.querySelector('.goals') as HTMLElement | null;
      if (goals) goals.scrollTop = 200;
    });

    const header = page.locator('.weekday-header');
    await expect(header).toBeInViewport();

    // Clean up
    while (true) {
      const row = page.locator('.goal-row').filter({ hasText: 'Many ' }).first();
      if ((await row.count()) === 0) break;
      await row.locator('.goal-name').click();
      await editorPage.delete();
    }
  });

  test('trailing M+1 cells render but are non-interactive', async ({ page }) => {
    const goalName = generateTestGoalName('Trailing');
    await homePage.createGoal(goalName);

    // Look for any trailing adjacent-month cell in the current month's grid.
    // Trailing cells belong to next month, all future, all disabled.
    const now = new Date();
    const firstOfNextMonth = new Date(now.getFullYear(), now.getMonth() + 1, 1);
    const nextMonthIso = isoDate(firstOfNextMonth);

    const row = await homePage.getGoalRow(goalName);
    const trailingCell = row.locator(
      `button[data-date="${nextMonthIso}"][data-outside-month="true"]`
    );

    // Depending on where the current month ends, Sept 1 of next month may or
    // may not be in the trailing row. Prefer it if present, otherwise fall
    // back to any trailing cell.
    const hasSpecific = await trailingCell.count();
    const cell = hasSpecific
      ? trailingCell
      : row.locator('button[data-outside-month="true"]').last();

    await expect(cell).toBeVisible();
    // DaySquare renders its "not interactive" state as a `disabled` CSS class
    // rather than the HTML `disabled` attribute, so assert on the class.
    await expect(cell).toHaveClass(/disabled/);

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });
});
