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
