import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';
import { GoalEditorPage } from './pages/GoalEditorPage';
import { generateTestGoalName } from './helpers/test-data';

test.describe('Goal Management', () => {
  let homePage: HomePage;
  let editorPage: GoalEditorPage;

  test.beforeEach(async ({ page }) => {
    homePage = new HomePage(page);
    editorPage = new GoalEditorPage(page);
    await homePage.goto();
  });

  test('should create a new goal', async ({ page }) => {
    const goalName = generateTestGoalName();

    await homePage.createGoal(goalName);

    // Verify goal appears in list
    await expect(page.locator(`text=${goalName}`)).toBeVisible();
  });

  test('should create goal with weekly target', async ({ page }) => {
    const goalName = generateTestGoalName('Weekly Goal');

    await homePage.newGoalButton.click();
    await editorPage.fillGoalDetails(goalName, 3, 'week');
    await editorPage.save();

    // Verify goal with target appears
    const goalRow = await homePage.getGoalRow(goalName);
    await expect(goalRow).toBeVisible();

    // Check for progress indicator (progress bar or count)
    // The UI might show "0/3" or have a progress bar
    const progressIndicator = goalRow.locator('.progress-bar, .progress-text, text=/\\/3/');
    const count = await progressIndicator.count();
    expect(count).toBeGreaterThan(0);
  });

  test('should create goal with monthly target', async ({ page }) => {
    const goalName = generateTestGoalName('Monthly Goal');

    await homePage.newGoalButton.click();
    await editorPage.fillGoalDetails(goalName, 20, 'month');
    await editorPage.save();

    // Verify goal appears
    await expect(page.locator(`text=${goalName}`)).toBeVisible();
  });

  test('should edit existing goal', async ({ page }) => {
    const originalName = generateTestGoalName('Original');
    const updatedName = generateTestGoalName('Updated');

    // Create goal
    await homePage.createGoal(originalName);

    // Click goal to open editor
    await page.locator(`text=${originalName}`).click();

    // Wait for editor to open
    await expect(editorPage.nameInput).toBeVisible();

    // Update name
    await editorPage.nameInput.clear();
    await editorPage.nameInput.fill(updatedName);
    await editorPage.save();

    // Verify updated name
    await expect(page.locator(`text=${updatedName}`)).toBeVisible();
    await expect(page.locator(`text=${originalName}`)).not.toBeVisible();
  });

  test('should archive goal', async ({ page }) => {
    const goalName = generateTestGoalName('To Archive');

    // Create goal
    await homePage.createGoal(goalName);

    // Click goal to open editor
    await page.locator(`text=${goalName}`).click();

    // Wait for editor
    await expect(editorPage.deleteButton).toBeVisible();

    // Archive it
    await editorPage.delete();

    // Verify goal is gone
    await expect(page.locator(`text=${goalName}`)).not.toBeVisible();
  });

  test('should persist goals across page reloads', async ({ page }) => {
    const goalName = generateTestGoalName('Persistent');

    // Create goal
    await homePage.createGoal(goalName);

    // Reload page
    await page.reload();
    await homePage.goto();

    // Verify goal still exists
    await expect(page.locator(`text=${goalName}`)).toBeVisible();

    // Clean up
    await page.locator(`text=${goalName}`).click();
    await editorPage.delete();
  });
});
