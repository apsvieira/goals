package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
	database, err := db.New(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		database.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to migrate database: %v", err)
	}

	server := api.NewServer(database, nil)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

func TestListGoals_Empty(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/goals", nil)
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

	body := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
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

	body := bytes.NewBufferString(`{"color": "#4CAF50"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateGoal(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Update the goal
	updateBody := bytes.NewBufferString(`{"name": "Running", "color": "#2196F3"}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+createdGoal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
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

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Archive the goal
	archiveReq := httptest.NewRequest("DELETE", "/api/v1/goals/"+createdGoal.ID, nil)
	archiveW := httptest.NewRecorder()
	server.ServeHTTP(archiveW, archiveReq)

	if archiveW.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", archiveW.Code)
	}

	// Verify goal is not in default list
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)

	if len(goals) != 0 {
		t.Errorf("expected 0 goals (archived), got %d", len(goals))
	}

	// Verify goal is in archived list
	listArchivedReq := httptest.NewRequest("GET", "/api/v1/goals?archived=true", nil)
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

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
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

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create same completion twice
	completionBody := `{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`

	req1 := httptest.NewRequest("POST", "/api/v1/completions", bytes.NewBufferString(completionBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.ServeHTTP(w1, req1)

	var completion1 models.Completion
	json.NewDecoder(w1.Body).Decode(&completion1)

	req2 := httptest.NewRequest("POST", "/api/v1/completions", bytes.NewBufferString(completionBody))
	req2.Header.Set("Content-Type", "application/json")
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

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create completions for multiple days
	dates := []string{"2026-01-01", "2026-01-05", "2026-01-10"}
	for _, date := range dates {
		body := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "` + date + `"}`)
		req := httptest.NewRequest("POST", "/api/v1/completions", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}

	// List completions for January
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
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

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	completionW := httptest.NewRecorder()
	server.ServeHTTP(completionW, completionReq)

	var completion models.Completion
	json.NewDecoder(completionW.Body).Decode(&completion)

	// Delete the completion
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+completion.ID, nil)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", deleteW.Code)
	}

	// Verify completion is deleted
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var completions []models.Completion
	json.NewDecoder(listW.Body).Decode(&completions)

	if len(completions) != 0 {
		t.Errorf("expected 0 completions after delete, got %d", len(completions))
	}
}

func TestGetCalendar(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var createdGoal models.Goal
	json.NewDecoder(createW.Body).Decode(&createdGoal)

	// Create a completion
	completionBody := bytes.NewBufferString(`{"goal_id": "` + createdGoal.ID + `", "date": "2026-01-05"}`)
	completionReq := httptest.NewRequest("POST", "/api/v1/completions", completionBody)
	completionReq.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(httptest.NewRecorder(), completionReq)

	// Get calendar
	calendarReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
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
