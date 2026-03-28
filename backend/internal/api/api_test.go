package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/api"
	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

func setupTestServer(t *testing.T) (*api.Server, func()) {
	t.Helper()

	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "goal-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.NewSQLite(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		database.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Enable dev login for tests (dev login endpoint is gated behind this env var)
	t.Setenv("DEV_LOGIN", "true")

	server := api.NewServer(database, nil)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

// authenticateTestUser calls the dev login endpoint and returns the session cookie.
// All subsequent requests must include this cookie to be authenticated.
func authenticateTestUser(t *testing.T, server *api.Server, email string) *http.Cookie {
	t.Helper()

	body := bytes.NewBufferString(`{"email":"` + email + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/dev/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to authenticate as %s: %d %s", email, w.Code, w.Body.String())
	}

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "session" {
			return cookie
		}
	}
	t.Fatal("no session cookie in dev login response")
	return nil
}

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

func TestOAuthStart_DoesNotLeakInternalErrors(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Hit the OAuth start endpoint with an invalid provider.
	// The handler should return a generic error, not leak internal details
	// (e.g., no "unknown provider", no Go error strings).
	req := httptest.NewRequest("GET", "/api/v1/auth/oauth/invalid-provider", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Should get 400, not 500
	if w.Code == http.StatusInternalServerError {
		t.Errorf("OAuth start should not return 500 for invalid provider, got %d", w.Code)
	}

	body := w.Body.String()
	// Response must not contain internal error details
	forbiddenSubstrings := []string{"sql:", "query", "panic", "runtime", "goroutine", "nil pointer"}
	for _, s := range forbiddenSubstrings {
		if bytes.Contains(w.Body.Bytes(), []byte(s)) {
			t.Errorf("OAuth error response leaks internal details (contains %q): %s", s, body)
		}
	}

	// Should use the generic message
	if w.Code == http.StatusBadRequest && !bytes.Contains(w.Body.Bytes(), []byte("failed to start authentication")) {
		t.Errorf("expected generic error message, got: %s", body)
	}
}

func TestOAuthCallback_DoesNotLeakInternalErrors(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Hit the OAuth callback with no valid state/code — should fail gracefully
	req := httptest.NewRequest("GET", "/api/v1/auth/oauth/google/callback?code=fake&state=fake", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Should redirect (307) with generic auth_error, not leak details
	if w.Code == http.StatusInternalServerError {
		t.Errorf("OAuth callback should not return 500, got %d", w.Code)
	}

	// If it redirects, the Location should contain "auth_error=authentication_failed" (generic)
	location := w.Header().Get("Location")
	if w.Code == http.StatusTemporaryRedirect && location != "" {
		if bytes.Contains([]byte(location), []byte("err.Error")) ||
			bytes.Contains([]byte(location), []byte("sql:")) ||
			bytes.Contains([]byte(location), []byte("panic")) {
			t.Errorf("OAuth callback redirect leaks internal error in URL: %s", location)
		}
	}
}

func TestListGoals_Empty(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	req := httptest.NewRequest("GET", "/api/v1/goals", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var goals []models.Goal
	if err := json.NewDecoder(w.Body).Decode(&goals); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(goals) != 0 {
		t.Errorf("expected 0 goals, got %d", len(goals))
	}
}

func TestCreateGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	body := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var goal models.Goal
	if err := json.NewDecoder(w.Body).Decode(&goal); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if goal.Name != "Exercise" {
		t.Errorf("expected name 'Exercise', got '%s'", goal.Name)
	}
	if goal.Color != "#4CAF50" {
		t.Errorf("expected color '#4CAF50', got '%s'", goal.Color)
	}
	if goal.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateGoal_MissingName(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	body := bytes.NewBufferString(`{"color": "#4CAF50"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Update the goal
	updateBody := bytes.NewBufferString(`{"name": "Running", "color": "#2196F3"}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+createdGoal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	server.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var updatedGoal models.Goal
	json.NewDecoder(updateW.Body).Decode(&updatedGoal)

	if updatedGoal.Name != "Running" {
		t.Errorf("expected name 'Running', got '%s'", updatedGoal.Name)
	}
	if updatedGoal.Color != "#2196F3" {
		t.Errorf("expected color '#2196F3', got '%s'", updatedGoal.Color)
	}
}

func TestArchiveGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Archive the goal
	archiveReq := httptest.NewRequest("DELETE", "/api/v1/goals/"+createdGoal.ID, nil)
	archiveReq.AddCookie(cookie)
	archiveW := httptest.NewRecorder()
	server.ServeHTTP(archiveW, archiveReq)

	if archiveW.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", archiveW.Code)
	}

	// Verify goal is not in default list
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)

	if len(goals) != 0 {
		t.Errorf("expected 0 goals (archived), got %d", len(goals))
	}

	// Verify goal is in archived list
	listArchivedReq := httptest.NewRequest("GET", "/api/v1/goals?archived=true", nil)
	listArchivedReq.AddCookie(cookie)
	listArchivedW := httptest.NewRecorder()
	server.ServeHTTP(listArchivedW, listArchivedReq)

	var archivedGoals []models.Goal
	json.NewDecoder(listArchivedW.Body).Decode(&archivedGoals)

	if len(archivedGoals) != 1 {
		t.Errorf("expected 1 archived goal, got %d", len(archivedGoals))
	}
}

func TestCreateCompletion(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	completionReq.AddCookie(cookie)
	completionW := httptest.NewRecorder()
	server.ServeHTTP(completionW, completionReq)

	if completionW.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", completionW.Code, completionW.Body.String())
	}

	var completion models.Completion
	json.NewDecoder(completionW.Body).Decode(&completion)

	if completion.GoalID != createdGoal.ID {
		t.Errorf("expected goal_id '%s', got '%s'", createdGoal.ID, completion.GoalID)
	}
	if completion.Date != "2026-01-05" {
		t.Errorf("expected date '2026-01-05', got '%s'", completion.Date)
	}
}

func TestCreateCompletion_Idempotent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create same completion twice
	completionBody := `{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`

	req1 := httptest.NewRequest("POST", "/api/v1/completions", bytes.NewBufferString(completionBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.AddCookie(cookie)
	w1 := httptest.NewRecorder()
	server.ServeHTTP(w1, req1)

	var completion1 models.Completion
	json.NewDecoder(w1.Body).Decode(&completion1)

	req2 := httptest.NewRequest("POST", "/api/v1/completions", bytes.NewBufferString(completionBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	var completion2 models.Completion
	json.NewDecoder(w2.Body).Decode(&completion2)

	// Should return same completion
	if completion1.ID != completion2.ID {
		t.Errorf("expected same ID on idempotent create, got '%s' and '%s'", completion1.ID, completion2.ID)
	}
}

func TestListCompletions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create completions for multiple days (using past dates to avoid future date validation)
	dates := []string{"2025-12-01", "2025-12-15", "2025-12-25"}
	for _, date := range dates {
		body := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "` + date + `"}`)
		req := httptest.NewRequest("POST", "/api/v1/completions", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}

	// List completions for December 2025
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2025-12-01&to=2025-12-31", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", listW.Code)
	}

	var completions []models.Completion
	json.NewDecoder(listW.Body).Decode(&completions)

	if len(completions) != 3 {
		t.Errorf("expected 3 completions, got %d", len(completions))
	}
}

func TestDeleteCompletion(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	completionReq.AddCookie(cookie)
	completionW := httptest.NewRecorder()
	server.ServeHTTP(completionW, completionReq)

	var completion models.Completion
	json.NewDecoder(completionW.Body).Decode(&completion)

	// Delete the completion
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+completion.ID, nil)
	deleteReq.AddCookie(cookie)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", deleteW.Code)
	}

	// Verify completion is deleted
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var completions []models.Completion
	json.NewDecoder(listW.Body).Decode(&completions)

	if len(completions) != 0 {
		t.Errorf("expected 0 completions after delete, got %d", len(completions))
	}
}

func TestCreateCompletion_FutureDateRejected(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Try to create a completion for a future date (tomorrow)
	futureDate := "2099-12-31"
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "` + futureDate + `"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	completionReq.AddCookie(cookie)
	completionW := httptest.NewRecorder()
	server.ServeHTTP(completionW, completionReq)

	if completionW.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for future date, got %d: %s", completionW.Code, completionW.Body.String())
	}

	responseBody := completionW.Body.String()
	if responseBody != "cannot create completions for future dates\n" {
		t.Errorf("expected error message 'cannot create completions for future dates', got '%s'", responseBody)
	}
}

func TestGetCalendar(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	completionReq.AddCookie(cookie)
	server.ServeHTTP(httptest.NewRecorder(), completionReq)

	// Get calendar
	calendarReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
	calendarReq.AddCookie(cookie)
	calendarW := httptest.NewRecorder()
	server.ServeHTTP(calendarW, calendarReq)

	if calendarW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", calendarW.Code)
	}

	var calendar models.CalendarResponse
	json.NewDecoder(calendarW.Body).Decode(&calendar)

	if len(calendar.Goals) != 1 {
		t.Errorf("expected 1 goal, got %d", len(calendar.Goals))
	}
	if len(calendar.Completions) != 1 {
		t.Errorf("expected 1 completion, got %d", len(calendar.Completions))
	}
}

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

func TestSecurityHeaders_HSTS(t *testing.T) {
	// Test with COOKIE_SECURE unset (simulating production)
	orig := os.Getenv("COOKIE_SECURE")
	os.Unsetenv("COOKIE_SECURE")
	defer os.Setenv("COOKIE_SECURE", orig)

	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header")
	}

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected Strict-Transport-Security header when COOKIE_SECURE is not 'false'")
	}

	// Test with COOKIE_SECURE=false (dev mode) — HSTS should be absent
	os.Setenv("COOKIE_SECURE", "false")
	req2 := httptest.NewRequest("GET", "/health", nil)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when COOKIE_SECURE is 'false'")
	}
}

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

func TestSync_RejectsOversizedCompletions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Build a sync request with too many completions (over 5000)
	completions := make([]map[string]interface{}, 5001)
	for i := range completions {
		completions[i] = map[string]interface{}{
			"id": fmt.Sprintf("comp-%d", i), "goal_id": "goal-1",
			"date": "2026-01-01", "updated_at": time.Now().UTC(), "deleted": false,
		}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals":          []interface{}{},
		"completions":    completions,
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized completions sync, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCompletion_IsSoftDelete(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var goal models.Goal
	json.NewDecoder(createW.Body).Decode(&goal)

	// Create a completion
	compBody := bytes.NewBufferString(`{"goal_id": "` + goal.ID + `", "date": "2026-01-05"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(cookie)
	compW := httptest.NewRecorder()
	server.ServeHTTP(compW, compReq)

	var completion models.Completion
	json.NewDecoder(compW.Body).Decode(&completion)

	// Delete the completion
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+completion.ID, nil)
	deleteReq.AddCookie(cookie)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteW.Code)
	}

	// The completion should NOT appear in the list (filtered by deleted_at IS NULL)
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var completions []models.Completion
	json.NewDecoder(listW.Body).Decode(&completions)

	if len(completions) != 0 {
		t.Errorf("expected 0 visible completions, got %d", len(completions))
	}

	// But re-creating the same completion should succeed (idempotent-ish)
	// This verifies the soft-deleted record doesn't block a new one
	compBody2 := bytes.NewBufferString(`{"goal_id": "` + goal.ID + `", "date": "2026-01-05"}`)
	compReq2 := httptest.NewRequest("POST", "/api/v1/completions", compBody2)
	compReq2.Header.Set("Content-Type", "application/json")
	compReq2.AddCookie(cookie)
	compW2 := httptest.NewRecorder()
	server.ServeHTTP(compW2, compReq2)

	// Should succeed — either 200 (found existing) or 201 (created new)
	if compW2.Code != http.StatusOK && compW2.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201 after re-creating deleted completion, got %d: %s", compW2.Code, compW2.Body.String())
	}
}

func TestDeleteAccount_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "delete-me@test.com")

	// Create a goal so there's data to delete
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup: failed to create goal: %d %s", createW.Code, createW.Body.String())
	}

	// Delete the account
	req := httptest.NewRequest("DELETE", "/api/v1/account", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "account deleted" {
		t.Errorf("expected status 'account deleted', got %q", resp["status"])
	}

	// Verify session is invalidated — goals request should fail
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after account deletion, got %d", listW.Code)
	}
}

func TestDeleteAccount_Unauthenticated(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/api/v1/account", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSync_RoundTrip_GoalsAndCompletions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "sync@test.com")

	now := time.Now().UTC()

	// Send goals and completions via sync
	syncBody, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals": []map[string]interface{}{
			{
				"id": "sync-goal-1", "name": "Read", "color": "#FF0000",
				"position": 1, "updated_at": now.Format(time.RFC3339Nano), "deleted": false,
			},
			{
				"id": "sync-goal-2", "name": "Write", "color": "#00FF00",
				"position": 2, "updated_at": now.Format(time.RFC3339Nano), "deleted": false,
			},
		},
		"completions": []map[string]interface{}{
			{
				"goal_id": "sync-goal-1", "date": "2026-03-28",
				"completed": true, "updated_at": now.Format(time.RFC3339Nano),
			},
		},
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync request failed: %d %s", w.Code, w.Body.String())
	}

	var syncResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&syncResp)

	// Verify server_time is present
	if _, ok := syncResp["server_time"]; !ok {
		t.Fatal("response missing server_time")
	}

	// Verify goals are now accessible via REST API
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list goals failed: %d %s", listW.Code, listW.Body.String())
	}

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(goals))
	}

	// Verify second sync with last_synced_at returns changes from other devices
	serverTime := syncResp["server_time"].(string)

	// Simulate a change from "another device" — create a goal via REST
	createBody := bytes.NewBufferString(`{"name": "Meditate", "color": "#0000FF"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create goal failed: %d %s", createW.Code, createW.Body.String())
	}

	// Sync again with last_synced_at — should receive the new goal
	syncBody2, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": serverTime,
		"goals":          []interface{}{},
		"completions":    []interface{}{},
	})

	req2 := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody2))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second sync failed: %d %s", w2.Code, w2.Body.String())
	}

	var syncResp2 map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&syncResp2)

	serverGoals, ok := syncResp2["goals"].([]interface{})
	if !ok {
		t.Fatal("response missing goals array")
	}
	if len(serverGoals) != 1 {
		t.Errorf("expected 1 server change (the new goal), got %d", len(serverGoals))
	}
}

func TestCreateGoal_RejectsInvalidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	body := bytes.NewBufferString(`{"name": "Exercise", "target_count": 3, "target_period": "year"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid target_period, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGoal_RejectsInvalidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var goal models.Goal
	json.NewDecoder(createW.Body).Decode(&goal)

	// Try to update with invalid target_period
	body := bytes.NewBufferString(`{"target_period": "century"}`)
	req := httptest.NewRequest("PATCH", "/api/v1/goals/"+goal.ID, body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid target_period, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateGoal_AcceptsValidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	for _, period := range []string{"week", "month"} {
		body := bytes.NewBufferString(fmt.Sprintf(`{"name": "Goal %s", "target_count": 3, "target_period": "%s"}`, period, period))
		req := httptest.NewRequest("POST", "/api/v1/goals", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 for target_period=%q, got %d: %s", period, w.Code, w.Body.String())
		}
	}
}

func TestUpdateGoal_RejectsEmptyName(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Try to update with empty name
	updateBody := bytes.NewBufferString(`{"name": ""}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+createdGoal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	server.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d: %s", updateW.Code, updateW.Body.String())
	}
}

func TestSync_ArchivedGoalPreservesArchivedAt(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "sync-archive@test.com")

	now := time.Now().UTC()

	// Send an archived goal via sync
	syncBody, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals": []map[string]interface{}{
			{
				"id": "archived-goal-1", "name": "Old Habit", "color": "#FF0000",
				"position": 1, "updated_at": now.Format(time.RFC3339Nano),
				"deleted": false, "archived": true,
			},
		},
		"completions": []interface{}{},
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync failed: %d %s", w.Code, w.Body.String())
	}

	// Fetch goals including archived — the goal should be archived, not deleted
	listReq := httptest.NewRequest("GET", "/api/v1/goals?archived=true", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list goals failed: %d %s", listW.Code, listW.Body.String())
	}

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)

	found := false
	for _, g := range goals {
		if g.ID == "archived-goal-1" {
			found = true
			if g.ArchivedAt == nil {
				t.Error("expected ArchivedAt to be set")
			}
			if g.DeletedAt != nil {
				t.Error("expected DeletedAt to be nil (archived, not deleted)")
			}
		}
	}
	if !found {
		t.Error("archived goal not found in list (may have been treated as deleted)")
	}
}

func TestAuthCodeExchange(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Generate an auth code directly (simulating what oauthCallback would do)
	code := server.AuthCodeStore().Generate("test-session-token")

	// Exchange it
	body := bytes.NewBufferString(`{"code":"` + code + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/exchange", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["session_token"] != "test-session-token" {
		t.Errorf("expected test-session-token, got %s", resp["session_token"])
	}

	// Second exchange should fail
	body2 := bytes.NewBufferString(`{"code":"` + code + `"}`)
	req2 := httptest.NewRequest("POST", "/api/v1/auth/exchange", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for reused code, got %d", w2.Code)
	}
}
