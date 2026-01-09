# Playwright E2E Testing Guide

## Overview

This directory contains end-to-end tests for the goal-tracker application using Playwright. The tests cover:
- **Authentication** - Google OAuth flow and session management
- **Goal Management** - Create, edit, archive goals with targets
- **Completion Tracking** - Toggle completions, progress bars, persistence
- **Offline Sync** - IndexedDB storage and sync when back online

## Setup

### Initial Setup

1. **Install dependencies** (already done if you ran `npm install`):
   ```bash
   cd frontend
   npm install
   ```

2. **Start development servers** (required for tests):
   ```bash
   # Terminal 1: Frontend
   cd frontend && npm run dev

   # Terminal 2: Backend
   cd backend && go run cmd/server/main.go
   ```

3. **Authenticate for tests**:
   ```bash
   npm run test:e2e:auth
   ```

   This opens a browser where you complete the Google OAuth flow. The session is saved to `.auth/user.json` and reused for all tests.

## Running Tests

```bash
# Run all tests
npm run test:e2e

# Run tests in UI mode (recommended for development)
npm run test:e2e:ui

# Run tests in debug mode (step through tests)
npm run test:e2e:debug

# Run specific test file
npx playwright test e2e/goals.spec.ts

# Run tests in headed mode (see browser)
npx playwright test --headed

# Run tests in specific browser
npx playwright test --project=chromium
npx playwright test --project=firefox
npx playwright test --project=webkit
```

## Re-authenticating

If your session expires (after ~7 days), run:
```bash
npm run test:e2e:auth
```

## Test Structure

### Test Suites

- **`auth.spec.ts`** - Authentication verification
- **`goals.spec.ts`** - Goal CRUD operations
- **`completions.spec.ts`** - Completion tracking and progress
- **`offline-sync.spec.ts`** - Offline functionality and sync

### Test Infrastructure

- **`fixtures/base.ts`** - Custom Playwright fixtures
- **`pages/HomePage.ts`** - Page Object Model for home page
- **`pages/GoalEditorPage.ts`** - Page Object Model for goal editor
- **`helpers/api.ts`** - API wrapper for direct backend calls
- **`helpers/test-data.ts`** - Test data generators

### Authentication Scripts

- **`auth.setup.ts`** - Runs before all tests, manages authentication
- **`auth.teardown.ts`** - Cleanup after tests
- **`manual-auth.ts`** - Standalone authentication script

## Writing Tests

### Using Page Object Models

```typescript
import { test, expect } from './fixtures/base';
import { HomePage } from './pages/HomePage';

test('example test', async ({ page }) => {
  const homePage = new HomePage(page);
  await homePage.goto();
  await homePage.createGoal('My Goal');

  await expect(page.locator('text=My Goal')).toBeVisible();
});
```

### Using Custom Fixtures

```typescript
import { test, expect } from './fixtures/base';

test('example with auto-created goal', async ({ page, testGoal }) => {
  // testGoal is automatically created and cleaned up
  await expect(page.locator(`text=${testGoal.name}`)).toBeVisible();
});
```

### Using API Helpers

```typescript
import { test, expect } from '@playwright/test';
import { GoalTrackerAPI } from './helpers/api';

test('create goal via API', async ({ request }) => {
  const api = new GoalTrackerAPI(request);
  const goal = await api.createGoal('API Goal', '#5B8C5A');

  expect(goal.id).toBeDefined();
});
```

## Debugging

### Debug Mode

```bash
npm run test:e2e:debug
```

Opens the Playwright Inspector where you can:
- Step through tests line by line
- See selector suggestions
- View console logs
- Pause execution

### Screenshots and Videos

Playwright automatically captures:
- Screenshots on failure
- Videos on failure (retained only on failure)
- Traces on first retry

View reports:
```bash
npx playwright show-report
```

### Common Issues

#### "Session expired" errors
**Solution:** Re-run `npm run test:e2e:auth`

#### "Port already in use"
**Solution:**
```bash
lsof -ti:5173,8080 | xargs kill -9
```

#### Tests timing out
**Solution:** Increase timeout in test:
```typescript
test('slow test', async ({ page }) => {
  test.setTimeout(60000); // 60 seconds
  // ...
});
```

#### Flaky tests
**Solutions:**
- Add explicit waits: `await page.waitForLoadState('networkidle')`
- Use `waitForSelector` with timeout
- Check for race conditions in sync operations

#### IndexedDB errors
**Solution:** Clear browser storage:
```typescript
test.beforeEach(async ({ context }) => {
  await context.clearCookies();
});
```

## Best Practices

1. **Use Page Object Models** for reusable interactions
2. **Use fixtures** for test data setup/teardown
3. **Use `data-testid` attributes** for stable selectors (when available)
4. **Wait for network idle** after navigation
5. **Use `expect` with auto-retry** for flaky checks
6. **Group related tests** in `describe` blocks
7. **Clean up test data** in `afterEach` hooks or fixtures

## Claude MCP Integration

### What is MCP?

Model Context Protocol (MCP) allows Claude Code to directly control the browser via Playwright. This enables:
- Interactive testing and debugging
- Test generation from exploration
- Visual verification with screenshots
- Accessibility checking

### Using MCP

In Claude Code:
```
Use playwright mcp to open http://localhost:5173 and test the goal creation flow
```

Claude can:
- Open browsers and navigate
- Click elements and fill forms
- Take screenshots
- Generate tests from exploration
- Debug issues interactively

### Authentication with MCP

1. Authenticate once: `npm run test:e2e:auth`
2. Start Claude session
3. Say: "Use playwright mcp to open localhost:5173"
4. If needed, login manually in the visible browser
5. Claude continues from authenticated state

## CI/CD Integration

The tests are configured to run in CI with:
- PostgreSQL test database
- Headless browsers (chromium only in CI)
- Retry on failure (2 retries)
- Sequential execution (no parallel in CI)

## Test Configuration

Tests are configured in `playwright.config.ts`:
- **Base URL**: `http://localhost:5173`
- **Browsers**: Chromium, Firefox, Webkit
- **Mobile**: Pixel 5, iPhone 12
- **Auto-start servers**: Frontend (5173) and Backend (8080)
- **Auth storage**: `.auth/user.json`

## Further Reading

- [Playwright Documentation](https://playwright.dev)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [Page Object Models](https://playwright.dev/docs/pom)
- [Playwright MCP Guide](/home/apsv/agent-notes/playwright-mcp-claude-guide.md)
