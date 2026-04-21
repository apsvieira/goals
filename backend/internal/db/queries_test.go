package db

import (
	"encoding/json"
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

func TestDebugReports_CreateAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	userID := "dbg-user-1"
	if err := db.CreateUser(&models.User{ID: userID, Email: "dbg@test.com", Name: "Dbg", CreatedAt: now}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	report := &models.DebugReport{
		UserID:      userID,
		ClientID:    "11111111-1111-1111-1111-111111111111",
		Trigger:     "shake",
		AppVersion:  "0.4.1",
		Platform:    "android",
		Device:      json.RawMessage(`{"model":"Pixel 8"}`),
		State:       json.RawMessage(`{"route":"home"}`),
		Description: "missing completion",
		Breadcrumbs: json.RawMessage(`[{"ts":1,"category":"log","level":"info","message":"hi"}]`),
	}
	if err := db.CreateDebugReport(report); err != nil {
		t.Fatalf("create debug report: %v", err)
	}
	if report.ID == "" {
		t.Fatal("expected ID to be populated")
	}

	got, err := db.GetDebugReport(report.ID)
	if err != nil {
		t.Fatalf("get debug report: %v", err)
	}
	if got == nil {
		t.Fatal("expected report, got nil")
	}
	if got.UserID != userID {
		t.Errorf("user_id: want %q got %q", userID, got.UserID)
	}
	if got.Trigger != "shake" {
		t.Errorf("trigger: want shake got %q", got.Trigger)
	}
	if got.Description != "missing completion" {
		t.Errorf("description: want %q got %q", "missing completion", got.Description)
	}
	if string(got.Device) == "" {
		t.Error("expected device json to round-trip")
	}
}

func TestDebugReports_ListFiltersByUserAndSince(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	alice := "alice-dbg"
	bob := "bob-dbg"
	if err := db.CreateUser(&models.User{ID: alice, Email: "a@t.com", Name: "A", CreatedAt: now}); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := db.CreateUser(&models.User{ID: bob, Email: "b@t.com", Name: "B", CreatedAt: now}); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	mk := func(owner, id string, created time.Time) *models.DebugReport {
		return &models.DebugReport{
			ID:          id,
			UserID:      owner,
			ClientID:    "22222222-2222-2222-2222-222222222222",
			CreatedAt:   created,
			Trigger:     "shake",
			AppVersion:  "0.4.1",
			Platform:    "android",
			Device:      json.RawMessage(`{}`),
			State:       json.RawMessage(`{}`),
			Breadcrumbs: json.RawMessage(`[]`),
		}
	}

	// Alice: 2 reports, one old (10 days ago), one recent (1 hour ago).
	// Bob: 1 report, recent.
	aliceOld := mk(alice, "alice-old", now.Add(-10*24*time.Hour))
	aliceNew := mk(alice, "alice-new", now.Add(-1*time.Hour))
	bobNew := mk(bob, "bob-new", now.Add(-30*time.Minute))
	for _, r := range []*models.DebugReport{aliceOld, aliceNew, bobNew} {
		if err := db.CreateDebugReport(r); err != nil {
			t.Fatalf("create %s: %v", r.ID, err)
		}
	}

	// Filter by alice — both her reports, no bob.
	aliceAll, err := db.ListDebugReports(DebugReportFilter{UserID: &alice})
	if err != nil {
		t.Fatalf("list alice: %v", err)
	}
	if len(aliceAll) != 2 {
		t.Fatalf("expected 2 reports for alice, got %d", len(aliceAll))
	}
	// Ordered DESC by created_at — newest first.
	if aliceAll[0].ID != "alice-new" {
		t.Errorf("expected alice-new first, got %q", aliceAll[0].ID)
	}

	// Filter by since — only last day — alice-old drops out.
	since := now.Add(-24 * time.Hour)
	recent, err := db.ListDebugReports(DebugReportFilter{Since: &since})
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent reports, got %d", len(recent))
	}
	for _, r := range recent {
		if r.ID == "alice-old" {
			t.Errorf("alice-old should have been filtered by since")
		}
	}

	// Limit.
	limited, err := db.ListDebugReports(DebugReportFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1 report with limit=1, got %d", len(limited))
	}
}

func TestDebugReports_DeleteOld(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	userID := "dbg-cleanup"
	if err := db.CreateUser(&models.User{ID: userID, Email: "c@t.com", Name: "C", CreatedAt: now}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	mk := func(id string, created time.Time) *models.DebugReport {
		return &models.DebugReport{
			ID:          id,
			UserID:      userID,
			ClientID:    "33333333-3333-3333-3333-333333333333",
			CreatedAt:   created,
			Trigger:     "auto",
			AppVersion:  "0.4.1",
			Platform:    "android",
			Device:      json.RawMessage(`{}`),
			State:       json.RawMessage(`{}`),
			Breadcrumbs: json.RawMessage(`[]`),
		}
	}

	// Four reports at 100d, 91d, 89d, 1d old. With a 90-day cutoff, only the
	// 100d and 91d reports should be deleted.
	reports := []*models.DebugReport{
		mk("r-100d", now.Add(-100*24*time.Hour)),
		mk("r-91d", now.Add(-91*24*time.Hour)),
		mk("r-89d", now.Add(-89*24*time.Hour)),
		mk("r-1d", now.Add(-1*24*time.Hour)),
	}
	for _, r := range reports {
		if err := db.CreateDebugReport(r); err != nil {
			t.Fatalf("create %s: %v", r.ID, err)
		}
	}

	cutoff := now.Add(-90 * 24 * time.Hour)
	n, err := db.DeleteOldDebugReports(cutoff)
	if err != nil {
		t.Fatalf("delete old: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 rows deleted, got %d", n)
	}

	remaining, err := db.ListDebugReports(DebugReportFilter{})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining reports, got %d", len(remaining))
	}
	seen := map[string]bool{}
	for _, r := range remaining {
		seen[r.ID] = true
	}
	if seen["r-100d"] || seen["r-91d"] {
		t.Errorf("old reports not deleted: %+v", seen)
	}
	if !seen["r-89d"] || !seen["r-1d"] {
		t.Errorf("recent reports missing: %+v", seen)
	}
}

func TestDebugReports_AccountDeleteCascades(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	userID := "dbg-cascade"
	if err := db.CreateUser(&models.User{ID: userID, Email: "cc@t.com", Name: "CC", CreatedAt: now}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	report := &models.DebugReport{
		UserID:      userID,
		ClientID:    "44444444-4444-4444-4444-444444444444",
		Trigger:     "shake",
		AppVersion:  "0.4.1",
		Platform:    "android",
		Device:      json.RawMessage(`{}`),
		State:       json.RawMessage(`{}`),
		Breadcrumbs: json.RawMessage(`[]`),
	}
	if err := db.CreateDebugReport(report); err != nil {
		t.Fatalf("create report: %v", err)
	}
	reportID := report.ID

	if err := db.DeleteAccount(userID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	got, err := db.GetDebugReport(reportID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Errorf("expected cascade delete to remove debug report, still present: %+v", got)
	}
}
