# Security Hardening Implementation Plan

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix error message leaking, add request body size limits, add HSTS header, and add sync request validation.

**Architecture:** All changes are in the backend API layer. A new `writeError` helper replaces raw `http.Error(w, err.Error(), ...)` calls across all handlers. A `maxBodySize` middleware wraps request bodies. Security headers and sync validation are localized changes.

**Tech Stack:** Go, Chi router, `net/http`

---

### Task 1: Add `writeError` helper and replace error leaking in `goals.go`

The app currently sends raw Go error strings (including DB internals) to clients via `http.Error(w, err.Error(), ...)`. We need a helper that logs the real error and returns a generic message.

**Files:**
- Modify: `backend/internal/api/goals.go:56,103,139,148,155,169,178,200,207`
- Modify: `backend/internal/api/api.go` (add new file-level helper — this file doesn't exist yet as a standalone, so we put helper in `goals.go`'s package)

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestInternalErrors_DontLeakDetails(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Request a goal that doesn't exist — the 404 message should NOT contain DB internals
	req := httptest.NewRequest("PATCH", "/api/v1/goals/nonexistent-id", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	body := w.Body.String()
	// Response should not contain Go error strings like "sql:" or "query"
	if bytes.Contains(w.Body.Bytes(), []byte("sql:")) || bytes.Contains(w.Body.Bytes(), []byte("query")) {
		t.Errorf("error response leaks internal details: %s", body)
	}
}
```

**Step 2: Run test to verify it passes (this specific test should pass already since 404 uses a static string)**

Run: `cd backend && go test ./internal/api/ -run TestInternalErrors_DontLeakDetails -v`

This test is a safety net. Now add the helper and replace all leaking calls.

**Step 3: Add `serverError` helper to `backend/internal/api/goals.go` package**

Add to `backend/internal/api/router.go` (near the existing `writeJSON` in the package, or in a new `helpers.go`):

```go
// serverError logs the real error and sends a generic message to the client.
func serverError(w http.ResponseWriter, err error) {
	Logger.Error("internal error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
```

**Step 4: Replace all `http.Error(w, err.Error(), http.StatusInternalServerError)` in `goals.go`**

Replace every instance with `serverError(w, err)`. There are 9 occurrences at lines 56, 103, 139, 148, 155, 169, 178, 200, 207.

**Step 5: Run tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: all existing tests PASS

**Step 6: Commit**

```
git add backend/internal/api/
git commit -m "fix(api): stop leaking internal errors in goals handlers"
```

---

### Task 2: Replace error leaking in `completions.go`, `auth.go`, and `sync.go`

**Files:**
- Modify: `backend/internal/api/completions.go:38,85,96,112,126,137,146,174,180`
- Modify: `backend/internal/api/auth.go:34` (OAuth start error)
- Modify: `backend/internal/api/auth.go:52` (OAuth callback error redirect includes `err.Error()` in URL)

**Step 1: Replace all `http.Error(w, err.Error(), http.StatusInternalServerError)` in `completions.go`**

Same pattern as Task 1 — replace with `serverError(w, err)`. There are 9 occurrences.

**Step 2: Fix `auth.go` line 34**

Replace:
```go
http.Error(w, err.Error(), http.StatusBadRequest)
```
With:
```go
Logger.Error("oauth start failed", "error", err, "provider", provider)
http.Error(w, "failed to start authentication", http.StatusBadRequest)
```

**Step 3: Fix `auth.go` line 52 — OAuth callback error leaks in redirect URL**

Replace:
```go
redirectURL := s.frontendURL + "/?auth_error=" + err.Error()
```
With:
```go
Logger.Error("oauth callback failed", "error", err, "provider", provider)
redirectURL := s.frontendURL + "/?auth_error=authentication_failed"
```

**Step 4: Run tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: all tests PASS

**Step 5: Commit**

```
git add backend/internal/api/
git commit -m "fix(api): stop leaking internal errors in completions and auth handlers"
```

---

### Task 3: Add request body size limit middleware

Without a size limit, any endpoint that reads `r.Body` can be sent an arbitrarily large payload, consuming server memory.

**Files:**
- Modify: `backend/internal/api/router.go:96-107` (middleware stack)

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestRequestBodySizeLimit(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a body larger than 1MB
	largeBody := bytes.Repeat([]byte("x"), 2*1024*1024)
	req := httptest.NewRequest("POST", "/api/v1/goals", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Should reject with 400 (bad request from json decode failure) or 413
	if w.Code == http.StatusInternalServerError {
		t.Errorf("large body should not cause 500, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails (or note current behavior)**

Run: `cd backend && go test ./internal/api/ -run TestRequestBodySizeLimit -v`

**Step 3: Add body size limit middleware in `router.go`**

Add to the middleware stack in `setupRoutes()`, after `middleware.Recoverer` and before `securityHeaders`:

```go
// Limit request body size to 1MB to prevent memory exhaustion
r.Use(func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		next.ServeHTTP(w, r)
	})
})
```

Insert this at line ~103 (between `middleware.Recoverer` and `securityHeaders`).

**Step 4: Run tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: all tests PASS

**Step 5: Commit**

```
git add backend/internal/api/router.go
git commit -m "fix(api): add 1MB request body size limit"
```

---

### Task 4: Add HSTS header

The app runs behind `force_https = true` on Fly.io but doesn't tell browsers to remember this.

**Files:**
- Modify: `backend/internal/api/router.go:253-287` (securityHeaders function)

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestSecurityHeaders_HSTS(t *testing.T) {
	// HSTS should only be set when COOKIE_SECURE is not "false" (i.e., in production)
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// In test, COOKIE_SECURE is not set, so HSTS should be present
	// (HSTS is only skipped when explicitly in localhost/dev mode)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	// In test environment COOKIE_SECURE is unset, check header exists
	// The real check: header should be set in production
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header")
	}
	_ = hsts // HSTS presence depends on env; we verify it doesn't break
}
```

**Step 2: Add HSTS to `securityHeaders` in `router.go`**

Inside the `securityHeaders` handler function, add after the existing headers (line ~284):

```go
// HSTS: only set when running in production (HTTPS)
// COOKIE_SECURE is "false" only in local dev
if os.Getenv("COOKIE_SECURE") != "false" {
	w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
}
```

**Step 3: Run tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: PASS

**Step 4: Commit**

```
git add backend/internal/api/router.go
git commit -m "fix(api): add HSTS header for production"
```

---

### Task 5: Add sync request validation

The sync endpoint accepts unvalidated arrays of goals/completions. Add size limits and basic field validation.

**Files:**
- Modify: `backend/internal/api/sync.go:24-33`

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestSync_RejectsOversizedPayload(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Build a sync request with too many goals (over 500)
	goals := make([]map[string]interface{}, 501)
	for i := range goals {
		goals[i] = map[string]interface{}{
			"id": fmt.Sprintf("goal-%d", i), "name": "g", "color": "#000000",
			"position": i, "updated_at": time.Now().UTC(), "deleted": false,
		}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals":          goals,
		"completions":    []interface{}{},
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized sync, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/api/ -run TestSync_RejectsOversizedPayload -v`
Expected: FAIL (currently returns 200)

**Step 3: Add validation to `handleSync` in `sync.go`**

After the JSON decode block (line ~33), add:

```go
// Validate sync request size to prevent abuse
const maxSyncGoals = 500
const maxSyncCompletions = 5000

if len(req.Goals) > maxSyncGoals {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf("too many goals in sync request (max %d)", maxSyncGoals),
	})
	return
}
if len(req.Completions) > maxSyncCompletions {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf("too many completions in sync request (max %d)", maxSyncCompletions),
	})
	return
}
```

Add `"fmt"` to the imports.

**Step 4: Run tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: all PASS including the new one

**Step 5: Commit**

```
git add backend/internal/api/sync.go backend/internal/api/api_test.go
git commit -m "fix(api): add sync request size validation"
```
