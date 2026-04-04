# Dev Login Env Var Implementation Plan

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the implicit localhost-detection dev login with an explicit `DEV_LOGIN=true` env var, and expose it to the frontend via a config endpoint so the AuthPage can show a "Dev Login" button.

**Architecture:** The backend gates the `/api/v1/auth/dev/login` endpoint behind `DEV_LOGIN=true`. A new unauthenticated `GET /api/v1/auth/config` endpoint returns `{ devLogin: bool }`. The frontend fetches this on the AuthPage and conditionally renders a dev login button that POSTs to the dev login endpoint, then refreshes auth state.

**Tech Stack:** Go (chi router), Svelte 5, TypeScript

---

## Tasks

### Task 1: Backend — replace localhost heuristic with `DEV_LOGIN` env var

**Files:**
- Modify: `backend/internal/api/router.go:122-128`

**Step 1: Change the dev login gate**

In `backend/internal/api/router.go`, replace the localhost-detection block (lines 122-128):

```go
			// Dev login - only enabled in development (localhost or unset BASE_URL)
			frontendURL := os.Getenv("FRONTEND_URL")
			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" || strings.Contains(baseURL, "localhost") || strings.Contains(frontendURL, "localhost") {
				r.Post("/dev/login", s.devLogin)
			}
```

With:

```go
			// Dev login - only enabled when DEV_LOGIN=true
			if os.Getenv("DEV_LOGIN") == "true" {
				r.Post("/dev/login", s.devLogin)
			}
```

**Step 2: Add the auth config endpoint**

In the same auth route group, after the dev login block, add:

```go
			// Auth config - tells the frontend what auth methods are available
			r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"devLogin": os.Getenv("DEV_LOGIN") == "true",
				})
			})
```

**Step 3: Run existing backend tests with DEV_LOGIN=true**

The existing tests use `authenticateTestUser` which calls `/api/v1/auth/dev/login`. Tests need `DEV_LOGIN=true` set.

```bash
cd backend && DEV_LOGIN=true go test ./... -v 2>&1 | tail -20
```

Expected: All tests pass. If they fail because `DEV_LOGIN` is not set in test env, we need to set it in the test helper.

**Step 4: Set DEV_LOGIN in test setup**

In `backend/internal/api/api_test.go`, add to `setupTestServer()` before `api.NewServer()`:

```go
	t.Setenv("DEV_LOGIN", "true")
```

This uses `t.Setenv` which automatically restores the original value after the test.

**Step 5: Run tests again**

```bash
cd backend && go test ./... -v 2>&1 | tail -20
```

Expected: All tests pass.

**Step 6: Commit**

```bash
git add backend/internal/api/router.go backend/internal/api/api_test.go
git commit -m "feat(backend): gate dev login behind DEV_LOGIN=true env var

Replace implicit localhost detection with explicit DEV_LOGIN=true.
Add GET /api/v1/auth/config endpoint returning { devLogin: bool }.
Set DEV_LOGIN=true in test setup so existing tests continue to work."
```

---

### Task 2: Frontend — add dev login button to AuthPage

**Files:**
- Modify: `frontend/src/lib/components/AuthPage.svelte`

**Step 1: Add dev login logic to AuthPage**

In `frontend/src/lib/components/AuthPage.svelte`, replace the entire `<script>` block:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';

  let devLoginEnabled = false;

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/auth/config');
      if (res.ok) {
        const config = await res.json();
        devLoginEnabled = config.devLogin;
      }
    } catch {
      // Config endpoint unavailable — no dev login
    }
  });

  function handleGoogleLogin() {
    window.location.href = '/api/v1/auth/oauth/google';
  }

  async function handleDevLogin() {
    try {
      const res = await fetch('/api/v1/auth/dev/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'dev@localhost' }),
      });
      if (res.ok) {
        window.location.reload();
      }
    } catch (e) {
      console.error('Dev login failed:', e);
    }
  }

  function handleLinkClick(e: MouseEvent, path: string) {
    e.preventDefault();
    window.history.pushState({}, '', path);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }
</script>
```

**Step 2: Add the dev login button to the template**

After the Google sign-in button inside `.auth-buttons`, add:

```svelte
      {#if devLoginEnabled}
        <button class="auth-btn dev-btn" on:click={handleDevLogin}>
          Dev Login
        </button>
      {/if}
```

**Step 3: Add CSS for the dev button**

Add inside the `<style>` block, after `.google-btn:hover`:

```css
  .dev-btn {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    font-size: 0.875rem;
  }

  .dev-btn:hover {
    background: var(--border);
  }
```

**Step 4: Run frontend type check**

```bash
cd frontend && npx svelte-check 2>&1 | grep -v node_modules
```

Expected: 0 errors in our code (only the 2 `esrap` dependency errors).

**Step 5: Commit**

```bash
git add frontend/src/lib/components/AuthPage.svelte
git commit -m "feat(frontend): add dev login button to AuthPage

Fetch /api/v1/auth/config on mount to check if dev login is enabled.
When DEV_LOGIN=true on the backend, show a 'Dev Login' button that
POSTs to /api/v1/auth/dev/login and reloads the page."
```

---

### Task 3: Verification

**Step 1: Run full backend tests**

```bash
cd backend && go test ./...
```

Expected: All pass.

**Step 2: Run full frontend checks**

```bash
cd frontend && npm run check && npx vitest run
```

Expected: Type check passes, all unit tests pass.

**Step 3: Manual smoke test**

Restart the backend with `DEV_LOGIN=true`:

```bash
# Kill existing backend
kill $(ss -tlnp | grep 8080 | grep -oP 'pid=\K[0-9]+')

# Start with DEV_LOGIN=true
cd backend && DEV_LOGIN=true go run ./cmd/server/ &
```

Then open `http://localhost:5174/` — the AuthPage should show both the Google button and the "Dev Login" button. Clicking "Dev Login" should log you in.

**Step 4: Verify dev login is hidden without env var**

```bash
# Kill backend
kill $(ss -tlnp | grep 8080 | grep -oP 'pid=\K[0-9]+')

# Start without DEV_LOGIN
cd backend && go run ./cmd/server/ &
```

Open `http://localhost:5174/` — only the Google button should appear.
