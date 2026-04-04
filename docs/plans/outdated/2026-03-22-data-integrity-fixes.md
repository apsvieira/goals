# Data Integrity Fixes Implementation Plan

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the hard-delete/soft-delete inconsistency in completions, implement `GetCompletionByGoalAndDateIncludingDeleted` for sync, add ownership parameter to `SoftDeleteCompletion`, and fix the swallowed error in `CreateGoal`.

**Architecture:** Changes span the DB interface, both SQLite and PostgreSQL implementations, the sync service, and the REST API completion handler. The DB interface gets one new method and one signature change. Both implementations (sqlite `queries.go` and `postgres.go`) must be updated in lockstep.

**Tech Stack:** Go, SQLite, PostgreSQL, database/sql

---

### Task 1: Convert `DeleteCompletion` from hard delete to soft delete

The REST `DELETE /completions/{id}` endpoint calls `DeleteCompletion` which runs `DELETE FROM completions`. Every other delete in the app uses soft deletes (`deleted_at` timestamp). This breaks sync — once hard-deleted, the completion can't be found by the sync engine.

**Files:**
- Modify: `backend/internal/db/queries.go:347-353` (SQLite `DeleteCompletion`)
- Modify: `backend/internal/db/postgres.go:472-478` (PostgreSQL `DeleteCompletion`)
- Test: `backend/internal/api/api_test.go`

**Step 1: Write the failing test**

The existing `TestDeleteCompletion` verifies the completion disappears from the list. It will still pass after our change since `ListCompletions` filters `deleted_at IS NULL`. But we need a new test to verify the record still exists in the DB (soft-deleted).

Add to `backend/internal/api/api_test.go`:

```go
func TestDeleteCompletion_IsSoftDelete(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "test@localhost")

	// Create a goal
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	var goal models.Goal
	json.NewDecoder(createW.Body).Decode(&goal)

	// Create a completion
	compBody := bytes.NewBufferString(`{"goal_id": "` + goal.ID + `", "date": "2026-01-05"}`)
	compReq := httptest.NewRequest("POST", "/api/v1/completions", compBody)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.AddCookie(cookie)
	compW := httptest.NewRecorder()
	server.ServeHTTP(compW, compReq)

	var completion models.Completion
	json.NewDecoder(compW.Body).Decode(&completion)

	// Delete the completion
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/completions/"+completion.ID, nil)
	deleteReq.AddCookie(cookie)
	deleteW := httptest.NewRecorder()
	server.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteW.Code)
	}

	// The completion should NOT appear in the list (filtered by deleted_at IS NULL)
	listReq := httptest.NewRequest("GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	var completions []models.Completion
	json.NewDecoder(listW.Body).Decode(&completions)

	if len(completions) != 0 {
		t.Errorf("expected 0 visible completions, got %d", len(completions))
	}

	// But re-creating the same completion should succeed (idempotent-ish)
	// This verifies the soft-deleted record doesn't block a new one
	compBody2 := bytes.NewBufferString(`{"goal_id": "` + goal.ID + `", "date": "2026-01-05"}`)
	compReq2 := httptest.NewRequest("POST", "/api/v1/completions", compBody2)
	compReq2.Header.Set("Content-Type", "application/json")
	compReq2.AddCookie(cookie)
	compW2 := httptest.NewRecorder()
	server.ServeHTTP(compW2, compReq2)

	// Should succeed — either 200 (found existing) or 201 (created new)
	if compW2.Code != http.StatusOK && compW2.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201 after re-creating deleted completion, got %d: %s", compW2.Code, compW2.Body.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/api/ -run TestDeleteCompletion_IsSoftDelete -v`
Expected: FAIL — re-creating after hard delete returns 200 with the old record's ID (since `GetCompletionByGoalAndDate` won't find it after hard delete, it'll create a new one, which actually returns 201. But if there's a unique constraint `(goal_id, date)` conflict from the hard-deleted row being gone, it will just work. The main thing to verify is the test establishes baseline behavior.)

**Step 3: Change `DeleteCompletion` to soft delete in both implementations**

In `backend/internal/db/queries.go`, replace lines 347-353:

```go
func (d *SQLiteDB) DeleteCompletion(id string) error {
	now := time.Now().UTC()
	_, err := d.Exec(
		`UPDATE completions SET deleted_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}
```

In `backend/internal/db/postgres.go`, replace lines 472-478:

```go
func (d *PostgresDB) DeleteCompletion(id string) error {
	now := time.Now().UTC()
	_, err := d.Exec(
		`UPDATE completions SET deleted_at = $1, updated_at = $2 WHERE id = $3`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}
```

**Step 4: Handle the `createCompletion` idempotency check**

Now that `DeleteCompletion` is a soft delete, `GetCompletionByGoalAndDate` (which filters `deleted_at IS NULL`) won't find the soft-deleted record, so `createCompletion` will try to INSERT a new row — but the `UNIQUE(goal_id, date)` constraint will fail because the old row is still there.

Fix: update `createCompletion` in `backend/internal/api/completions.go` to handle this. After the existing idempotency check (line 93-102), if the INSERT fails with a unique constraint violation, "un-delete" the existing soft-deleted record instead.

Alternatively — simpler approach — change `GetCompletionByGoalAndDate` to find soft-deleted records too, or add an UPSERT. The simplest fix: modify `CreateCompletion` in both DB implementations to use `INSERT ... ON CONFLICT (goal_id, date) DO UPDATE SET deleted_at = NULL, updated_at = ?`.

In `backend/internal/db/queries.go`, replace the `CreateCompletion` function:

```go
func (d *SQLiteDB) CreateCompletion(c *models.Completion) error {
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().UTC()
	}

	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (goal_id, date) DO UPDATE SET deleted_at = NULL, updated_at = excluded.updated_at`,
		c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}
```

Do the same in `backend/internal/db/postgres.go` with `$1`-style placeholders:

```go
func (d *PostgresDB) CreateCompletion(c *models.Completion) error {
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().UTC()
	}

	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (goal_id, date) DO UPDATE SET deleted_at = NULL, updated_at = $5`,
		c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}
```

**Step 5: Run all tests**

Run: `cd backend && go test ./internal/api/ -v`
Expected: all PASS

**Step 6: Commit**

```
git add backend/internal/db/queries.go backend/internal/db/postgres.go backend/internal/api/api_test.go
git commit -m "fix(db): convert DeleteCompletion to soft delete for sync consistency"
```

---

### Task 2: Add `GetCompletionByGoalAndDateIncludingDeleted` for sync

The sync engine's `getCompletionIncludingDeleted` calls `GetCompletionByGoalAndDate`, which filters `deleted_at IS NULL`. This means the sync engine can't see soft-deleted completions, breaking conflict resolution.

**Files:**
- Modify: `backend/internal/db/interface.go:25` (add new method)
- Modify: `backend/internal/db/queries.go` (SQLite implementation)
- Modify: `backend/internal/db/postgres.go` (PostgreSQL implementation)
- Modify: `backend/internal/sync/sync.go:159-166` (use the new method)

**Step 1: Add the method to the interface**

In `backend/internal/db/interface.go`, add after line 25 (`GetCompletionByGoalAndDate`):

```go
GetCompletionByGoalAndDateIncludingDeleted(goalID, date string) (*models.Completion, error)
```

**Step 2: Implement in SQLite (`queries.go`)**

Find `GetCompletionByGoalAndDate` in `queries.go` and add a new function after it:

```go
func (d *SQLiteDB) GetCompletionByGoalAndDateIncludingDeleted(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	var deletedAt sql.NullTime
	var updatedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE goal_id = ? AND date = ?`,
		goalID, date,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion including deleted: %w", err)
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time
	}
	if deletedAt.Valid {
		c.DeletedAt = &deletedAt.Time
	}
	return &c, nil
}
```

**Step 3: Implement in PostgreSQL (`postgres.go`)**

Same logic with `$1`/`$2` placeholders:

```go
func (d *PostgresDB) GetCompletionByGoalAndDateIncludingDeleted(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	var deletedAt sql.NullTime
	var updatedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE goal_id = $1 AND date = $2`,
		goalID, date,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion including deleted: %w", err)
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time
	}
	if deletedAt.Valid {
		c.DeletedAt = &deletedAt.Time
	}
	return &c, nil
}
```

**Step 4: Update sync to use the new method**

In `backend/internal/sync/sync.go`, replace lines 159-166:

```go
func (s *Service) getCompletionIncludingDeleted(goalID, date string) (*models.Completion, error) {
	return s.db.GetCompletionByGoalAndDateIncludingDeleted(goalID, date)
}
```

**Step 5: Run tests**

Run: `cd backend && go test ./... -v`
Expected: all PASS

**Step 6: Commit**

```
git add backend/internal/db/interface.go backend/internal/db/queries.go backend/internal/db/postgres.go backend/internal/sync/sync.go
git commit -m "fix(sync): add GetCompletionByGoalAndDateIncludingDeleted for proper conflict resolution"
```

---

### Task 3: Add `userID` parameter to `SoftDeleteCompletion`

`SoftDeleteCompletion(goalID, date)` lacks a user ownership check. The sync code verifies ownership before calling it, but the DB method itself doesn't enforce it. Adding `userID` makes it defense-in-depth consistent with `SoftDeleteGoal`.

**Files:**
- Modify: `backend/internal/db/interface.go:48` (change signature)
- Modify: `backend/internal/db/queries.go:803-813` (SQLite)
- Modify: `backend/internal/db/postgres.go:907-917` (PostgreSQL)
- Modify: all callers of `SoftDeleteCompletion` (check with grep)

**Step 1: Find all callers**

Run: `grep -rn "SoftDeleteCompletion" backend/`

Expected callers: `interface.go`, `queries.go`, `postgres.go`, and any sync code that calls it. Currently `SoftDeleteCompletion` is in the interface but not directly called by sync — the sync uses `UpsertCompletion` with `DeletedAt` set. Verify before changing.

If no callers exist outside the DB package, the interface change is safe. If callers exist, update them too.

**Step 2: Update the interface**

In `backend/internal/db/interface.go`, change line 48:

```go
SoftDeleteCompletion(userID *string, goalID, date string) error
```

**Step 3: Update SQLite implementation**

In `backend/internal/db/queries.go`, replace `SoftDeleteCompletion`:

```go
func (d *SQLiteDB) SoftDeleteCompletion(userID *string, goalID, date string) error {
	now := time.Now().UTC()
	query := `UPDATE completions SET deleted_at = ?, updated_at = ?
		WHERE goal_id = ? AND date = ?
		AND goal_id IN (SELECT id FROM goals WHERE `
	args := []any{now, now, goalID, date}

	if userID == nil {
		query += `user_id IS NULL)`
	} else {
		query += `user_id = ?)`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}
```

**Step 4: Update PostgreSQL implementation**

In `backend/internal/db/postgres.go`, replace `SoftDeleteCompletion`:

```go
func (d *PostgresDB) SoftDeleteCompletion(userID *string, goalID, date string) error {
	now := time.Now().UTC()
	query := `UPDATE completions SET deleted_at = $1, updated_at = $2
		WHERE goal_id = $3 AND date = $4
		AND goal_id IN (SELECT id FROM goals WHERE `
	args := []any{now, now, goalID, date}

	if userID == nil {
		query += `user_id IS NULL)`
	} else {
		query += fmt.Sprintf(`user_id = $%d)`, len(args)+1)
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}
```

**Step 5: Update any callers**

Check `grep -rn "SoftDeleteCompletion" backend/` and update any call sites to pass the `userID` parameter.

**Step 6: Run tests**

Run: `cd backend && go test ./... -v`
Expected: all PASS (compile + tests)

**Step 7: Commit**

```
git add backend/internal/db/
git commit -m "fix(db): add userID ownership check to SoftDeleteCompletion"
```

---

### Task 4: Fix swallowed error in `CreateGoal` position calculation

`CreateGoal` ignores the error from `QueryRow(...).Scan(&maxPos)` when computing the next position. If the query fails, the goal silently gets position 0.

**Files:**
- Modify: `backend/internal/db/queries.go:125-132` (SQLite)
- Modify: `backend/internal/db/postgres.go:214-221` (PostgreSQL)

**Step 1: Fix SQLite implementation**

In `backend/internal/db/queries.go`, replace lines 127-132:

```go
	var maxPos sql.NullInt64
	var err error
	if g.UserID == nil {
		err = d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL`).Scan(&maxPos)
	} else {
		err = d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id = ? AND deleted_at IS NULL`, *g.UserID).Scan(&maxPos)
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("get max position: %w", err)
	}
```

Note: `MAX()` returns NULL (not no rows) when the table is empty, so `sql.ErrNoRows` shouldn't normally happen here, but we handle it defensively.

**Step 2: Fix PostgreSQL implementation**

In `backend/internal/db/postgres.go`, apply the same pattern (with `$1` placeholder).

**Step 3: Run tests**

Run: `cd backend && go test ./... -v`
Expected: all PASS

**Step 4: Commit**

```
git add backend/internal/db/queries.go backend/internal/db/postgres.go
git commit -m "fix(db): handle error in CreateGoal position calculation"
```
