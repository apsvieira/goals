import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';
import { GoalEditorPage } from './pages/GoalEditorPage';
import { generateTestGoalName, getTodayDayNumber } from './helpers/test-data';

test.describe('Bug fixes: undo completion & progress tracking', () => {
  test.setTimeout(60000);
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

    // Verify it's marked (button should have .filled class)
    const goalRow = await homePage.getGoalRow(goalName);
    const dayButton = goalRow.locator(`button[aria-label="Day ${today}"]`);
    await expect(dayButton).toHaveClass(/filled/);

    // Undo: toggle off
    await homePage.toggleCompletion(goalName, today);
    await expect(dayButton).not.toHaveClass(/filled/);

    // Wait for sync to complete before reload
    await page.waitForTimeout(5000);

    // Reload and verify it's still unchecked
    await page.reload();
    await page.waitForLoadState('networkidle', { timeout: 10000 });
    await page.waitForSelector('.goal-row', { timeout: 10000 });
    const rowAfterReload = await homePage.getGoalRow(goalName);
    const btnAfterReload = rowAfterReload.locator(`button[aria-label="Day ${today}"]`);
    await expect(btnAfterReload).not.toHaveClass(/filled/);

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

    // Wait for sync to complete
    await page.waitForTimeout(5000);

    // Reload
    await page.reload();
    await page.waitForLoadState('networkidle', { timeout: 10000 });
    await page.waitForSelector('.goal-row', { timeout: 10000 });

    // Verify the completion is shown in the calendar
    const goalRow = await homePage.getGoalRow(goalName);
    const dayBtn = goalRow.locator(`button[aria-label="Day ${today}"]`);
    await expect(dayBtn).toHaveClass(/filled/, { timeout: 10000 });

    // Verify progress bar shows 1/3, not 0/3
    const progressText = goalRow.locator('.progress-text');
    await expect(progressText).toContainText('1', { timeout: 10000 });

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });
});
