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
