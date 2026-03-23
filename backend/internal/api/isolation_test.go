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
