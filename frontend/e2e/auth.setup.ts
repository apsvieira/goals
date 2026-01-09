import { test as setup, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const authFile = '.auth/user.json';

setup('authenticate', async ({ page, context }) => {
  // Check if auth file already exists and is recent (less than 7 days old)
  if (fs.existsSync(authFile)) {
    const stats = fs.statSync(authFile);
    const ageInDays = (Date.now() - stats.mtime.getTime()) / (1000 * 60 * 60 * 24);

    if (ageInDays < 7) {
      console.log('Auth file is recent, skipping authentication');
      return;
    }
  }

  console.log('Setting up authentication...');

  // Go to the app
  await page.goto('/');

  // Wait for auth page to load
  await expect(page.locator('h1', { hasText: 'tiny tracker' })).toBeVisible({ timeout: 10000 });

  // Click "Sign in with Google" button
  const googleButton = page.locator('button', { hasText: 'Sign in with Google' });
  await expect(googleButton).toBeVisible();

  // MANUAL STEP: User must complete OAuth flow
  console.log('\n========================================');
  console.log('MANUAL AUTHENTICATION REQUIRED');
  console.log('========================================');
  console.log('1. A browser window will open');
  console.log('2. Click "Sign in with Google"');
  console.log('3. Complete the Google OAuth flow');
  console.log('4. Wait for the app to load');
  console.log('========================================\n');

  // Set a long timeout for manual OAuth (5 minutes)
  page.setDefaultTimeout(300000);

  // Click the button to initiate OAuth
  await googleButton.click();

  // Wait for OAuth redirect and successful login
  // The app should redirect back and show the main interface
  // We know auth succeeded when we see the Header with user profile
  await page.waitForURL('http://localhost:5173/', {
    timeout: 300000,
    waitUntil: 'networkidle'
  });

  // Verify we're authenticated by checking for authenticated UI elements
  // The Header component should be visible with user menu
  await expect(page.locator('header')).toBeVisible({ timeout: 30000 });

  // Additional verification: check that auth page is not visible
  await expect(page.locator('h1', { hasText: 'tiny tracker' })).not.toBeVisible();

  console.log('Authentication successful! Saving session...');

  // Save authentication state
  await context.storageState({ path: authFile });

  console.log(`Auth state saved to ${authFile}`);
});
