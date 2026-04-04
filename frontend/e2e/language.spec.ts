import { test, expect } from './fixtures/base';

test.describe('Language Switching', () => {
  test('defaults to English', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Header should show English text
    await expect(page.locator('button:has-text("New Goal")')).toBeVisible();
  });

  test('can switch to Portuguese via user menu', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Open user menu
    await page.locator('.user-indicator').click();

    // Click pt-BR language option
    await page.locator('button.language-btn:has-text("Português")').click();

    // Header should now show Portuguese text
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();
  });

  test('language preference persists across page reload', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Switch to Portuguese
    await page.locator('.user-indicator').click();
    await page.locator('button.language-btn:has-text("Português")').click();
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();

    // Reload page
    await page.reload();
    await page.waitForSelector('header', { timeout: 10000 });

    // Should still be in Portuguese
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();
  });

  test('aria-labels are translated in Portuguese', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('goal-tracker-locale', 'pt-BR');
    });
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Month nav buttons should have Portuguese aria-labels
    await expect(page.locator('button[aria-label="Mês anterior"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Próximo mês"]')).toBeAttached();
  });
});
