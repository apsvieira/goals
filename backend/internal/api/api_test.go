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
