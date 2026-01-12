import { test as setup, expect } from '@playwright/test';
import * as fs from 'fs';

const authFile = '.auth/user.json';

setup('authenticate', async ({ page, browser }) => {
  // Check if auth file already exists and is recent (less than 7 days old)
  if (fs.existsSync(authFile)) {
    const stats = fs.statSync(authFile);
    const ageInDays = (Date.now() - stats.mtime.getTime()) / (1000 * 60 * 60 * 24);

    if (ageInDays < 7) {
      console.log('Auth file is recent, skipping authentication');
      return;
    }
  }

  console.log('Setting up test authentication...');

  // Create a new context for authentication
  const context = await browser.newContext();
  const authPage = await context.newPage();

  // Listen to console logs to debug
  authPage.on('console', msg => console.log('PAGE LOG:', msg.text()));

  // Create test session through Vite proxy (avoids CORS issues)
  const response = await context.request.post('http://localhost:5173/api/v1/auth/test-session');

  if (!response.ok()) {
    throw new Error(`Failed to create test session: ${response.status()} ${response.statusText()}`);
  }

  const sessionData = await response.json();
  console.log('Test session created for:', sessionData.user.email);

  // Navigate to the app - the session cookie should now be available
  await authPage.goto('http://localhost:5173/');

  // Wait a bit for the app to initialize and check auth
  await authPage.waitForTimeout(3000);

  // Check if there's an error message on the page
  const errorElement = authPage.locator('[class*="error"]');
  if (await errorElement.count() > 0) {
    console.log('Error on page:', await errorElement.textContent());
  }

  // Check the auth store state
  const authState = await authPage.evaluate(() => {
    // Access the Svelte store if possible
    return (window as any).__AUTH_STATE__ || 'unknown';
  });
  console.log('Auth state:', authState);

  // Verify we're authenticated by checking for authenticated UI elements
  // The Header component should be visible with user menu
  await expect(authPage.locator('header')).toBeVisible({ timeout: 10000 });

  console.log('Authentication successful! Saving session...');

  // Save authentication state
  await context.storageState({ path: authFile });

  console.log(`Auth state saved to ${authFile}`);

  // Clean up
  await authPage.close();
  await context.close();
});
