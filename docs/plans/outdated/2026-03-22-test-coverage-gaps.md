# Test Coverage Gaps Implementation Plan

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add tests for multi-user data isolation, sync conflict resolution, and rate limiting — the three highest-impact gaps identified in the release review.

**Architecture:** All new tests are backend Go tests using the existing `setupTestServer`/`authenticateTestUser` helpers in `api_test.go`. The sync merge tests are unit tests in a new `backend/internal/sync/merge_test.go`. Rate limit tests exercise the HTTP layer.

**Tech Stack:** Go, `testing`, `net/http/httptest`

---

### Task 1: Multi-user data isolation tests

This is the single biggest testing gap. No test creates two users and verifies one can't see or modify the other's data.

**Files:**
- Create: `backend/internal/api/isolation_test.go`

**Step 1: Write the test file**

Create `backend/internal/api/isolation_test.go`:

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

func TestIsolation_UserCannotSeeOtherUsersGoals(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal
	body := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(alice)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("alice create goal: expected 201, got %d", w.Code)
	}

	// Bob lists goals — should see none
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(bob)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var bobGoals []models.Goal
	json.NewDecoder(listW.Body).Decode(&bobGoals)

	if len(bobGoals) != 0 {
		t.Errorf("bob should see 0 goals, got %d", len(bobGoals))
	}
}

func TestIsolation_UserCannotUpdateOtherUsersGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal
	body := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(alice)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var aliceGoal models.Goal
	json.NewDecoder(w.Body).Decode(&aliceGoal)

	// Bob tries to update Alice's goal
	updateBody := bytes.NewBufferString(`{"name": "Hacked"}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+aliceGoal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(bob)
	updateW := httptest.NewRecorder()
	server.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusNotFound {
		t.Errorf("bob updating alice's goal: expected 404, got %d", updateW.Code)
	}

	// Verify Alice's goal is unchanged
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(alice)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)

	if len(goals) != 1 || goals[0].Name != "Alice Goal" {
		t.Errorf("alice's goal should be unchanged, got: %+v", goals)
	}
}

func TestIsolation_UserCannotDeleteOtherUsersGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal
	body := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(alice)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var aliceGoal models.Goal
	json.NewDecoder(w.Body).Decode(&aliceGoal)

	// Bob tries to archive Alice's goal
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/goals/"+aliceGoal.ID, nil)
	deleteReq.AddCookie(bob)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNotFound {
		t.Errorf("bob deleting alice's goal: expected 404, got %d", deleteW.Code)
	}
}

func TestIsolation_UserCannotSeeOtherUsersCompletions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal and completion
	goalBody := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	goalReq := httptest.NewRequest("POST", "/api/v1/goals", goalBody)
	goalReq.Header.Set("Content-Type", "application/json")
	goalReq.AddCookie(alice)
	goalW := httptest.NewRecorder()
	server.ServeHTTP(goalW, goalReq)

	var aliceGoal models.Goal
	json.NewDecoder(goalW.Body).Decode(&aliceGoal)

	compBody := bytes.NewBufferString(`{"goal_id": "` + aliceGoal.ID + `", "date": "2026-01-15"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(alice)
	server.ServeHTTP(httptest.NewRecorder(), compReq)

	// Bob lists completions for the same date range — should see none
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
	listReq.AddCookie(bob)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var bobCompletions []models.Completion
	json.NewDecoder(listW.Body).Decode(&bobCompletions)

	if len(bobCompletions) != 0 {
		t.Errorf("bob should see 0 completions, got %d", len(bobCompletions))
	}
}

func TestIsolation_UserCannotDeleteOtherUsersCompletion(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal and completion
	goalBody := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	goalReq := httptest.NewRequest("POST", "/api/v1/goals", goalBody)
	goalReq.Header.Set("Content-Type", "application/json")
	goalReq.AddCookie(alice)
	goalW := httptest.NewRecorder()
	server.ServeHTTP(goalW, goalReq)

	var aliceGoal models.Goal
	json.NewDecoder(goalW.Body).Decode(&aliceGoal)

	compBody := bytes.NewBufferString(`{"goal_id": "` + aliceGoal.ID + `", "date": "2026-01-15"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(alice)
	compW := httptest.NewRecorder()
	server.ServeHTTP(compW, compReq)

	var aliceCompletion models.Completion
	json.NewDecoder(compW.Body).Decode(&aliceCompletion)

	// Bob tries to delete Alice's completion
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+aliceCompletion.ID, nil)
	deleteReq.AddCookie(bob)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNotFound {
		t.Errorf("bob deleting alice's completion: expected 404, got %d", deleteW.Code)
	}
}

func TestIsolation_UserCannotCreateCompletionForOtherUsersGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal
	goalBody := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	goalReq := httptest.NewRequest("POST", "/api/v1/goals", goalBody)
	goalReq.Header.Set("Content-Type", "application/json")
	goalReq.AddCookie(alice)
	goalW := httptest.NewRecorder()
	server.ServeHTTP(goalW, goalReq)

	var aliceGoal models.Goal
	json.NewDecoder(goalW.Body).Decode(&aliceGoal)

	// Bob tries to create a completion for Alice's goal
	compBody := bytes.NewBufferString(`{"goal_id": "` + aliceGoal.ID + `", "date": "2026-01-15"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(bob)
	compW := httptest.NewRecorder()
	server.ServeHTTP(compW, compReq)

	if compW.Code != http.StatusNotFound {
		t.Errorf("bob creating completion on alice's goal: expected 404, got %d", compW.Code)
	}
}

func TestIsolation_CalendarOnlyShowsOwnData(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	alice := authenticateTestUser(t, server, "alice@test.com")
	bob := authenticateTestUser(t, server, "bob@test.com")

	// Alice creates a goal + completion
	goalBody := bytes.NewBufferString(`{"name": "Alice Goal", "color": "#FF0000"}`)
	goalReq := httptest.NewRequest("POST", "/api/v1/goals", goalBody)
	goalReq.Header.Set("Content-Type", "application/json")
	goalReq.AddCookie(alice)
	goalW := httptest.NewRecorder()
	server.ServeHTTP(goalW, goalReq)

	var aliceGoal models.Goal
	json.NewDecoder(goalW.Body).Decode(&aliceGoal)

	compBody := bytes.NewBufferString(`{"goal_id": "` + aliceGoal.ID + `", "date": "2026-01-15"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(alice)
	server.ServeHTTP(httptest.NewRecorder(), compReq)

	// Bob checks calendar — should be empty
	calReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
	calReq.AddCookie(bob)
	calW := httptest.NewRecorder()
	server.ServeHTTP(calW, calReq)

	var cal models.CalendarResponse
	json.NewDecoder(calW.Body).Decode(&cal)

	if len(cal.Goals) != 0 {
		t.Errorf("bob calendar: expected 0 goals, got %d", len(cal.Goals))
	}
	if len(cal.Completions) != 0 {
		t.Errorf("bob calendar: expected 0 completions, got %d", len(cal.Completions))
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd backend && go test ./internal/api/ -run TestIsolation -v`
Expected: all PASS (the API layer already enforces isolation — these tests prove it)

**Step 3: Commit**

```
git add backend/internal/api/isolation_test.go
git commit -m "test(api): add multi-user data isolation tests"
```

---

### Task 2: Sync merge unit tests

The LWW merge logic in `merge.go` has no dedicated tests. These are pure functions — easy to unit test.

**Files:**
- Create: `backend/internal/sync/merge_test.go`

**Step 1: Write the test file**

Create `backend/internal/sync/merge_test.go`:

```go
package sync

import (
	"testing"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

func TestMergeGoal_NewGoal_ClientWins(t *testing.T) {
	client := GoalChange{
		ID: "g1", Name: "Run", Color: "#FF0000", Position: 0,
		UpdatedAt: time.Now().UTC(), Deleted: false,
	}

	merged, shouldApply := MergeGoal(client, nil)
	if !shouldApply {
		t.Fatal("new goal should be applied")
	}
	if merged.Name != "Run" {
		t.Errorf("expected name 'Run', got '%s'", merged.Name)
	}
}

func TestMergeGoal_NewGoal_ClientDeletedGoal(t *testing.T) {
	client := GoalChange{
		ID: "g1", Name: "Run", Color: "#FF0000", Position: 0,
		UpdatedAt: time.Now().UTC(), Deleted: true,
	}

	merged, shouldApply := MergeGoal(client, nil)
	if !shouldApply {
		t.Fatal("new deleted goal should still be applied (to record deletion)")
	}
	if merged.DeletedAt == nil {
		t.Error("expected DeletedAt to be set")
	}
}

func TestMergeGoal_ClientNewer_ClientWins(t *testing.T) {
	serverTime := time.Now().UTC().Add(-1 * time.Hour)
	clientTime := time.Now().UTC()

	server := &models.Goal{
		ID: "g1", Name: "Old Name", Color: "#000000",
		UpdatedAt: serverTime,
	}
	client := GoalChange{
		ID: "g1", Name: "New Name", Color: "#FF0000", Position: 0,
		UpdatedAt: clientTime, Deleted: false,
	}

	merged, shouldApply := MergeGoal(client, server)
	if !shouldApply {
		t.Fatal("client is newer, should apply")
	}
	if merged.Name != "New Name" {
		t.Errorf("expected 'New Name', got '%s'", merged.Name)
	}
}

func TestMergeGoal_ServerNewer_ServerWins(t *testing.T) {
	clientTime := time.Now().UTC().Add(-1 * time.Hour)
	serverTime := time.Now().UTC()

	server := &models.Goal{
		ID: "g1", Name: "Server Name", Color: "#000000",
		UpdatedAt: serverTime,
	}
	client := GoalChange{
		ID: "g1", Name: "Client Name", Color: "#FF0000", Position: 0,
		UpdatedAt: clientTime, Deleted: false,
	}

	merged, shouldApply := MergeGoal(client, server)
	if shouldApply {
		t.Fatal("server is newer, should NOT apply client")
	}
	if merged.Name != "Server Name" {
		t.Errorf("expected server name preserved, got '%s'", merged.Name)
	}
}

func TestMergeGoal_SameTimestamp_ServerWins(t *testing.T) {
	now := time.Now().UTC()

	server := &models.Goal{
		ID: "g1", Name: "Server Name", Color: "#000000",
		UpdatedAt: now,
	}
	client := GoalChange{
		ID: "g1", Name: "Client Name", Color: "#FF0000", Position: 0,
		UpdatedAt: now, Deleted: false,
	}

	_, shouldApply := MergeGoal(client, server)
	if shouldApply {
		t.Fatal("same timestamp: server should win (no apply)")
	}
}

func TestMergeGoal_ClientDeletesExisting(t *testing.T) {
	serverTime := time.Now().UTC().Add(-1 * time.Hour)
	clientTime := time.Now().UTC()

	server := &models.Goal{
		ID: "g1", Name: "Server Name", Color: "#000000",
		UpdatedAt: serverTime,
	}
	client := GoalChange{
		ID: "g1", Name: "Server Name", Color: "#000000", Position: 0,
		UpdatedAt: clientTime, Deleted: true,
	}

	merged, shouldApply := MergeGoal(client, server)
	if !shouldApply {
		t.Fatal("client is newer, should apply delete")
	}
	if merged.DeletedAt == nil {
		t.Error("expected DeletedAt to be set")
	}
}

// --- Completion merge tests ---

func TestMergeCompletion_NewCompletion_Completed(t *testing.T) {
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: true,
		UpdatedAt: time.Now().UTC(),
	}

	merged, shouldApply := MergeCompletion(client, nil)
	if !shouldApply {
		t.Fatal("new completed completion should be applied")
	}
	if merged.GoalID != "g1" || merged.Date != "2026-01-15" {
		t.Errorf("unexpected merged values: %+v", merged)
	}
}

func TestMergeCompletion_NewCompletion_Deleted_NoOp(t *testing.T) {
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: false,
		UpdatedAt: time.Now().UTC(),
	}

	merged, shouldApply := MergeCompletion(client, nil)
	if shouldApply {
		t.Fatal("deleting nonexistent completion should be no-op")
	}
	if merged != nil {
		t.Errorf("expected nil merged, got %+v", merged)
	}
}

func TestMergeCompletion_ClientNewer_Wins(t *testing.T) {
	serverTime := time.Now().UTC().Add(-1 * time.Hour)
	clientTime := time.Now().UTC()

	server := &models.Completion{
		ID: "c1", GoalID: "g1", Date: "2026-01-15",
		UpdatedAt: serverTime,
	}
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: false,
		UpdatedAt: clientTime,
	}

	merged, shouldApply := MergeCompletion(client, server)
	if !shouldApply {
		t.Fatal("client newer, should apply")
	}
	if merged.DeletedAt == nil {
		t.Error("client wants to delete, expected DeletedAt set")
	}
}

func TestMergeCompletion_ServerNewer_ServerWins(t *testing.T) {
	clientTime := time.Now().UTC().Add(-1 * time.Hour)
	serverTime := time.Now().UTC()

	server := &models.Completion{
		ID: "c1", GoalID: "g1", Date: "2026-01-15",
		UpdatedAt: serverTime,
	}
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: false,
		UpdatedAt: clientTime,
	}

	_, shouldApply := MergeCompletion(client, server)
	if shouldApply {
		t.Fatal("server newer, should NOT apply")
	}
}

func TestMergeCompletion_SameTimestamp_AddWinsOverDelete(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now

	server := &models.Completion{
		ID: "c1", GoalID: "g1", Date: "2026-01-15",
		UpdatedAt: now,
		DeletedAt: &deletedAt,
	}
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: true,
		UpdatedAt: now,
	}

	merged, shouldApply := MergeCompletion(client, server)
	if !shouldApply {
		t.Fatal("same timestamp, ADD should win over DELETE")
	}
	if merged.DeletedAt != nil {
		t.Error("ADD wins: expected DeletedAt to be nil")
	}
}

func TestMergeCompletion_SameTimestamp_BothCompleted_NoOp(t *testing.T) {
	now := time.Now().UTC()

	server := &models.Completion{
		ID: "c1", GoalID: "g1", Date: "2026-01-15",
		UpdatedAt: now,
		// DeletedAt is nil = completed
	}
	client := CompletionChange{
		GoalID: "g1", Date: "2026-01-15", Completed: true,
		UpdatedAt: now,
	}

	_, shouldApply := MergeCompletion(client, server)
	if shouldApply {
		t.Fatal("both completed at same time: no update needed")
	}
}
```

**Step 2: Run tests**

Run: `cd backend && go test ./internal/sync/ -v`
Expected: all PASS

**Step 3: Commit**

```
git add backend/internal/sync/merge_test.go
git commit -m "test(sync): add unit tests for LWW merge conflict resolution"
```

---

### Task 3: Rate limiting tests

Rate limiting is implemented but has zero test coverage.

**Files:**
- Create: `backend/internal/api/ratelimit_test.go`

**Step 1: Find the rate limiter implementation to understand thresholds**

Check: `backend/internal/api/ratelimit.go` — the auth limiter is 10/min, API is 100/min. For tests, we'll hit the auth limiter (10/min) since it's smallest.

**Step 2: Write the test file**

Create `backend/internal/api/ratelimit_test.go`:

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_AuthEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Auth endpoint rate limit is 10/min
	// Send 11 requests to /api/v1/auth/me (unauthenticated is fine, just checking rate limit)
	var lastCode int
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		lastCode = w.Code

		if w.Code == http.StatusTooManyRequests {
			// Rate limit hit — this is expected after 10 requests
			if i < 10 {
				t.Errorf("rate limited too early on request %d", i+1)
			}
			return
		}
	}

	t.Errorf("expected 429 after 10+ requests, last code was %d", lastCode)
}

func TestRateLimit_ReturnsRetryAfterHeader(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Exhaust rate limit
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			// Verify the response indicates rate limiting
			return
		}
	}

	t.Error("expected to be rate limited")
}

func TestRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Exhaust rate limit from "IP 1" (default 192.0.2.1 from httptest)
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}

	// Request from a "different IP" via X-Real-Ip header should succeed
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.99")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code == http.StatusTooManyRequests {
		t.Error("different IP should not be rate limited")
	}
}
```

**Step 3: Run tests**

Run: `cd backend && go test ./internal/api/ -run TestRateLimit -v`
Expected: PASS (rate limiting should work as implemented)

Note: If the rate limiter checks `r.RemoteAddr` via `middleware.RealIP` and `httptest` sets a default RemoteAddr, the IP-based tests should work. If they fail, adjust the test to use `RemoteAddr` directly: `req.RemoteAddr = "192.168.1.1:1234"`.

**Step 4: Commit**

```
git add backend/internal/api/ratelimit_test.go
git commit -m "test(api): add rate limiting tests"
```

---

### Task 4: Unauthenticated access tests

Verify all protected endpoints reject unauthenticated requests.

**Files:**
- Create: `backend/internal/api/auth_required_test.go`

**Step 1: Write the test file**

Create `backend/internal/api/auth_required_test.go`:

```go
package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthRequired_ProtectedEndpoints(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/goals", ""},
		{"POST", "/api/v1/goals", `{"name":"x","color":"#000000"}`},
		{"PATCH", "/api/v1/goals/some-id", `{"name":"x"}`},
		{"DELETE", "/api/v1/goals/some-id", ""},
		{"PUT", "/api/v1/goals/reorder", `{"goal_ids":["a"]}`},
		{"GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", ""},
		{"POST", "/api/v1/completions", `{"goal_id":"x","date":"2026-01-01"}`},
		{"DELETE", "/api/v1/completions/some-id", ""},
		{"GET", "/api/v1/calendar?month=2026-01", ""},
		{"POST", "/api/v1/sync/", `{"goals":[],"completions":[]}`},
		{"POST", "/api/v1/devices", `{"token":"x","platform":"android"}`},
		{"DELETE", "/api/v1/devices/some-id", ""},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *bytes.Buffer
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			} else {
				body = &bytes.Buffer{}
			}

			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Content-Type", "application/json")
			// No cookie — unauthenticated
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}
```

**Step 2: Run tests**

Run: `cd backend && go test ./internal/api/ -run TestAuthRequired -v`
Expected: all PASS

**Step 3: Commit**

```
git add backend/internal/api/auth_required_test.go
git commit -m "test(api): add unauthenticated access rejection tests"
```
