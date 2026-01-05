package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// TestE2E_FullUserFlow tests the complete user journey:
// 1. Create a goal
// 2. Toggle completions for multiple days
// 3. Navigate months (via calendar endpoint)
// 4. Update goal
// 5. Archive goal
func TestE2E_FullUserFlow(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Step 1: Create a goal "Exercise"
	t.Log("Step 1: Creating goal 'Exercise'")
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("Step 1: Expected 201, got %d: %s", createW.Code, createW.Body.String())
	}

	var goal models.Goal
	json.NewDecoder(createW.Body).Decode(&goal)
	t.Logf("Created goal: %s (ID: %s)", goal.Name, goal.ID)

	// Step 2: Toggle completions for days 1, 5, 10 of January 2026
	t.Log("Step 2: Marking completions for days 1, 5, 10")
	days := []string{"2026-01-01", "2026-01-05", "2026-01-10"}
	completionIDs := make([]string, 0, len(days))

	for _, date := range days {
		body := bytes.NewBufferString(`{"goal_id": "` + goal.ID + `", "date": "` + date + `"}`)
		req := httptest.NewRequest("POST", "/api/v1/completions", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("Step 2: Expected 201 for %s, got %d", date, w.Code)
		}

		var completion models.Completion
		json.NewDecoder(w.Body).Decode(&completion)
		completionIDs = append(completionIDs, completion.ID)
		t.Logf("Marked complete: %s", date)
	}

	// Step 3: Get January 2026 calendar
	t.Log("Step 3: Fetching calendar for January 2026")
	calendarReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
	calendarW := httptest.NewRecorder()
	server.ServeHTTP(calendarW, calendarReq)

	if calendarW.Code != http.StatusOK {
		t.Fatalf("Step 3: Expected 200, got %d", calendarW.Code)
	}

	var calendar models.CalendarResponse
	json.NewDecoder(calendarW.Body).Decode(&calendar)

	if len(calendar.Goals) != 1 {
		t.Errorf("Step 3: Expected 1 goal, got %d", len(calendar.Goals))
	}
	if len(calendar.Completions) != 3 {
		t.Errorf("Step 3: Expected 3 completions, got %d", len(calendar.Completions))
	}
	t.Logf("Calendar shows %d goals, %d completions", len(calendar.Goals), len(calendar.Completions))

	// Step 4: Navigate to February (should be empty)
	t.Log("Step 4: Checking February 2026 is empty")
	febReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-02", nil)
	febW := httptest.NewRecorder()
	server.ServeHTTP(febW, febReq)

	var febCalendar models.CalendarResponse
	json.NewDecoder(febW.Body).Decode(&febCalendar)

	if len(febCalendar.Goals) != 1 {
		t.Errorf("Step 4: Expected 1 goal in Feb, got %d", len(febCalendar.Goals))
	}
	if len(febCalendar.Completions) != 0 {
		t.Errorf("Step 4: Expected 0 completions in Feb, got %d", len(febCalendar.Completions))
	}
	t.Log("February correctly shows 0 completions")

	// Step 5: Untoggle one completion (delete day 5)
	t.Log("Step 5: Untoggling completion for day 5")
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+completionIDs[1], nil)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Step 5: Expected 204, got %d", deleteW.Code)
	}

	// Verify only 2 completions remain
	verifyReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
	verifyW := httptest.NewRecorder()
	server.ServeHTTP(verifyW, verifyReq)

	var verifyCalendar models.CalendarResponse
	json.NewDecoder(verifyW.Body).Decode(&verifyCalendar)

	if len(verifyCalendar.Completions) != 2 {
		t.Errorf("Step 5: Expected 2 completions after delete, got %d", len(verifyCalendar.Completions))
	}
	t.Log("Verified 2 completions remain after untoggle")

	// Step 6: Update goal name and color
	t.Log("Step 6: Updating goal to 'Running' with blue color")
	updateBody := bytes.NewBufferString(`{"name": "Running", "color": "#2196F3"}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+goal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	server.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("Step 6: Expected 200, got %d", updateW.Code)
	}

	var updatedGoal models.Goal
	json.NewDecoder(updateW.Body).Decode(&updatedGoal)

	if updatedGoal.Name != "Running" {
		t.Errorf("Step 6: Expected name 'Running', got '%s'", updatedGoal.Name)
	}
	if updatedGoal.Color != "#2196F3" {
		t.Errorf("Step 6: Expected color '#2196F3', got '%s'", updatedGoal.Color)
	}
	t.Log("Goal updated successfully")

	// Step 7: Create a second goal
	t.Log("Step 7: Creating second goal 'No Smoking'")
	secondBody := bytes.NewBufferString(`{"name": "No Smoking", "color": "#E91E63"}`)
	secondReq := httptest.NewRequest("POST", "/api/v1/goals", secondBody)
	secondReq.Header.Set("Content-Type", "application/json")
	secondW := httptest.NewRecorder()
	server.ServeHTTP(secondW, secondReq)

	if secondW.Code != http.StatusCreated {
		t.Fatalf("Step 7: Expected 201, got %d", secondW.Code)
	}

	// Verify 2 goals in calendar
	finalReq := httptest.NewRequest("GET", "/api/v1/calendar?month=2026-01", nil)
	finalW := httptest.NewRecorder()
	server.ServeHTTP(finalW, finalReq)

	var finalCalendar models.CalendarResponse
	json.NewDecoder(finalW.Body).Decode(&finalCalendar)

	if len(finalCalendar.Goals) != 2 {
		t.Errorf("Step 7: Expected 2 goals, got %d", len(finalCalendar.Goals))
	}
	t.Log("Now tracking 2 goals")

	// Step 8: Archive the first goal
	t.Log("Step 8: Archiving 'Running' goal")
	archiveReq := httptest.NewRequest("DELETE", "/api/v1/goals/"+goal.ID, nil)
	archiveW := httptest.NewRecorder()
	server.ServeHTTP(archiveW, archiveReq)

	if archiveW.Code != http.StatusNoContent {
		t.Fatalf("Step 8: Expected 204, got %d", archiveW.Code)
	}

	// Verify only 1 active goal remains
	activeReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	activeW := httptest.NewRecorder()
	server.ServeHTTP(activeW, activeReq)

	var activeGoals []models.Goal
	json.NewDecoder(activeW.Body).Decode(&activeGoals)

	if len(activeGoals) != 1 {
		t.Errorf("Step 8: Expected 1 active goal, got %d", len(activeGoals))
	}
	if activeGoals[0].Name != "No Smoking" {
		t.Errorf("Step 8: Expected 'No Smoking', got '%s'", activeGoals[0].Name)
	}
	t.Log("Only 'No Smoking' goal remains active")

	t.Log("E2E test completed successfully!")
}
