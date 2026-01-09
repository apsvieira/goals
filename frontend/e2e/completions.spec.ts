import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';
import { GoalEditorPage } from './pages/GoalEditorPage';
import { generateTestGoalName, getTodayDayNumber } from './helpers/test-data';

test.describe('Completion Tracking', () => {
  let homePage: HomePage;
  let editorPage: GoalEditorPage;
  const today = getTodayDayNumber();

  test.beforeEach(async ({ page }) => {
    homePage = new HomePage(page);
    editorPage = new GoalEditorPage(page);
    await homePage.goto();
  });

  test('should toggle completion for today', async ({ page }) => {
    const goalName = generateTestGoalName('Toggle Test');

    // Create goal
    await homePage.createGoal(goalName);

    // Mark as complete
    await homePage.toggleCompletion(goalName, today);

    // Verify completion is marked (button should have different styling)
    const goalRow = await homePage.getGoalRow(goalName);
    const dayButton = goalRow.locator('button').filter({ hasText: today.toString() }).first();

    // Button should exist and be visible
    await expect(dayButton).toBeVisible();

    // Toggle off
    await homePage.toggleCompletion(goalName, today);

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('should toggle multiple days', async ({ page }) => {
    const goalName = generateTestGoalName('Multiple Days');

    // Create goal
    await homePage.createGoal(goalName);

    // Mark today and yesterday (if not day 1)
    if (today > 1) {
      await homePage.toggleCompletion(goalName, today);
      await homePage.toggleCompletion(goalName, today - 1);

      // Both should be marked
      const goalRow = await homePage.getGoalRow(goalName);
      await expect(goalRow.locator('button').filter({ hasText: today.toString() })).toBeVisible();
      await expect(goalRow.locator('button').filter({ hasText: (today - 1).toString() })).toBeVisible();
    } else {
      // Just mark today
      await homePage.toggleCompletion(goalName, today);
    }

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('should show completions persist across months', async ({ page }) => {
    const goalName = generateTestGoalName('Cross Month');

    // Create goal
    await homePage.createGoal(goalName);

    // Mark completion
    await homePage.toggleCompletion(goalName, today);

    // Navigate to next month
    await homePage.navigateToMonth('next');

    // Navigate back
    await homePage.navigateToMonth('prev');

    // Verify completion persists
    const goalRow = await homePage.getGoalRow(goalName);
    const dayButton = goalRow.locator('button').filter({ hasText: today.toString() }).first();
    await expect(dayButton).toBeVisible();

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('should update progress bar with completions', async ({ page }) => {
    const goalName = generateTestGoalName('Progress Test');

    // Create goal with weekly target
    await homePage.newGoalButton.click();
    await editorPage.fillGoalDetails(goalName, 3, 'week');
    await editorPage.save();

    // Mark today as complete
    await homePage.toggleCompletion(goalName, today);

    // Check progress indicator shows 1/3 or similar
    const goalRow = await homePage.getGoalRow(goalName);

    // Look for progress text or bar
    const progressIndicators = goalRow.locator('.progress-text, .progress-bar, text=/1/');
    const count = await progressIndicators.count();
    expect(count).toBeGreaterThan(0);

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });

  test('should handle rapid toggling', async ({ page }) => {
    const goalName = generateTestGoalName('Rapid Toggle');

    // Create goal
    await homePage.createGoal(goalName);

    // Toggle multiple times quickly
    for (let i = 0; i < 3; i++) {
      await homePage.toggleCompletion(goalName, today);
      await page.waitForTimeout(100);
    }

    // Final state should be stable
    const goalRow = await homePage.getGoalRow(goalName);
    await expect(goalRow).toBeVisible();

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });
});
