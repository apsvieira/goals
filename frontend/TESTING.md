# E2E Testing Guide

## Running E2E Tests

### Prerequisites

1. Start the backend server:
   ```bash
   cd ../backend && go run ./cmd/server
   ```

2. Start the frontend dev server:
   ```bash
   cd frontend && npm run dev
   ```

### Run Tests

```bash
cd frontend

# Run all tests (recommended: use single worker for stability)
npx playwright test --workers=1

# Run specific browser
npx playwright test --project=chromium
npx playwright test --project=firefox
npx playwright test --project=webkit

# Run specific test file
npx playwright test e2e/goals.spec.ts
npx playwright test e2e/completions.spec.ts
npx playwright test e2e/offline-sync.spec.ts

# Run with UI mode (interactive)
npx playwright test --ui

# Run headed (see browser)
npx playwright test --headed
```

### Test Auth Setup

Tests use a shared auth state stored in `.auth/user.json`. The first run will create a test session automatically via the `/api/v1/auth/test-session` endpoint (only available on localhost).

To regenerate auth:
```bash
rm -rf .auth/
npx playwright test --project=setup
```

---

## Testing Against Production Backend

To test the frontend locally while syncing with the **production** backend:

### 1. Start frontend with production proxy

```bash
cd frontend
npm run dev -- --config vite.config.prod-test.ts
```

This proxies `/api/*` requests to `https://goal-tracker-app.fly.dev`.

### 2. Open the app

Navigate to http://localhost:5173

### 3. Authenticate

Click "Sign in with Google" - this will authenticate against the production backend.

### 4. Test sync

- Create/edit/delete goals
- Toggle completions
- Check browser console (F12) for sync errors

### Notes

- The test session endpoint (`/auth/test-session`) is NOT available on production
- You must use real Google OAuth to authenticate
- Sync errors will show detailed status codes and response bodies in the console

---

## Troubleshooting

### Port 8080 already in use

```bash
# Find and kill process on port 8080
lsof -ti :8080 | xargs -r kill -9
```

### WebKit tests fail with missing dependencies

```bash
sudo npx playwright install-deps webkit
```

### Tests are flaky when run in parallel

Use single worker mode:
```bash
npx playwright test --workers=1
```

### Auth issues between test runs

Clear auth state and regenerate:
```bash
rm -rf .auth/
npx playwright test --project=setup
```
