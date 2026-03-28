package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

func setupTestDB(t *testing.T) (*SQLiteDB, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "goal-tracker-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := NewSQLite(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		database.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to migrate database: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return database, cleanup
}

func TestSoftDeleteCompletion_RespectsOwnership(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	aliceID := "alice-id"
	bobID := "bob-id"

	// Create users first (goals have FK constraint on users)
	if err := db.CreateUser(&models.User{ID: aliceID, Email: "alice@test.com", Name: "Alice", CreatedAt: now}); err != nil {
		t.Fatalf("failed to create Alice: %v", err)
	}
	if err := db.CreateUser(&models.User{ID: bobID, Email: "bob@test.com", Name: "Bob", CreatedAt: now}); err != nil {
		t.Fatalf("failed to create Bob: %v", err)
	}

	// Create Alice's goal and completion
	aliceGoal := &models.Goal{
		ID:        "alice-goal",
		Name:      "Exercise",
		Color:     "#FF0000",
		UserID:    &aliceID,
		CreatedAt: now,
	}
	if err := db.CreateGoal(aliceGoal); err != nil {
		t.Fatalf("failed to create Alice's goal: %v", err)
	}

	aliceCompletion := &models.Completion{
		ID:        "alice-comp",
		GoalID:    "alice-goal",
		Date:      "2026-01-15",
		CreatedAt: now,
	}
	if err := db.CreateCompletion(aliceCompletion); err != nil {
		t.Fatalf("failed to create Alice's completion: %v", err)
	}

	// Bob tries to soft-delete Alice's completion — should silently do nothing
	err := db.SoftDeleteCompletion(&bobID, "alice-goal", "2026-01-15")
	if err != nil {
		t.Fatalf("SoftDeleteCompletion returned error: %v", err)
	}

	// Verify Alice's completion is NOT deleted (still visible)
	comp, err := db.GetCompletionByGoalAndDate("alice-goal", "2026-01-15")
	if err != nil {
		t.Fatalf("failed to get completion: %v", err)
	}
	if comp == nil {
		t.Fatal("Alice's completion was deleted by Bob — ownership check failed")
	}
	if comp.DeletedAt != nil {
		t.Fatal("Alice's completion was soft-deleted by Bob — ownership check failed")
	}

	// Alice soft-deletes her own completion — should succeed
	err = db.SoftDeleteCompletion(&aliceID, "alice-goal", "2026-01-15")
	if err != nil {
		t.Fatalf("SoftDeleteCompletion by owner returned error: %v", err)
	}

	// Verify the completion is now soft-deleted (not visible via normal query)
	comp, err = db.GetCompletionByGoalAndDate("alice-goal", "2026-01-15")
	if err != nil {
		t.Fatalf("failed to get completion after owner delete: %v", err)
	}
	if comp != nil {
		t.Fatal("expected completion to be invisible after soft delete by owner")
	}

	// But it should still exist when including deleted
	comp, err = db.GetCompletionByGoalAndDateIncludingDeleted("alice-goal", "2026-01-15")
	if err != nil {
		t.Fatalf("failed to get completion including deleted: %v", err)
	}
	if comp == nil {
		t.Fatal("expected soft-deleted completion to exist when including deleted")
	}
	if comp.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set on soft-deleted completion")
	}
}

func TestUpsertCompletion_ConflictsOnGoalAndDate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	userID := "user-upsert-test"
	if err := db.CreateUser(&models.User{ID: userID, Email: "upsert@test.com", Name: "Upsert", CreatedAt: now}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a goal
	goal := &models.Goal{
		ID:        "goal-upsert-comp",
		Name:      "Test",
		Color:     "#000000",
		UserID:    &userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateGoal(goal); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// Insert a completion
	c1 := &models.Completion{
		ID:        "comp-1",
		GoalID:    "goal-upsert-comp",
		Date:      "2026-03-28",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.UpsertCompletion(c1); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Soft-delete it
	later := now.Add(time.Second)
	c1.DeletedAt = &later
	c1.UpdatedAt = later
	if err := db.UpsertCompletion(c1); err != nil {
		t.Fatalf("soft-delete upsert: %v", err)
	}

	// Upsert with a DIFFERENT ID but same (goal_id, date) — simulates sync race
	evenLater := later.Add(time.Second)
	c2 := &models.Completion{
		ID:        "comp-2-different",
		GoalID:    "goal-upsert-comp",
		Date:      "2026-03-28",
		CreatedAt: evenLater,
		UpdatedAt: evenLater,
	}
	if err := db.UpsertCompletion(c2); err != nil {
		t.Fatalf("upsert with different ID, same (goal_id, date) should not error: %v", err)
	}

	// Verify only one completion exists for this (goal_id, date)
	comp, err := db.GetCompletionByGoalAndDate("goal-upsert-comp", "2026-03-28")
	if err != nil {
		t.Fatalf("get completion: %v", err)
	}
	if comp == nil {
		t.Fatal("expected completion to exist after upsert")
	}
	if comp.DeletedAt != nil {
		t.Fatal("expected completion to be active (not soft-deleted) after upsert")
	}
	if comp.ID != "comp-2-different" {
		t.Fatalf("expected id to be updated to comp-2-different, got %s", comp.ID)
	}
}

func TestCreateGoal_PositionsAreSequential(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	userID := "user-1"
	if err := db.CreateUser(&models.User{ID: userID, Email: "test@test.com", Name: "Test", CreatedAt: now}); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create 3 goals and verify positions are 0, 1, 2
	for i := 0; i < 3; i++ {
		goal := &models.Goal{
			ID:        fmt.Sprintf("goal-%d", i),
			Name:      fmt.Sprintf("Goal %d", i),
			Color:     "#FF0000",
			UserID:    &userID,
			CreatedAt: now,
		}
		if err := db.CreateGoal(goal); err != nil {
			t.Fatalf("failed to create goal %d: %v", i, err)
		}
		if goal.Position != i {
			t.Errorf("goal %d: expected position %d, got %d", i, i, goal.Position)
		}
	}
}
