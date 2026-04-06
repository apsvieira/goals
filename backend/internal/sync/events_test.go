package sync

import (
	"testing"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

func setupEventsTest(t *testing.T) (*Service, string, func()) {
	t.Helper()
	database, cleanup := setupTestSyncDB(t)
	svc := NewService(database)

	// Create a test user
	user, err := database.GetOrCreateUserByProvider("test", "events-user", "events@test.com", "EventsUser", "")
	if err != nil {
		cleanup()
		t.Fatalf("create user: %v", err)
	}

	return svc, user.ID, cleanup
}

func TestProcessEvents_GoalUpsert(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	events := []EventRequest{
		{
			ID:        "evt-1",
			Type:      EventTypeGoalUpsert,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				ID:    "goal-1",
				Name:  "Run",
				Color: "#FF0000",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}

	if len(resp.Processed) != 1 || resp.Processed[0] != "evt-1" {
		t.Errorf("expected processed=[evt-1], got %v", resp.Processed)
	}

	// Verify the goal was created
	goal, err := svc.db.GetGoalByID("goal-1")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if goal == nil {
		t.Fatal("expected goal to exist")
	}
	if goal.Name != "Run" {
		t.Errorf("expected name 'Run', got '%s'", goal.Name)
	}
	if goal.Color != "#FF0000" {
		t.Errorf("expected color '#FF0000', got '%s'", goal.Color)
	}
	if goal.UserID == nil || *goal.UserID != userID {
		t.Errorf("expected goal owned by user %s", userID)
	}
	if goal.DeletedAt != nil {
		t.Error("expected goal not to be deleted")
	}
}

func TestProcessEvents_GoalUpsertWithTargets(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	targetCount := 3
	targetPeriod := "week"

	events := []EventRequest{
		{
			ID:        "evt-target",
			Type:      EventTypeGoalUpsert,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				ID:           "goal-target",
				Name:         "Exercise",
				Color:        "#00FF00",
				Position:     2,
				TargetCount:  &targetCount,
				TargetPeriod: &targetPeriod,
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 1 {
		t.Fatalf("expected 1 processed, got %d", len(resp.Processed))
	}

	goal, err := svc.db.GetGoalByID("goal-target")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if goal.TargetCount == nil || *goal.TargetCount != 3 {
		t.Errorf("expected target_count=3, got %v", goal.TargetCount)
	}
	if goal.TargetPeriod == nil || *goal.TargetPeriod != "week" {
		t.Errorf("expected target_period=week, got %v", goal.TargetPeriod)
	}
}

func TestProcessEvents_GoalDelete(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// First create a goal
	goal := &models.Goal{
		ID:        "goal-del",
		Name:      "ToDelete",
		Color:     "#000000",
		UserID:    &userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := svc.db.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	events := []EventRequest{
		{
			ID:        "evt-2",
			Type:      EventTypeGoalDelete,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				ID: "goal-del",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 1 || resp.Processed[0] != "evt-2" {
		t.Errorf("expected processed=[evt-2], got %v", resp.Processed)
	}

	// Verify the goal is soft-deleted
	deleted, err := svc.db.GetGoalByID("goal-del")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if deleted == nil {
		t.Fatal("expected goal to still exist (soft-deleted)")
	}
	if deleted.DeletedAt == nil {
		t.Error("expected goal to be soft-deleted")
	}
}

func TestProcessEvents_CompletionSet(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Create a goal first
	goal := &models.Goal{
		ID:        "goal-comp",
		Name:      "CompGoal",
		Color:     "#000000",
		UserID:    &userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := svc.db.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	events := []EventRequest{
		{
			ID:        "evt-3",
			Type:      EventTypeCompletionSet,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				GoalID: "goal-comp",
				Date:   "2026-04-05",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(resp.Processed))
	}

	// Verify the completion was created
	comp, err := svc.db.GetCompletionByGoalAndDateIncludingDeleted("goal-comp", "2026-04-05")
	if err != nil {
		t.Fatalf("GetCompletion failed: %v", err)
	}
	if comp == nil {
		t.Fatal("expected completion to exist")
	}
	if comp.DeletedAt != nil {
		t.Error("expected completion not to be deleted")
	}
}

func TestProcessEvents_CompletionUnset(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Create goal and completion
	goal := &models.Goal{
		ID:        "goal-uncomp",
		Name:      "UncompGoal",
		Color:     "#000000",
		UserID:    &userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := svc.db.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	comp := &models.Completion{
		ID:        "goal-uncomp:2026-04-05",
		GoalID:    "goal-uncomp",
		Date:      "2026-04-05",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := svc.db.UpsertCompletion(comp); err != nil {
		t.Fatalf("upsert completion: %v", err)
	}

	events := []EventRequest{
		{
			ID:        "evt-4",
			Type:      EventTypeCompletionUnset,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				GoalID: "goal-uncomp",
				Date:   "2026-04-05",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(resp.Processed))
	}

	// Verify the completion is soft-deleted
	result, err := svc.db.GetCompletionByGoalAndDateIncludingDeleted("goal-uncomp", "2026-04-05")
	if err != nil {
		t.Fatalf("GetCompletion failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected completion to still exist (soft-deleted)")
	}
	if result.DeletedAt == nil {
		t.Error("expected completion to be soft-deleted")
	}
}

func TestProcessEvents_Idempotency(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	events := []EventRequest{
		{
			ID:        "evt-idem",
			Type:      EventTypeGoalUpsert,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				ID:    "goal-idem",
				Name:  "First",
				Color: "#FF0000",
			},
		},
	}

	// Process first time
	resp1, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("first ProcessEvents failed: %v", err)
	}
	if len(resp1.Processed) != 1 {
		t.Fatalf("expected 1 processed, got %d", len(resp1.Processed))
	}

	// Process same event again with different payload
	eventsRetry := []EventRequest{
		{
			ID:        "evt-idem",
			Type:      EventTypeGoalUpsert,
			Timestamp: time.Now().UTC().Add(time.Minute),
			Payload: EventPayload{
				ID:    "goal-idem",
				Name:  "Updated",
				Color: "#00FF00",
			},
		},
	}

	resp2, err := svc.ProcessEvents(userID, eventsRetry)
	if err != nil {
		t.Fatalf("second ProcessEvents failed: %v", err)
	}
	if len(resp2.Processed) != 1 || resp2.Processed[0] != "evt-idem" {
		t.Errorf("expected processed=[evt-idem], got %v", resp2.Processed)
	}

	// Verify the goal was NOT updated (idempotency prevented re-processing)
	goal, err := svc.db.GetGoalByID("goal-idem")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if goal.Name != "First" {
		t.Errorf("expected name 'First' (original), got '%s' (idempotency failed)", goal.Name)
	}
	if goal.Color != "#FF0000" {
		t.Errorf("expected color '#FF0000' (original), got '%s'", goal.Color)
	}
}

func TestProcessEvents_BatchProcessing(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	now := time.Now().UTC()
	events := []EventRequest{
		{
			ID:        "evt-b1",
			Type:      EventTypeGoalUpsert,
			Timestamp: now,
			Payload: EventPayload{
				ID:    "goal-b1",
				Name:  "Goal One",
				Color: "#FF0000",
			},
		},
		{
			ID:        "evt-b2",
			Type:      EventTypeGoalUpsert,
			Timestamp: now.Add(time.Second),
			Payload: EventPayload{
				ID:    "goal-b2",
				Name:  "Goal Two",
				Color: "#00FF00",
			},
		},
		{
			ID:        "evt-b3",
			Type:      EventTypeGoalUpsert,
			Timestamp: now.Add(2 * time.Second),
			Payload: EventPayload{
				ID:    "goal-b3",
				Name:  "Goal Three",
				Color: "#0000FF",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 3 {
		t.Fatalf("expected 3 processed, got %d", len(resp.Processed))
	}

	// Verify all goals were created
	for _, id := range []string{"goal-b1", "goal-b2", "goal-b3"} {
		goal, err := svc.db.GetGoalByID(id)
		if err != nil {
			t.Fatalf("GetGoalByID(%s) failed: %v", id, err)
		}
		if goal == nil {
			t.Errorf("expected goal %s to exist", id)
		}
	}
}

func TestProcessEvents_InvalidEventType(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	events := []EventRequest{
		{
			ID:        "evt-bad",
			Type:      "invalid_type",
			Timestamp: time.Now().UTC(),
			Payload:   EventPayload{},
		},
	}

	_, err := svc.ProcessEvents(userID, events)
	if err == nil {
		t.Fatal("expected error for invalid event type")
	}
}

func TestProcessEvents_GoalOwnership(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Create another user
	otherUser, err := svc.db.GetOrCreateUserByProvider("test", "other-user", "other@test.com", "Other", "")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	otherID := otherUser.ID

	// Create a goal owned by the other user
	goal := &models.Goal{
		ID:        "goal-other",
		Name:      "OtherGoal",
		Color:     "#000000",
		UserID:    &otherID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := svc.db.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	// Try to upsert the other user's goal
	events := []EventRequest{
		{
			ID:        "evt-own",
			Type:      EventTypeGoalUpsert,
			Timestamp: time.Now().UTC().Add(time.Hour),
			Payload: EventPayload{
				ID:    "goal-other",
				Name:  "Hijacked",
				Color: "#FF0000",
			},
		},
	}

	_, err = svc.ProcessEvents(userID, events)
	if err == nil {
		t.Fatal("expected error when modifying another user's goal")
	}
}

func TestProcessEvents_CompletionOwnershipCheck(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Create another user with a goal
	otherUser, err := svc.db.GetOrCreateUserByProvider("test", "comp-other", "comp-other@test.com", "Other", "")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	otherID := otherUser.ID

	goal := &models.Goal{
		ID:        "goal-comp-other",
		Name:      "OtherGoal",
		Color:     "#000000",
		UserID:    &otherID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := svc.db.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	// Try to set completion on the other user's goal
	events := []EventRequest{
		{
			ID:        "evt-comp-own",
			Type:      EventTypeCompletionSet,
			Timestamp: time.Now().UTC(),
			Payload: EventPayload{
				GoalID: "goal-comp-other",
				Date:   "2026-04-05",
			},
		},
	}

	_, err = svc.ProcessEvents(userID, events)
	if err == nil {
		t.Fatal("expected error when setting completion on another user's goal")
	}
}

func TestProcessEvents_EventsProcessedInTimestampOrder(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	now := time.Now().UTC()

	// Send events in reverse order: the create is timestamped after the update
	// Both refer to the same goal, so order matters
	events := []EventRequest{
		{
			ID:        "evt-order-2",
			Type:      EventTypeGoalUpsert,
			Timestamp: now.Add(time.Second), // later timestamp
			Payload: EventPayload{
				ID:    "goal-order",
				Name:  "Updated Name",
				Color: "#00FF00",
			},
		},
		{
			ID:        "evt-order-1",
			Type:      EventTypeGoalUpsert,
			Timestamp: now, // earlier timestamp
			Payload: EventPayload{
				ID:    "goal-order",
				Name:  "Original Name",
				Color: "#FF0000",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 2 {
		t.Fatalf("expected 2 processed, got %d", len(resp.Processed))
	}

	// The goal should have the "Updated Name" because that event has the later timestamp
	goal, err := svc.db.GetGoalByID("goal-order")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if goal.Name != "Updated Name" {
		t.Errorf("expected 'Updated Name', got '%s' (events not processed in timestamp order)", goal.Name)
	}
}

func TestProcessEvents_EmptyBatch(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Empty events slice - this would be caught by the handler validation,
	// but the service itself should handle it gracefully
	events := []EventRequest{}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 0 {
		t.Errorf("expected 0 processed, got %d", len(resp.Processed))
	}
}

func TestProcessEvents_GoalUpsertStaleTimestamp_LWWRejects(t *testing.T) {
	svc, userID, cleanup := setupEventsTest(t)
	defer cleanup()

	// Insert a goal with a recent timestamp directly in the DB (server copy).
	serverTime := time.Now().UTC()
	serverGoal := &models.Goal{
		ID:        "goal-lww",
		Name:      "ServerName",
		Color:     "#00FF00",
		Position:  1,
		UserID:    &userID,
		CreatedAt: serverTime,
		UpdatedAt: serverTime,
	}
	if err := svc.db.UpsertGoal(serverGoal); err != nil {
		t.Fatalf("upsert server goal: %v", err)
	}

	// Send a goal_upsert event with an older timestamp (stale client change).
	staleTime := serverTime.Add(-time.Hour)
	events := []EventRequest{
		{
			ID:        "evt-stale",
			Type:      EventTypeGoalUpsert,
			Timestamp: staleTime,
			Payload: EventPayload{
				ID:    "goal-lww",
				Name:  "StaleName",
				Color: "#FF0000",
			},
		},
	}

	resp, err := svc.ProcessEvents(userID, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}
	if len(resp.Processed) != 1 || resp.Processed[0] != "evt-stale" {
		t.Errorf("expected processed=[evt-stale], got %v", resp.Processed)
	}

	// Verify the goal is unchanged (server wins LWW).
	goal, err := svc.db.GetGoalByID("goal-lww")
	if err != nil {
		t.Fatalf("GetGoalByID failed: %v", err)
	}
	if goal == nil {
		t.Fatal("expected goal to exist")
	}
	if goal.Name != "ServerName" {
		t.Errorf("expected name 'ServerName' (server wins), got '%s'", goal.Name)
	}
	if goal.Color != "#00FF00" {
		t.Errorf("expected color '#00FF00' (server wins), got '%s'", goal.Color)
	}
	if goal.Position != 1 {
		t.Errorf("expected position 1 (server wins), got %d", goal.Position)
	}
}
