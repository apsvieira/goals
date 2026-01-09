import { test as base, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

// Custom fixture type
type GoalTrackerFixtures = {
  goalTrackerPage: Page;
  testGoal: { name: string; id: string | null };
};

// Extend base test with custom fixtures
export const test = base.extend<GoalTrackerFixtures>({
  // goalTrackerPage: pre-configured page with helpers
  goalTrackerPage: async ({ page }, use) => {
    // Add custom helper methods to page
    await page.goto('/');
    await use(page);
  },

  // testGoal: automatically create and cleanup a test goal
  testGoal: async ({ page }, use) => {
    const goalName = `Test Goal ${Date.now()}`;
    let goalId: string | null = null;

    // Setup: Create a test goal
    await page.goto('/');

    // Wait for page to load
    await page.waitForSelector('button:has-text("New Goal")', { timeout: 10000 });

    await page.click('button:has-text("New Goal")');
    await page.fill('input[placeholder="Goal name"]', goalName);
    await page.click('button:has-text("Save")');

    // Wait for goal to appear
    await page.waitForSelector(`text=${goalName}`, { timeout: 5000 });

    await use({ name: goalName, id: goalId });

    // Teardown: Archive the test goal
    try {
      const goalElement = page.locator(`text=${goalName}`).first();
      if (await goalElement.isVisible()) {
        await goalElement.click();
        const archiveButton = page.locator('button:has-text("Archive")');
        if (await archiveButton.isVisible()) {
          await archiveButton.click();
        }
      }
    } catch (error) {
      // Cleanup failed, but don't fail the test
      console.warn('Failed to clean up test goal:', error);
    }
  },
});

export { expect };
