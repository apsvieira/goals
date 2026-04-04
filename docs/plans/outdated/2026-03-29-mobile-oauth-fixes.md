# Mobile OAuth Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix Android OAuth flow that shows a blank screen by correcting CORS origins for Capacitor 8, replacing the 307 redirect with an HTML-based deep-link handoff, and cleaning up the manifest.

**Architecture:** Three independent backend/Android fixes. The CORS fix adds the Capacitor 8 default WebView origin (`https://localhost`) to the allowed list. The redirect fix replaces the HTTP 307 to a custom URL scheme with an HTML page that uses JavaScript to trigger the deep link (standard pattern for mobile OAuth). The manifest fix removes a misplaced `autoVerify` attribute.

**Tech Stack:** Go (backend), Android XML (manifest), `net/http/httptest` (tests)

---

### Task 1: Add `https://localhost` to CORS mobile origins

Capacitor 8 (v5+) serves WebView content via `https://localhost` by default. The current CORS middleware only allows `capacitor://localhost` and `http://localhost`, so every `fetch` from the Android app — including the post-OAuth code exchange — is blocked by CORS.

**Files:**
- Modify: `backend/internal/api/router.go:338-342`
- Test: `backend/internal/api/api_test.go` (new test)

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestCORS_CapacitorOrigins(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// All Capacitor origins that must be allowed
	origins := []string{
		"capacitor://localhost",
		"http://localhost",
		"https://localhost",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			// Preflight
			req := httptest.NewRequest("OPTIONS", "/api/v1/auth/me", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("preflight expected 200, got %d", w.Code)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("expected Allow-Origin %q, got %q", origin, got)
			}
			if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Errorf("expected Allow-Credentials true, got %q", got)
			}

			// Actual request
			req2 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
			req2.Header.Set("Origin", origin)
			w2 := httptest.NewRecorder()
			server.ServeHTTP(w2, req2)

			if got := w2.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("expected Allow-Origin %q on GET, got %q", origin, got)
			}
		})
	}
}

func TestCORS_UnknownOriginBlocked(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unknown origin should not get CORS header, got %q", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./internal/api/ -run TestCORS -v`
Expected: `TestCORS_CapacitorOrigins/https://localhost` FAILS — the origin is not in the allowlist.

**Step 3: Write the fix**

In `backend/internal/api/router.go`, change lines 338-342 from:

```go
	// Mobile app origins that are always allowed (Capacitor/Cordova apps)
	mobileOrigins := map[string]bool{
		"capacitor://localhost": true,
		"http://localhost":      true,
	}
```

to:

```go
	// Mobile app origins that are always allowed (Capacitor/Cordova apps)
	// Capacitor 5+ defaults to https://localhost as the WebView origin on Android.
	mobileOrigins := map[string]bool{
		"capacitor://localhost": true,
		"http://localhost":      true,
		"https://localhost":     true,
	}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./internal/api/ -run TestCORS -v`
Expected: All PASS.

**Step 5: Run full test suite**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./...`
Expected: All PASS, no regressions.

**Step 6: Commit**

```
fix(api): add https://localhost to CORS for Capacitor 8

Capacitor 5+ changed the default Android WebView origin from
http://localhost to https://localhost. Without this origin in the
allowlist, all fetch requests from the mobile app are blocked by CORS.
```

---

### Task 2: Replace 307 redirect with HTML deep-link page

The server currently responds to the mobile OAuth callback with a `307 Temporary Redirect` to `tinytracker://auth?code=...`. Chrome Custom Tabs on some Android versions don't cleanly handle server-side redirects to custom URL schemes — the intent fires but the tab stays open on a blank page. The standard pattern is to serve an HTML page that uses JavaScript to trigger the redirect and shows a fallback link.

**Files:**
- Modify: `backend/internal/api/auth.go:64-69`
- Test: `backend/internal/api/api_test.go` (new test)

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestMobileOAuthCallback_ReturnsHTMLRedirect(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Generate an auth code and simulate the mobile callback redirect.
	// We can't do a full OAuth flow in tests (needs real Google), so we test
	// the HTML redirect helper directly by checking the oauthCallback behavior
	// with a pre-seeded auth code.
	//
	// Instead, verify the mobile redirect helper produces correct HTML.
	code := "test-auth-code-123"
	redirectURL := "tinytracker://auth?code=" + code

	html := api.MobileRedirectHTML(redirectURL)

	// Must be HTML, not a redirect status
	if !bytes.Contains([]byte(html), []byte("<html>")) {
		t.Error("expected HTML document")
	}
	// Must contain JS redirect
	if !bytes.Contains([]byte(html), []byte("window.location.href")) {
		t.Error("expected JavaScript redirect")
	}
	// Must contain the deep link URL
	if !bytes.Contains([]byte(html), []byte(redirectURL)) {
		t.Errorf("expected HTML to contain %q", redirectURL)
	}
	// Must contain a fallback link
	if !bytes.Contains([]byte(html), []byte("</a>")) {
		t.Error("expected fallback anchor link")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./internal/api/ -run TestMobileOAuthCallback_ReturnsHTMLRedirect -v`
Expected: FAIL — `api.MobileRedirectHTML` is not defined.

**Step 3: Write the implementation**

In `backend/internal/api/auth.go`, add the helper function and update `oauthCallback`:

Add this function (e.g. above `oauthCallback`):

```go
// MobileRedirectHTML returns an HTML page that redirects to the given deep-link
// URL via JavaScript, with a fallback tap-to-continue link. This is more
// reliable than a 307 redirect to a custom URL scheme in Chrome Custom Tabs.
func MobileRedirectHTML(deepLinkURL string) string {
	// The URL is a server-generated tinytracker://auth?code=<hex> value,
	// not user input, so escaping is defence-in-depth only.
	escaped := template.HTMLEscapeString(deepLinkURL)
	return `<html><head><title>Redirecting…</title></head><body>` +
		`<script>window.location.href="` + escaped + `";</script>` +
		`<p style="font-family:sans-serif;text-align:center;margin-top:40vh">` +
		`Redirecting to app&hellip; <a href="` + escaped + `">Tap here</a> if nothing happens.</p>` +
		`</body></html>`
}
```

Add `"html/template"` to the imports at the top of `auth.go`.

Then change the mobile redirect block in `oauthCallback` (lines 64-69) from:

```go
	// Handle mobile OAuth callback - redirect with one-time auth code
	if result.IsMobile {
		code := s.authCodeStore.Generate(result.SessionToken)
		redirectURL := MobileRedirectScheme + "://auth?code=" + code
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
```

to:

```go
	// Handle mobile OAuth callback — serve HTML page that deep-links into the app.
	// Using an HTML page with JS redirect is more reliable than a 307 to a custom
	// URL scheme in Chrome Custom Tabs on Android.
	if result.IsMobile {
		code := s.authCodeStore.Generate(result.SessionToken)
		deepLink := MobileRedirectScheme + "://auth?code=" + code
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(MobileRedirectHTML(deepLink)))
		return
	}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./internal/api/ -run TestMobileOAuthCallback_ReturnsHTMLRedirect -v`
Expected: PASS.

**Step 5: Run full test suite**

Run: `cd /home/apsv/source/personal/goal-tracker/backend && go test ./...`
Expected: All PASS. Existing `TestOAuthCallback_DoesNotLeakInternalErrors` should still pass (it tests the error path, not the success path).

**Step 6: Commit**

```
fix(api): use HTML page for mobile OAuth deep-link redirect

Replace HTTP 307 redirect to tinytracker:// with an HTML page that uses
JavaScript to trigger the deep link. Chrome Custom Tabs on some Android
versions don't cleanly handle server-side redirects to custom URL
schemes, leaving a blank tab open. The HTML page also provides a
fallback tap-to-continue link.
```

---

### Task 3: Remove `autoVerify` from custom-scheme intent filter

`android:autoVerify="true"` is for App Links (HTTPS), where Android verifies domain ownership via `/.well-known/assetlinks.json`. On a custom scheme (`tinytracker://`) there is no domain to verify — the check always fails. This is harmless on most devices but can prevent deep-link handling on some OEMs.

**Files:**
- Modify: `frontend/android/app/src/main/AndroidManifest.xml:26`

**Step 1: Update the manifest**

In `frontend/android/app/src/main/AndroidManifest.xml`, change line 26 from:

```xml
            <intent-filter android:autoVerify="true">
```

to:

```xml
            <intent-filter>
```

Also update the comment on line 25 — remove "goaltracker://" since the actual scheme is "tinytracker":

```xml
            <!-- Deep link handling for tinytracker:// URL scheme -->
```

**Step 2: Verify the XML is valid**

Visually confirm the intent-filter block looks correct:

```xml
            <!-- Deep link handling for tinytracker:// URL scheme -->
            <intent-filter>
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="tinytracker" />
            </intent-filter>
```

**Step 3: Commit**

```
fix(android): remove autoVerify from custom-scheme intent filter

autoVerify is for App Links (HTTPS deep links) and requires a
/.well-known/assetlinks.json file on the domain. For custom URL schemes
it always fails verification, which can prevent deep-link handling on
some OEMs.
```

---

### Task 4: Verify on Google Cloud Console (manual)

This task cannot be automated — it requires checking the Google Cloud Console.

**Step 1:** Open [Google Cloud Console > APIs & Services > Credentials](https://console.cloud.google.com/apis/credentials).

**Step 2:** Find the OAuth 2.0 Client ID starting with `12954263815-auivk32id...`.

**Step 3:** Confirm the **Authorized redirect URIs** list includes:
```
https://goal-tracker-app.fly.dev/api/v1/auth/oauth/google/callback
```

If it's missing, add it and save.

**Step 4:** Also confirm `BASE_URL` on Fly matches by running:
```
fly ssh console -a goal-tracker-app -C "printenv BASE_URL"
```
Expected output: `https://goal-tracker-app.fly.dev`

---

### Task 5: Deploy and test on device

**Step 1:** Push changes, wait for CI to build the debug APK (or create a release tag).

**Step 2:** Install the new APK on the Android device.

**Step 3:** Test the OAuth flow end-to-end:
1. Open the app — login page should render (not blank)
2. Tap "Sign in with Google"
3. Chrome Custom Tab opens, Google login page appears (not blank)
4. Sign in with Google account
5. Should see brief "Redirecting..." HTML page, then app opens
6. App should show authenticated state (goals page)
7. Kill and reopen the app — should still be authenticated (token persisted)

**Step 4:** Check Fly logs during the test:
```
fly logs -a goal-tracker-app
```
Look for any errors in the OAuth flow (state mismatch, exchange failures, etc.).
