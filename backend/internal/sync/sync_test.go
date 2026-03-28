package sync

import (
	"sync"
	"testing"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

func setupTestSyncDB(t *testing.T) (db.Database, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	database, err := db.NewSQLite(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		database.Close()
		t.Fatalf("migrate: %v", err)
	}
	return database, func() { database.Close() }
}

func TestApplyChanges_SerializesPerUser(t *testing.T) {
	database, cleanup := setupTestSyncDB(t)
	defer cleanup()

	svc := NewService(database)

	// Create user (the DB assigns the actual user ID)
	user, err := database.GetOrCreateUserByProvider("test", "serial", "serial@test.com", "Test", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := user.ID

	// Create a goal that both sync requests will modify
	goal := &models.Goal{
		ID:        "goal-serial",
		Name:      "Original",
		Color:     "#000000",
		UserID:    &userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := database.UpsertGoal(goal); err != nil {
		t.Fatalf("upsert goal: %v", err)
	}

	// Run two sync requests concurrently
	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "Update-" + string(rune('A'+idx))
			tp := "week"
			req := &SyncRequest{
				Goals: []GoalChange{
					{
						ID:           "goal-serial",
						Name:         name,
						Color:        "#111111",
						TargetPeriod: &tp,
						UpdatedAt:    time.Now().UTC().Add(time.Duration(idx) * time.Second),
					},
				},
			}
			_, errs[idx] = svc.ApplyChanges(userID, req)
		}(i)
	}
	wg.Wait()

	// Both should succeed (no constraint violations or data races)
	for i, err := range errs {
		if err != nil {
			t.Errorf("sync %d failed: %v", i, err)
		}
	}
}
