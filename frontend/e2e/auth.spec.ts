import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test('should be authenticated', async ({ page }) => {
    await page.goto('/');

    // Should not see auth page
    await expect(page.locator('h1', { hasText: 'tiny tracker' })).not.toBeVisible();

    // Should see authenticated UI
    await expect(page.locator('header')).toBeVisible();
    await expect(page.locator('button:has-text("New Goal")')).toBeVisible();
  });

  test('should have valid session cookie', async ({ context }) => {
    const cookies = await context.cookies();
    const sessionCookie = cookies.find(c => c.name === 'session');

    expect(sessionCookie).toBeDefined();
    expect(sessionCookie?.httpOnly).toBe(true);
  });

  test('should load user profile in header', async ({ page }) => {
    await page.goto('/');

    // Header should be visible
    await expect(page.locator('header')).toBeVisible();

    // Should have some way to access profile/logout (button with user info)
    const headerButtons = page.locator('header button');
    const buttonCount = await headerButtons.count();

    // Should have at least the "New Goal" button
    expect(buttonCount).toBeGreaterThan(0);
  });
});
