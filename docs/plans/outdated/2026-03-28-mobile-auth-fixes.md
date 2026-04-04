# Mobile OAuth Flow Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix four bugs that make mobile OAuth completely non-functional: URL scheme mismatch, missing mobile auth in sync.ts, AuthPage not using mobile OAuth, and deep link handler reading wrong scheme/param.

**Architecture:** Four independent fixes across the Go backend (1 file) and Svelte/TypeScript frontend (4 files). Each fix is a self-contained task that can be committed separately. The backend fix is a one-line string change. The frontend fixes replicate the existing pattern in `api.ts` (dynamic `API_BASE` + Bearer token) and wire up the existing `startMobileOAuth()` helper that was never called. Android versionCode bump is included as a final task.

**Tech Stack:** Go 1.24, TypeScript/Svelte 5, Capacitor 8, Vitest, GitHub Actions

**Findings reference:** `~/agent-notes/goal-tracker/mobile-auth-findings.md`

---

## Task 1: Fix URL scheme mismatch in backend (Critical)

**Problem:** `backend/internal/api/auth.go:62` redirects to `goaltracker://auth?code=...` but the Android manifest at `frontend/android/app/src/main/AndroidManifest.xml:30` registers `tinytracker` as the scheme. The deep link never reaches the app.

**Files:**
- Modify: `backend/internal/api/auth.go:62`
- Create: `backend/internal/api/auth_test.go`

### Step 1: Write a test that verifies the mobile redirect URL scheme

Extract the scheme to a constant in `backend/internal/api/auth.go` (cleaner and testable):

```go
// MobileRedirectScheme is the URL scheme registered in the Android manifest.
// MUST match: frontend/android/app/src/main/AndroidManifest.xml <data android:scheme="...">
// and: frontend/capacitor.config.ts plugins.App.urlScheme
const MobileRedirectScheme = "tinytracker"
```

Create `backend/internal/api/auth_test.go`:

```go
package api

import (
	"testing"
)

func TestMobileRedirectScheme(t *testing.T) {
	// This scheme MUST match the Android manifest and Capacitor config.
	if MobileRedirectScheme != "tinytracker" {
		t.Errorf("MobileRedirectScheme = %q, want %q", MobileRedirectScheme, "tinytracker")
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd backend && go test -v -run TestMobileRedirectScheme ./internal/api/`
Expected: FAIL — `MobileRedirectScheme` does not exist yet

### Step 3: Apply the fix

In `backend/internal/api/auth.go`, add the constant at package level and update line 62:

Before:
```go
redirectURL := "goaltracker://auth?code=" + code
```

After:
```go
redirectURL := MobileRedirectScheme + "://auth?code=" + code
```

### Step 4: Run tests

Run: `cd backend && go test -v ./...`
Expected: All PASS

### Step 5: Commit

```bash
git add backend/internal/api/auth.go backend/internal/api/auth_test.go
git commit -m "fix(auth): use tinytracker:// scheme for mobile OAuth redirect

The backend was redirecting to goaltracker://auth but the Android manifest
registers tinytracker as the deep link scheme. Extract to a named constant
to prevent future drift."
```

---

## Task 2: Add mobile auth support to sync.ts (High)

**Problem:** `frontend/src/lib/sync.ts:28` hardcodes `const API_BASE = '/api/v1'` (relative URL) and the fetch call at line 240-246 has no `Authorization: Bearer` header. On native platforms, the relative URL resolves to the Capacitor local web server (not the production backend), and there is no auth token.

**Reference pattern:** `frontend/src/lib/api.ts:21-31` shows exactly how to do this:
- `PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev'`
- `getApiBase()` checks `Capacitor.isNativePlatform()` and returns production URL on native
- `request()` calls `getToken()` and adds `Authorization: Bearer` header when a token exists

**Files:**
- Modify: `frontend/src/lib/sync.ts:1-2,27-28,240-246`

### Step 1: Add imports

At top of `frontend/src/lib/sync.ts`, add after existing imports:

```typescript
import { Capacitor } from '@capacitor/core';
import { getToken } from './token-storage';
```

### Step 2: Replace the API_BASE constant

Change from:
```typescript
const API_BASE = '/api/v1';
```

To:
```typescript
const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

function getApiBase(): string {
  if (Capacitor.isNativePlatform()) {
    return `${PRODUCTION_API_URL}/api/v1`;
  }
  return '/api/v1';
}

const API_BASE = getApiBase();
```

### Step 3: Add Authorization header to the fetch call

In the `sync()` method, replace the fetch call (around line 240-246):

Before:
```typescript
      const res = await fetch(`${API_BASE}/sync/`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(req),
      });
```

After:
```typescript
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      const token = await getToken();
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const res = await fetch(`${API_BASE}/sync/`, {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify(req),
      });
```

### Step 4: Run frontend tests

Run: `cd frontend && npm run check && npx vitest run`
Expected: All PASS

### Step 5: Commit

```bash
git add frontend/src/lib/sync.ts
git commit -m "fix(sync): add mobile auth support to sync.ts

sync.ts hardcoded a relative API_BASE and included no Authorization
header. On native platforms the relative URL resolved to the Capacitor
local web server. Replicate the pattern from api.ts: dynamic API_BASE
+ Bearer token from token-storage."
```

---

## Task 3: Wire up mobile OAuth in AuthPage.svelte and App.svelte (High)

**Problem:** `AuthPage.svelte` does `window.location.href = '/api/v1/auth/oauth/google'` which navigates within the Capacitor WebView to its local server. The existing `startMobileOAuth()` in `mobile-auth.ts` correctly opens the production URL but is never called. Same issue in `App.svelte:441-443` (`handleSignIn`).

**Files:**
- Modify: `frontend/src/lib/components/AuthPage.svelte`
- Modify: `frontend/src/App.svelte`

### Step 1: Fix AuthPage.svelte

Add imports in the `<script>` block:

```typescript
import { Capacitor } from '@capacitor/core';
import { startMobileOAuth } from '../mobile-auth';
```

Replace `handleGoogleLogin`:

```typescript
function handleGoogleLogin() {
  if (Capacitor.isNativePlatform()) {
    startMobileOAuth();
    return;
  }
  window.location.href = '/api/v1/auth/oauth/google';
}
```

Also fix the `onMount` auth config fetch which uses a relative URL that fails on native:

```typescript
const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

function getConfigUrl(): string {
  if (Capacitor.isNativePlatform()) {
    return `${PRODUCTION_API_URL}/api/v1/auth/config`;
  }
  return '/api/v1/auth/config';
}

onMount(async () => {
  try {
    const res = await fetch(getConfigUrl());
    if (res.ok) {
      const config = await res.json();
      devLoginEnabled = config.devLogin;
    }
  } catch {
    // Config endpoint unavailable — no dev login
  }
});
```

### Step 2: Fix App.svelte handleSignIn

Add import (around line 5, `Capacitor` is already imported):

```typescript
import { startMobileOAuth } from './lib/mobile-auth';
```

Replace `handleSignIn` (around line 441-443):

```typescript
function handleSignIn() {
  if (Capacitor.isNativePlatform()) {
    startMobileOAuth();
    return;
  }
  window.location.href = '/api/v1/auth/oauth/google';
}
```

### Step 3: Run checks

Run: `cd frontend && npm run check && npx vitest run`
Expected: All PASS

### Step 4: Commit

```bash
git add frontend/src/lib/components/AuthPage.svelte frontend/src/App.svelte
git commit -m "fix(auth): use startMobileOAuth() on native platforms for login

AuthPage.svelte and App.svelte handleSignIn used relative URLs for
OAuth which resolve to the Capacitor local web server on Android.
Now check Capacitor.isNativePlatform() and call the existing
startMobileOAuth() helper."
```

---

## Task 4: Fix deep link handler — wrong scheme, wrong param, missing exchange call (Medium)

**Problem:** `frontend/src/App.svelte:588-600` checks for `goaltracker://auth` (wrong scheme), reads `token` param (old API), and calls `saveToken()` directly instead of exchanging the code via `POST /api/v1/auth/exchange`.

**Files:**
- Modify: `frontend/src/App.svelte:588-600`

### Step 1: Replace the deep link handler

Replace the `appUrlOpen` listener code. Change:

```typescript
    if (url.startsWith('goaltracker://auth')) {
      try {
        const urlObj = new URL(url);
        const token = urlObj.searchParams.get('token');
        if (token) {
          await saveToken(token);
          await checkAuth();
        }
```

With:

```typescript
    if (url.startsWith('tinytracker://auth')) {
      try {
        const urlObj = new URL(url);
        const code = urlObj.searchParams.get('code');
        if (code) {
          // Exchange the one-time auth code for a session token
          const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';
          const res = await fetch(`${PRODUCTION_API_URL}/api/v1/auth/exchange`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code }),
          });
          if (!res.ok) {
            throw new Error(`Auth code exchange failed (${res.status})`);
          }
          const data = await res.json();
          await saveToken(data.session_token);
          // Close the in-app browser opened by startMobileOAuth()
          try {
            const { Browser } = await import('@capacitor/browser');
            await Browser.close();
          } catch {
            // Browser plugin may not be available or already closed
          }
          await checkAuth();
        }
```

### Step 2: Run checks

Run: `cd frontend && npm run check && npx vitest run`
Expected: All PASS

### Step 3: Commit

```bash
git add frontend/src/App.svelte
git commit -m "fix(auth): deep link handler uses correct scheme, reads code, exchanges for token

The appUrlOpen handler checked for goaltracker:// (wrong scheme) and read
a 'token' param (old API). Now checks tinytracker://, reads 'code' param,
calls POST /api/v1/auth/exchange to get a session token, and closes the
in-app browser."
```

---

## Task 5: Bump Android versionCode for Play Store

**Files:**
- Modify: `frontend/android/app/build.gradle:17-18`

### Step 1: Bump version

Change `versionCode 2` to `versionCode 3` and `versionName "1.0.1"` to `versionName "1.0.2"`.

### Step 2: Commit

```bash
git add frontend/android/app/build.gradle
git commit -m "chore(android): bump versionCode to 3 for mobile auth fix release"
```

---

## Verification

### Automated tests

**Backend:**
```bash
cd backend && go test -v ./...
```
Key tests:
- `TestMobileRedirectScheme` — confirms scheme constant is `tinytracker`
- `TestAuthCodeExchange` — confirms exchange endpoint works
- `TestAuthCodeStore_StoreAndExchange` — confirms one-time codes work

**Frontend:**
```bash
cd frontend && npm run check && npx vitest run
```
Key tests:
- All existing sync tests still pass
- TypeScript compilation succeeds (catches import/type errors in the UI wiring)

### Manual end-to-end verification on device

Requires a debug APK from GitHub Actions CI.

1. **Install debug APK** from the workflow artifact
2. **Fresh sign-in:** Open app > tap "Sign in with Google" > verify in-app browser opens to `https://goal-tracker-app.fly.dev/api/v1/auth/oauth/google?mobile=true` > complete OAuth > verify browser closes and app shows authenticated home screen
3. **Sync works:** Create a goal > wait 2 min or background/foreground > verify goal appears in web app
4. **Deep link scheme:** Run `adb shell am start -a android.intent.action.VIEW -d "tinytracker://auth?code=test" software.maleficent.tinytracker` > verify app opens (code exchange will fail gracefully since "test" is not a real code)
5. **Logout and re-login:** Profile > logout > sign in again > verify OAuth works a second time
6. **Offline resilience:** Airplane mode on > create goal > airplane mode off > verify goal syncs

### GitHub Actions CI verification

1. Push to main (or trigger `android-build.yml` manually on the branch)
2. Verify build job succeeds: `npm ci` > `npm run build` > `npx cap sync android` > `./gradlew assembleDebug`
3. Download `debug-apk` artifact and install: `adb install app-debug.apk`
4. Verify version: `adb shell dumpsys package software.maleficent.tinytracker | grep versionCode` — expect `versionCode=3`

### Deployment coordination

All changes must deploy together:
- Backend deploys via Fly.io on push to main (~1-2 min)
- Android build triggers on push to main for `frontend/**` paths (~5+ min)
- During the window between backend deploy and app update, old app versions will fail auth — acceptable for pre-launch

---

## Task Dependency Graph

```
Task 1 (backend scheme) ─────┐
Task 2 (sync.ts mobile) ─────┤
Task 3 (AuthPage + App) ─────┼──→ Task 5 (version bump) ──→ Verification
Task 4 (deep link handler) ──┘
```

Tasks 1-4 are independent. Task 5 (version bump) should be last.
