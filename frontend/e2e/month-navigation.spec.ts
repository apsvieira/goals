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
