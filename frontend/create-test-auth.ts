#!/usr/bin/env tsx
/**
 * Create test authentication using the backend test-session endpoint
 */
import { chromium } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const authFile = path.join(__dirname, '.auth', 'user.json');

async function createTestAuth() {
  console.log('Creating test authentication...\n');

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    baseURL: 'http://localhost:5173'
  });
  const page = await context.newPage();

  try {
    // Navigate to frontend first to establish the domain
    await page.goto('http://localhost:5173');

    // Call the backend test-session endpoint via the proxy
    // This ensures the cookie is set for localhost:5173
    const response = await page.evaluate(async () => {
      try {
        const res = await fetch('/api/v1/auth/test-session', {
          method: 'POST',
          credentials: 'include'
        });
        const text = await res.text();
        let data;
        try {
          data = JSON.parse(text);
        } catch (e) {
          data = { error: 'Invalid JSON', text };
        }
        return { ok: res.ok, status: res.status, data };
      } catch (err: any) {
        return { ok: false, status: 0, error: err.message };
      }
    });

    if (!response.ok) {
      throw new Error(`Failed to create test session: ${JSON.stringify(response)}`);
    }

    console.log('Test session created for:', response.data.user.email);

    // Check if cookie was set
    const cookies = await context.cookies();
    console.log('Cookies after test-session:', JSON.stringify(cookies, null, 2));

    // Reload the page so the app can check auth with the new cookie
    await page.reload({ waitUntil: 'networkidle' });

    console.log('Page reloaded, waiting for header...');

    // Wait for the app to recognize we're authenticated
    try {
      await page.waitForSelector('header', { timeout: 15000 });
      console.log('Header found - authenticated!');
    } catch (e) {
      // Debug: check what's on the page
      const hasAuthPage = await page.locator('h1:has-text("tiny tracker")').count();
      const bodyText = await page.textContent('body');
      console.log('Has auth page (tiny tracker):', hasAuthPage > 0);
      console.log('Page content preview:', bodyText?.substring(0, 300));
      throw new Error('Header not found - authentication failed');
    }

    // Save auth state
    await context.storageState({ path: authFile });

    console.log(`\n✓ Auth state saved to ${authFile}`);
    console.log('✓ You can now run Playwright tests\n');

  } catch (error) {
    console.error('Failed to create test auth:', error);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

createTestAuth();
