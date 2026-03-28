# Mid/Low Severity Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve the remaining mid-to-low severity issues (#3, #6, #7, #8, #9, #10) from the pre-launch code review.

**Architecture:** Six independent fixes spanning backend validation, UUID generation, database atomicity, sync protocol, and mobile auth security. Backend is Go with SQLite (test) and PostgreSQL (prod). Frontend is TypeScript/Svelte. All backend changes need dual implementation in `queries.go` (SQLite) and `postgres.go` (PostgreSQL).

**Tech Stack:** Go 1.22+, TypeScript/Svelte, SQLite (test), PostgreSQL (prod), `net/http/httptest`, `google/uuid`

**Review reference:** `~/agent-notes/code-reviews/2026-03/goal-tracker-pre-launch-review.md`

---

## Triage Note

Issue #4 (sync round-trip test) already exists at `api_test.go:832` (`TestSync_RoundTrip_GoalsAndCompletions`), implemented as part of the blocking-issues plan. No further action needed.

---

## Task 1: Reject empty name in `updateGoal` (review issue #3)

**Severity:** Medium

The `updateGoal` handler at `goals.go:121` checks `*req.Name != ""` before validating length, which means `{"name": ""}` bypasses validation entirely. The `createGoal` handler correctly rejects empty names via `validateGoalName()`.

**Files:**
- Modify: `backend/internal/api/goals.go:120-126`
- Test: `backend/internal/api/api_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/api/api_test.go`:

```go
func TestUpdateGoal_RejectsEmptyName(t *testing.T) {
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

	// Try to update with empty name
	updateBody := bytes.NewBufferString(`{"name": ""}`)
	updateReq := httptest.NewRequest("PATCH", "/api/v1/goals/"+createdGoal.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	server.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d: %s", updateW.Code, updateW.Body.String())
	}
}
```

**Step 2: Run the test to verify it fails**

Run: `cd backend && go test ./internal/api/ -v -run TestUpdateGoal_RejectsEmptyName`
Expected: FAIL — returns 200 instead of 400

**Step 3: Implement the fix**

In `backend/internal/api/goals.go`, replace the name validation block (lines 120-126):

```go
	// Validate name if provided
	if req.Name != nil {
		if valid, errMsg := validateGoalName(*req.Name); !valid {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}
```

This reuses `validateGoalName()` (already used by `createGoal`), which rejects empty strings and strings over 200 chars.

**Step 4: Run the test to verify it passes**

Run: `cd backend && go test ./internal/api/ -v -run TestUpdateGoal`
Expected: Both `TestUpdateGoal` and `TestUpdateGoal_RejectsEmptyName` PASS

**Step 5: Commit**

```bash
git add backend/internal/api/goals.go backend/internal/api/api_test.go
git commit -m "fix(api): reject empty name in updateGoal

Previously, sending {\"name\": \"\"} to PATCH /goals/:id bypassed
validation because the empty-string check was combined with the nil
check. Now uses validateGoalName() consistently with createGoal."
```

---

## Task 2: Add `target_period` validation (review issue #10)

**Severity:** Low

`target_period` is accepted as any string in `createGoal`, `updateGoal`, and the sync endpoint. Valid values are `"week"` and `"month"` (per `models.go:11`). A client could persist `"year"` or `"banana"`.

**Files:**
- Modify: `backend/internal/api/goals.go` (add validator, use in create + update)
- Modify: `backend/internal/sync/sync.go:60-84` (validate during sync merge)
- Test: `backend/internal/api/api_test.go`

**Step 1: Write the failing tests**

Add to `backend/internal/api/api_test.go`:

```go
func TestCreateGoal_RejectsInvalidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	body := bytes.NewBufferString(`{"name": "Exercise", "target_count": 3, "target_period": "year"}`)
	req := httptest.NewRequest("POST", "/api/v1/goals", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid target_period, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGoal_RejectsInvalidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal first
	createBody := bytes.NewBufferString(`{"name": "Exercise"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var goal models.Goal
	json.NewDecoder(createW.Body).Decode(&goal)

	// Try to update with invalid target_period
	body := bytes.NewBufferString(`{"target_period": "century"}`)
	req := httptest.NewRequest("PATCH", "/api/v1/goals/"+goal.ID, body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid target_period, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateGoal_AcceptsValidTargetPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	for _, period := range []string{"week", "month"} {
		body := bytes.NewBufferString(fmt.Sprintf(`{"name": "Goal %s", "target_count": 3, "target_period": "%s"}`, period, period))
		req := httptest.NewRequest("POST", "/api/v1/goals", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 for target_period=%q, got %d: %s", period, w.Code, w.Body.String())
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/api/ -v -run TestCreateGoal_RejectsInvalidTargetPeriod`
Expected: FAIL — returns 201 instead of 400

**Step 3: Add the validator**

In `backend/internal/api/goals.go`, after the `validateColor` function (around line 48), add:

```go
// validateTargetPeriod checks if the target period is "week" or "month"
func validateTargetPeriod(period string) (bool, string) {
	if period != "week" && period != "month" {
		return false, "target_period must be \"week\" or \"month\""
	}
	return true, ""
}
```

**Step 4: Add validation to `createGoal`**

In `backend/internal/api/goals.go`, inside `createGoal`, after the color validation block (around line 84) and before the `if req.Color == ""` default, add:

```go
	// Validate target_period if provided
	if req.TargetPeriod != nil {
		if valid, errMsg := validateTargetPeriod(*req.TargetPeriod); !valid {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}
```

**Step 5: Add validation to `updateGoal`**

In `backend/internal/api/goals.go`, inside `updateGoal`, after the color validation block (around line 134), add:

```go
	// Validate target_period if provided
	if req.TargetPeriod != nil {
		if valid, errMsg := validateTargetPeriod(*req.TargetPeriod); !valid {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}
```

**Step 6: Add validation in sync's `ApplyChanges`**

In `backend/internal/sync/sync.go`, inside the `for _, clientGoal := range req.Goals` loop (around line 60), before the `MergeGoal` call, add validation. Note: for sync we should **skip** invalid goals rather than fail the entire sync, to maintain resilience:

```go
		// Validate target_period if provided
		if clientGoal.TargetPeriod != nil && *clientGoal.TargetPeriod != "week" && *clientGoal.TargetPeriod != "month" {
			continue // Skip goals with invalid target_period
		}
```

**Step 7: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/api/ -v -run "TestCreateGoal_Rejects|TestUpdateGoal_Rejects|TestCreateGoal_Accepts"`
Expected: All PASS

**Step 8: Run full test suite**

Run: `cd backend && go test ./...`
Expected: All PASS

**Step 9: Commit**

```bash
git add backend/internal/api/goals.go backend/internal/sync/sync.go backend/internal/api/api_test.go
git commit -m "fix(api): validate target_period is 'week' or 'month'

Previously, any string was accepted for target_period in create, update,
and sync endpoints. Now rejects invalid values in the REST API (400) and
silently skips them during sync to avoid failing the entire sync batch."
```

---

## Task 3: Replace weak UUID generation (review issue #6)

**Severity:** Low

`generateUUID()` in `queries.go:646` and `generatePostgresUUID()` in `postgres.go:744` use `time.Now().UnixNano()` which can produce duplicates under concurrent requests. These are used for `auth_providers`, `device_tokens`, and `sessions` — not for goals/completions (which already use `uuid.New()`). The fix is to replace them with `uuid.New().String()`.

**Files:**
- Modify: `backend/internal/db/queries.go:646-649`
- Modify: `backend/internal/db/postgres.go:744-747`

**Step 1: Run the existing tests as baseline**

Run: `cd backend && go test ./internal/db/ -v`
Expected: All PASS

**Step 2: Replace `generateUUID` in SQLite impl**

In `backend/internal/db/queries.go`, replace:

```go
// generateUUID creates a simple UUID for IDs
func generateUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}
```

with:

```go
// generateUUID creates a UUID v4 for IDs
func generateUUID() string {
	return uuid.New().String()
}
```

Add `"github.com/google/uuid"` to the import block (if not already present). Remove `"time"` from imports if it becomes unused (check: `time.Now()` is used elsewhere in the file, so it stays).

Also remove `"fmt"` from imports if it becomes unused after this change. Check: `fmt.Errorf` and `fmt.Sprintf` are used extensively, so `"fmt"` stays.

**Step 3: Replace `generatePostgresUUID` in PostgreSQL impl**

In `backend/internal/db/postgres.go`, replace:

```go
// generatePostgresUUID creates a simple UUID for IDs
func generatePostgresUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}
```

with:

```go
// generatePostgresUUID creates a UUID v4 for IDs
func generatePostgresUUID() string {
	return uuid.New().String()
}
```

Add `"github.com/google/uuid"` to the import block (if not already present).

**Step 4: Run `go mod tidy`**

Run: `cd backend && go mod tidy`
Expected: No changes (google/uuid is already a dependency, used in `merge.go` and `goals.go`)

**Step 5: Run full test suite**

Run: `cd backend && go test ./...`
Expected: All PASS

**Step 6: Commit**

```bash
git add backend/internal/db/queries.go backend/internal/db/postgres.go
git commit -m "fix(db): replace weak UUID generation with google/uuid

generateUUID() and generatePostgresUUID() used time.Now().UnixNano()
which could produce duplicates under concurrent requests. Now uses
uuid.New().String() consistent with the rest of the codebase."
```

---

## Task 4: Make `CreateGoal` position assignment atomic (review issue #7)

**Severity:** Low

`CreateGoal` in both SQLite and PostgreSQL implementations uses two separate statements — `SELECT MAX(position)` then `INSERT` — outside a transaction. Under concurrent requests from the same user, two goals could get the same position.

The fix combines these into a single `INSERT ... SELECT` statement so the position is computed atomically. The inserted position is then read back to update the Go struct.

**Files:**
- Modify: `backend/internal/db/queries.go:125-156` (SQLite impl)
- Modify: `backend/internal/db/postgres.go:214-245` (PostgreSQL impl)
- Test: `backend/internal/db/queries_test.go`

**Step 1: Write a test that verifies positions are sequential**

Add to `backend/internal/db/queries_test.go`:

```go
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
```

**Step 2: Run the test to verify it passes (existing behavior works for sequential case)**

Run: `cd backend && go test ./internal/db/ -v -run TestCreateGoal_PositionsAreSequential`
Expected: PASS (the current code works for sequential creates; we're adding this to prevent regressions)

**Step 3: Refactor SQLite `CreateGoal` to use atomic INSERT**

In `backend/internal/db/queries.go`, replace `CreateGoal` (lines 125-156) with:

```go
func (d *SQLiteDB) CreateGoal(g *models.Goal) error {
	now := time.Now().UTC()
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}

	var position int
	if g.UserID == nil {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES (?, ?, ?, COALESCE((SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL), -1) + 1, ?, ?, ?, ?, ?)
			 RETURNING position`,
			g.ID, g.Name, g.Color, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	} else {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES (?, ?, ?, COALESCE((SELECT MAX(position) FROM goals WHERE user_id = ? AND deleted_at IS NULL), -1) + 1, ?, ?, ?, ?, ?)
			 RETURNING position`,
			g.ID, g.Name, g.Color, *g.UserID, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	}
	g.Position = position
	return nil
}
```

Note: `RETURNING` is supported in SQLite 3.35+ (2021-03-12). Go's `modernc.org/sqlite` and `mattn/go-sqlite3` both support this.

**Step 4: Refactor PostgreSQL `CreateGoal` similarly**

In `backend/internal/db/postgres.go`, replace `CreateGoal` (lines 214-245) with:

```go
func (d *PostgresDB) CreateGoal(g *models.Goal) error {
	now := time.Now().UTC()
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}

	var position int
	if g.UserID == nil {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES ($1, $2, $3, COALESCE((SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL), -1) + 1, $4, $5, $6, $7, $8)
			 RETURNING position`,
			g.ID, g.Name, g.Color, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	} else {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES ($1, $2, $3, COALESCE((SELECT MAX(position) FROM goals WHERE user_id = $4 AND deleted_at IS NULL), -1) + 1, $5, $6, $7, $8, $9)
			 RETURNING position`,
			g.ID, g.Name, g.Color, *g.UserID, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	}
	g.Position = position
	return nil
}
```

**Step 5: Run the tests**

Run: `cd backend && go test ./... -v`
Expected: All PASS, including `TestCreateGoal_PositionsAreSequential` and all existing tests that create goals

**Step 6: Commit**

```bash
git add backend/internal/db/queries.go backend/internal/db/postgres.go backend/internal/db/queries_test.go
git commit -m "fix(db): make CreateGoal position assignment atomic

SELECT MAX(position) and INSERT were separate statements, so concurrent
creates could produce duplicate positions. Now uses a single INSERT with
a subquery and RETURNING to compute position atomically."
```

---

## Task 5: Distinguish `archived` from `deleted` in sync protocol (review issue #9)

**Severity:** Low

Currently, the client sends `deleted: !!goal.archived_at` (sync.ts:161), so archiving a goal on device A sends `deleted: true` to the server, which sets `DeletedAt` (not `ArchivedAt`). The server then broadcasts `deleted: true` to other devices, which set `archived_at`. The round-trip works but the server DB loses the archive/delete distinction, preventing future "unarchive from another device" support.

The fix adds an `archived` boolean to the sync wire format, separate from `deleted`.

**Files:**
- Modify: `backend/internal/sync/types.go` (add `Archived` field to `GoalChange`)
- Modify: `backend/internal/sync/merge.go` (handle `Archived` in `MergeGoal` and `GoalToChange`)
- Modify: `frontend/src/lib/sync.ts` (send/receive `archived` separately)
- Test: `backend/internal/sync/merge_test.go`
- Test: `backend/internal/api/api_test.go`

### Part A: Backend changes

**Step 1: Write failing test — archived goal syncs with `ArchivedAt` preserved**

Add to `backend/internal/sync/merge_test.go`:

```go
func TestMergeGoal_ArchivedClientGoal_SetsArchivedAt(t *testing.T) {
	now := time.Now().UTC()

	clientChange := sync.GoalChange{
		ID:        "goal-1",
		Name:      "Read",
		Color:     "#FF0000",
		Position:  1,
		UpdatedAt: now,
		Deleted:   false,
		Archived:  true,
	}

	mergedGoal, shouldApply := sync.MergeGoal(clientChange, nil)
	if !shouldApply {
		t.Fatal("expected shouldApply to be true for new archived goal")
	}
	if mergedGoal.ArchivedAt == nil {
		t.Fatal("expected ArchivedAt to be set for archived goal")
	}
	if mergedGoal.DeletedAt != nil {
		t.Fatal("expected DeletedAt to be nil for archived (not deleted) goal")
	}
}
```

**Step 2: Run the test to verify it fails**

Run: `cd backend && go test ./internal/sync/ -v -run TestMergeGoal_ArchivedClientGoal`
Expected: FAIL — `GoalChange` has no `Archived` field yet

**Step 3: Add `Archived` field to `GoalChange`**

In `backend/internal/sync/types.go`, add `Archived` to `GoalChange`:

```go
type GoalChange struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Color        string    `json:"color"`
	Position     int       `json:"position"`
	TargetCount  *int      `json:"target_count,omitempty"`
	TargetPeriod *string   `json:"target_period,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	Deleted      bool      `json:"deleted"`
	Archived     bool      `json:"archived"`
}
```

**Step 4: Update `MergeGoal` to handle `Archived`**

In `backend/internal/sync/merge.go`, update the new-goal branch (around line 14-29):

Replace:
```go
	if serverGoal == nil {
		now := time.Now().UTC()
		goal := &models.Goal{
			ID:           clientChange.ID,
			Name:         clientChange.Name,
			Color:        clientChange.Color,
			Position:     clientChange.Position,
			TargetCount:  clientChange.TargetCount,
			TargetPeriod: clientChange.TargetPeriod,
			UpdatedAt:    clientChange.UpdatedAt,
			CreatedAt:    now,
		}
		if clientChange.Deleted {
			goal.DeletedAt = &clientChange.UpdatedAt
		}
		return goal, true
	}
```

With:
```go
	if serverGoal == nil {
		now := time.Now().UTC()
		goal := &models.Goal{
			ID:           clientChange.ID,
			Name:         clientChange.Name,
			Color:        clientChange.Color,
			Position:     clientChange.Position,
			TargetCount:  clientChange.TargetCount,
			TargetPeriod: clientChange.TargetPeriod,
			UpdatedAt:    clientChange.UpdatedAt,
			CreatedAt:    now,
		}
		if clientChange.Deleted {
			goal.DeletedAt = &clientChange.UpdatedAt
		}
		if clientChange.Archived {
			goal.ArchivedAt = &clientChange.UpdatedAt
		}
		return goal, true
	}
```

Update the client-wins branch (around line 33-46):

Replace:
```go
		serverGoal.UpdatedAt = clientChange.UpdatedAt
		if clientChange.Deleted {
			serverGoal.DeletedAt = &clientChange.UpdatedAt
		} else {
			serverGoal.DeletedAt = nil
		}
		return serverGoal, true
```

With:
```go
		serverGoal.UpdatedAt = clientChange.UpdatedAt
		if clientChange.Deleted {
			serverGoal.DeletedAt = &clientChange.UpdatedAt
		} else {
			serverGoal.DeletedAt = nil
		}
		if clientChange.Archived {
			serverGoal.ArchivedAt = &clientChange.UpdatedAt
		} else {
			serverGoal.ArchivedAt = nil
		}
		return serverGoal, true
```

**Step 5: Update `GoalToChange` to include `Archived`**

In `backend/internal/sync/merge.go`, update `GoalToChange` (around line 99-111):

Replace:
```go
func GoalToChange(goal *models.Goal) GoalChange {
	return GoalChange{
		ID:           goal.ID,
		Name:         goal.Name,
		Color:        goal.Color,
		Position:     goal.Position,
		TargetCount:  goal.TargetCount,
		TargetPeriod: goal.TargetPeriod,
		UpdatedAt:    goal.UpdatedAt,
		Deleted:      goal.DeletedAt != nil,
	}
}
```

With:
```go
func GoalToChange(goal *models.Goal) GoalChange {
	return GoalChange{
		ID:           goal.ID,
		Name:         goal.Name,
		Color:        goal.Color,
		Position:     goal.Position,
		TargetCount:  goal.TargetCount,
		TargetPeriod: goal.TargetPeriod,
		UpdatedAt:    goal.UpdatedAt,
		Deleted:      goal.DeletedAt != nil,
		Archived:     goal.ArchivedAt != nil,
	}
}
```

**Step 6: Run the sync tests**

Run: `cd backend && go test ./internal/sync/ -v`
Expected: All PASS, including the new `TestMergeGoal_ArchivedClientGoal_SetsArchivedAt`

**Step 7: Write integration test — archived goal round-trips through sync**

Add to `backend/internal/api/api_test.go`:

```go
func TestSync_ArchivedGoalPreservesArchivedAt(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "sync-archive@test.com")

	now := time.Now().UTC()

	// Send an archived goal via sync
	syncBody, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals": []map[string]interface{}{
			{
				"id": "archived-goal-1", "name": "Old Habit", "color": "#FF0000",
				"position": 1, "updated_at": now.Format(time.RFC3339Nano),
				"deleted": false, "archived": true,
			},
		},
		"completions": []interface{}{},
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync failed: %d %s", w.Code, w.Body.String())
	}

	// Fetch goals including archived — the goal should be archived, not deleted
	listReq := httptest.NewRequest("GET", "/api/v1/goals?include_archived=true", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list goals failed: %d %s", listW.Code, listW.Body.String())
	}

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)

	found := false
	for _, g := range goals {
		if g.ID == "archived-goal-1" {
			found = true
			if g.ArchivedAt == nil {
				t.Error("expected ArchivedAt to be set")
			}
			if g.DeletedAt != nil {
				t.Error("expected DeletedAt to be nil (archived, not deleted)")
			}
		}
	}
	if !found {
		t.Error("archived goal not found in list (may have been treated as deleted)")
	}
}
```

**Step 8: Run integration test**

Run: `cd backend && go test ./internal/api/ -v -run TestSync_ArchivedGoal`
Expected: PASS

### Part B: Frontend changes

**Step 9: Update client sync upload — send `archived` separately**

In `frontend/src/lib/sync.ts`, update the `GoalChange` interface (around line 30-39):

Add `archived` field:
```typescript
interface GoalChange {
  id: string;
  name: string;
  color: string;
  position: number;
  target_count?: number;
  target_period?: 'week' | 'month';
  updated_at: string;
  deleted: boolean;
  archived: boolean;
}
```

**Step 10: Update goal change construction — use `archived` field instead of overloading `deleted`**

In `frontend/src/lib/sync.ts`, in the `create_goal`/`update_goal` branch (around line 149-163), change:

```typescript
            goalChanges.push({
              id: goal.id,
              name: goal.name,
              color: goal.color,
              position: goal.position,
              target_count: goal.target_count,
              target_period: goal.target_period,
              updated_at: op.timestamp,
              deleted: false,
              archived: !!goal.archived_at,
            });
```

(Changed `deleted: !!goal.archived_at` to `deleted: false, archived: !!goal.archived_at` — create/update is not a delete.)

In the `delete_goal` branch (around line 164-179), change to:

```typescript
            goalChanges.push({
              id: goal.id,
              name: goal.name,
              color: goal.color,
              position: goal.position,
              target_count: goal.target_count,
              target_period: goal.target_period,
              updated_at: op.timestamp,
              deleted: true,
              archived: false,
            });
```

In the `reorder_goals` branch (around line 200-219), change to include `archived`:

```typescript
              goalChanges.push({
                id: goal.id,
                name: goal.name,
                color: goal.color,
                position: i + 1,
                target_count: goal.target_count,
                target_period: goal.target_period,
                updated_at: op.timestamp,
                deleted: false,
                archived: !!goal.archived_at,
              });
```

**Step 11: Update `applyServerChanges` — handle `archived` from server**

In `frontend/src/lib/sync.ts`, update `applyServerChanges` (around line 279-307). Replace the goal loop:

```typescript
    for (const goalChange of response.goals ?? []) {
      const goal: Goal = {
        id: goalChange.id,
        name: goalChange.name,
        color: goalChange.color,
        position: goalChange.position,
        target_count: goalChange.target_count,
        target_period: goalChange.target_period,
        created_at: goalChange.updated_at,
        archived_at: goalChange.archived ? goalChange.updated_at : undefined,
      };
      if (!goalChange.deleted) {
        await saveLocalGoal(goal);
      }
      // For truly deleted goals, we don't save them locally at all
      // (previously we were saving them with archived_at, conflating the two)
    }
```

Note: Goals with `deleted: true` are no longer saved locally. Goals with `archived: true` get `archived_at` set. This is a behavioral change — previously deleted goals lingered as "archived." If the UI already filters out archived goals from the active list, this is safe. If deleted goals need to remain for local display, keep the existing save behavior but distinguish the timestamp source.

**Important consideration:** Verify that the app doesn't need to display deleted goals locally (e.g., for offline undo). If it does, keep saving them but with a separate `deleted_at` field on the local `Goal` type. For this plan, we assume deleted goals should not persist locally since they are deleted.

**Step 12: Run frontend tests**

Run: `cd frontend && npx vitest run`
Expected: All PASS. If any sync tests reference the old `deleted: !!goal.archived_at` pattern, update them to use the new `deleted: false, archived: !!goal.archived_at` pattern.

**Step 13: Run full backend test suite**

Run: `cd backend && go test ./...`
Expected: All PASS

**Step 14: Commit**

```bash
git add backend/internal/sync/types.go backend/internal/sync/merge.go backend/internal/sync/merge_test.go backend/internal/api/api_test.go frontend/src/lib/sync.ts
git commit -m "fix(sync): distinguish archived from deleted in sync protocol

Previously, archived goals were sent as deleted:true, causing the server
to set DeletedAt instead of ArchivedAt. Adds an 'archived' boolean to
the GoalChange wire format so both states are preserved independently."
```

---

## Task 6: Use one-time auth code for mobile OAuth (review issue #8)

**Severity:** Medium (security improvement, not blocking launch per review)

The mobile OAuth callback redirects to `goaltracker://auth?token=<session_token>`, exposing the raw token in the URL where it can appear in system logs, browser history, or be intercepted. The standard mitigation is a one-time authorization code exchanged for a token.

**Files:**
- Create: `backend/internal/auth/authcode.go` (in-memory code store with TTL)
- Modify: `backend/internal/api/auth.go:59-65` (use auth code in redirect)
- Modify: `backend/internal/api/server.go` (register exchange endpoint)
- Test: `backend/internal/auth/authcode_test.go`
- Test: `backend/internal/api/api_test.go`

### Part A: Auth code store

**Step 1: Write failing test — store and exchange an auth code**

Create `backend/internal/auth/authcode_test.go`:

```go
package auth

import (
	"testing"
	"time"
)

func TestAuthCodeStore_StoreAndExchange(t *testing.T) {
	store := NewAuthCodeStore(30 * time.Second)

	code := store.Generate("session-token-123")

	token, ok := store.Exchange(code)
	if !ok {
		t.Fatal("expected exchange to succeed")
	}
	if token != "session-token-123" {
		t.Errorf("expected session-token-123, got %s", token)
	}

	// Second exchange should fail (one-time use)
	_, ok = store.Exchange(code)
	if ok {
		t.Fatal("expected second exchange to fail (one-time use)")
	}
}

func TestAuthCodeStore_ExpiredCode(t *testing.T) {
	store := NewAuthCodeStore(1 * time.Millisecond)

	code := store.Generate("session-token-456")

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	_, ok := store.Exchange(code)
	if ok {
		t.Fatal("expected exchange to fail for expired code")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -v -run TestAuthCodeStore`
Expected: FAIL — `NewAuthCodeStore` does not exist yet

**Step 3: Implement the auth code store**

Create `backend/internal/auth/authcode.go`:

```go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type authCodeEntry struct {
	sessionToken string
	expiresAt    time.Time
}

// AuthCodeStore provides short-lived, one-time-use authorization codes
// that can be exchanged for session tokens. Codes are stored in memory
// since they only need to survive the few seconds between OAuth redirect
// and the mobile app's exchange request.
type AuthCodeStore struct {
	mu      sync.Mutex
	codes   map[string]authCodeEntry
	ttl     time.Duration
}

func NewAuthCodeStore(ttl time.Duration) *AuthCodeStore {
	return &AuthCodeStore{
		codes: make(map[string]authCodeEntry),
		ttl:   ttl,
	}
}

// Generate creates a new auth code for the given session token.
func (s *AuthCodeStore) Generate(sessionToken string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	code := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up expired codes opportunistically
	now := time.Now()
	for k, v := range s.codes {
		if now.After(v.expiresAt) {
			delete(s.codes, k)
		}
	}

	s.codes[code] = authCodeEntry{
		sessionToken: sessionToken,
		expiresAt:    now.Add(s.ttl),
	}
	return code
}

// Exchange validates and consumes a one-time auth code, returning the
// associated session token. Returns ("", false) if the code is invalid,
// expired, or already used.
func (s *AuthCodeStore) Exchange(code string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.codes[code]
	if !ok {
		return "", false
	}
	delete(s.codes, code) // One-time use

	if time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.sessionToken, true
}
```

**Step 4: Run the auth code tests**

Run: `cd backend && go test ./internal/auth/ -v -run TestAuthCodeStore`
Expected: Both tests PASS

**Step 5: Commit the auth code store**

```bash
git add backend/internal/auth/authcode.go backend/internal/auth/authcode_test.go
git commit -m "feat(auth): add one-time auth code store for mobile OAuth

In-memory store with configurable TTL. Codes are cryptographically
random and single-use."
```

### Part B: Wire into the API

**Step 6: Add `AuthCodeStore` to the `Server` struct**

Check `backend/internal/api/server.go` for the `Server` struct definition and its constructor. Add the `AuthCodeStore` field and initialize it in `NewServer`.

In the `Server` struct, add:
```go
	authCodeStore *auth.AuthCodeStore
```

In `NewServer`, initialize:
```go
	authCodeStore: auth.NewAuthCodeStore(30 * time.Second),
```

**Step 7: Update mobile OAuth callback to use auth code**

In `backend/internal/api/auth.go`, replace lines 59-65:

```go
	// Handle mobile OAuth callback - redirect with one-time auth code
	if result.IsMobile {
		code := s.authCodeStore.Generate(result.SessionToken)
		redirectURL := "goaltracker://auth?code=" + code
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
```

**Step 8: Add the exchange endpoint handler**

In `backend/internal/api/auth.go`, add:

```go
// exchangeAuthCode exchanges a one-time auth code for a session token.
// Used by the mobile app after OAuth redirect.
func (s *Server) exchangeAuthCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	sessionToken, ok := s.authCodeStore.Exchange(body.Code)
	if !ok {
		http.Error(w, "invalid or expired code", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"session_token": sessionToken,
	})
}
```

**Step 9: Register the exchange endpoint**

In `backend/internal/api/server.go`, find the route registration and add (in the public/unauthenticated group since the mobile app doesn't have a session yet):

```go
	r.Post("/api/v1/auth/exchange", s.exchangeAuthCode)
```

**Step 10: Write integration test**

Add to `backend/internal/api/api_test.go`:

```go
func TestAuthCodeExchange(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Generate an auth code directly (simulating what oauthCallback would do)
	code := server.AuthCodeStore().Generate("test-session-token")

	// Exchange it
	body := bytes.NewBufferString(`{"code":"` + code + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/exchange", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["session_token"] != "test-session-token" {
		t.Errorf("expected test-session-token, got %s", resp["session_token"])
	}

	// Second exchange should fail
	body2 := bytes.NewBufferString(`{"code":"` + code + `"}`)
	req2 := httptest.NewRequest("POST", "/api/v1/auth/exchange", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for reused code, got %d", w2.Code)
	}
}
```

Note: This test needs the server to expose `AuthCodeStore()` as a public method. Add to `server.go`:

```go
func (s *Server) AuthCodeStore() *auth.AuthCodeStore {
	return s.authCodeStore
}
```

**Step 11: Run the test**

Run: `cd backend && go test ./internal/api/ -v -run TestAuthCodeExchange`
Expected: PASS

**Step 12: Run full test suite**

Run: `cd backend && go test ./...`
Expected: All PASS

**Step 13: Commit**

```bash
git add backend/internal/api/auth.go backend/internal/api/server.go backend/internal/api/api_test.go
git commit -m "feat(auth): use one-time auth code for mobile OAuth redirect

The mobile OAuth callback now redirects with a short-lived auth code
instead of the raw session token. The mobile app exchanges the code for
a token via POST /api/v1/auth/exchange. This prevents the session token
from appearing in system logs, browser history, or referrer headers."
```

**Step 14 (post-merge note): Update mobile app**

The Android app needs to be updated to call `POST /api/v1/auth/exchange` with the received code instead of using the token directly from the URL. This is a client-side change in the Android project, not covered in this plan. The old `?token=` parameter is no longer sent, so **this is a breaking change for the mobile client**. Coordinate deployment: ship the backend change and the app update together, or add temporary backward compatibility by supporting both `?token=` and `?code=` during a transition period.

---

## Validation Checklist

After all tasks are complete:

1. **Backend unit tests:** `cd backend && go test ./internal/auth/ -v` — all pass
2. **Backend integration tests:** `cd backend && go test ./internal/api/ -v` — all pass
3. **Full backend suite:** `cd backend && go test ./...` — all pass
4. **Frontend tests:** `cd frontend && npx vitest run` — all pass
5. **Manual spot-checks:**
   - `PATCH /api/v1/goals/:id` with `{"name": ""}` → 400
   - `POST /api/v1/goals` with `{"name": "X", "target_period": "year"}` → 400
   - Create 3 goals rapidly → positions are 0, 1, 2 (no duplicates)

## Task Dependency Graph

```
Task 1 (empty name) ─────────┐
Task 2 (target_period) ──────┤
Task 3 (UUID generation) ────┼──→ Validation
Task 4 (atomic position) ────┤
Task 5 (archived vs deleted) ┤
Task 6A (auth code store) ───┤
         │                    │
Task 6B (wire into API) ─────┘
```

Tasks 1-5 and 6A are all independent of each other and can run in parallel.
Task 6B depends on Task 6A.
