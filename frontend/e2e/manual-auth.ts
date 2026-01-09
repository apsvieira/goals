#!/usr/bin/env tsx
/**
 * Manual authentication script
 * Run this when you need to refresh the authentication token
 *
 * Usage: npm run test:e2e:auth
 */
import { chromium } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const authFile = path.join(__dirname, '..', '.auth', 'user.json');

async function authenticate() {
  console.log('Starting manual authentication...\n');

  const browser = await chromium.launch({
    headless: false,
    slowMo: 100
  });

  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // Navigate to app
    await page.goto('http://localhost:5173');

    // Wait for auth page
    await page.waitForSelector('h1:has-text("tiny tracker")', { timeout: 10000 });

    console.log('========================================');
    console.log('MANUAL AUTHENTICATION');
    console.log('========================================');
    console.log('1. Click "Sign in with Google"');
    console.log('2. Complete the OAuth flow');
    console.log('3. Wait for the app to load');
    console.log('4. This window will close automatically');
    console.log('========================================\n');

    // Wait for successful authentication (URL changes back to root)
    await page.waitForURL('http://localhost:5173/', {
      timeout: 300000, // 5 minutes
      waitUntil: 'networkidle'
    });

    // Wait for authenticated UI
    await page.waitForSelector('header', { timeout: 30000 });

    // Save auth state
    const authDir = path.dirname(authFile);
    if (!fs.existsSync(authDir)) {
      fs.mkdirSync(authDir, { recursive: true });
    }

    await context.storageState({ path: authFile });

    console.log('\n✓ Authentication successful!');
    console.log(`✓ Auth state saved to ${authFile}`);
    console.log('✓ You can now run Playwright tests\n');

  } catch (error) {
    console.error('Authentication failed:', error);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

authenticate();
