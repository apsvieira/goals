# Pre-Launch Bugfixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all blocking and high-severity issues from the pre-launch code review before Play Store release.

**Architecture:** Three independent fixes — a frontend sync bug, a backend validation gap, and missing test coverage for account deletion and sync round-trips. The backend fixes are in Go (tested with `go test`), the frontend fix is in TypeScript (tested with `vitest`).

**Tech Stack:** Go 1.22+, TypeScript/Svelte, SQLite (test), PostgreSQL (prod), vitest, `net/http/httptest`

**Review reference:** `~/agent-notes/code-reviews/2026-03/goal-tracker-pre-launch-review.md`

---

## Context: Review Issue #1 Downgraded

The review flags `DeleteAccount` missing an explicit `DELETE FROM auth_providers` as Critical. However, `auth_providers.user_id` has `ON DELETE CASCADE` (`migrations/003_add_users.sql:12`) and SQLite enables FK enforcement (`sqlite.go:41`). Deleting the `users` row cascades to `auth_providers` automatically. This is **not a blocking bug**. An explicit delete is added in Task 1 for consistency with the other explicit deletes in the transaction, but it is cosmetic.

---

## Task 1: Fix `DeleteAccount` — add explicit `auth_providers` delete (cosmetic consistency)

**Files:**
- Modify: `backend/internal/db/queries.go:979-1007` (SQLite impl)
- Modify: `backend/internal/db/postgres.go:1083-1112` (Postgres impl)

**Step 1: Add `auth_providers` delete to SQLite impl**

In `backend/internal/db/queries.go`, inside `DeleteAccount`, add before `// Delete sessions`:

```go
	// Delete auth providers
	if _, err := tx.Exec(`DELETE FROM auth_providers WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete auth providers: %w", err)
	}
```

**Step 2: Add `auth_providers` delete to Postgres impl**

In `backend/internal/db/postgres.go`, inside `DeleteAccount`, add before `// Delete sessions`:

```go
	// Delete auth providers
	if _, err := tx.Exec(`DELETE FROM auth_providers WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete auth providers: %w", err)
	}
```

**Step 3: Run existing tests**

Run: `cd backend && go test ./internal/db/ -v -run TestSoftDelete`
Expected: PASS (no regressions)

**Step 4: Commit**

```bash
git add backend/internal/db/queries.go backend/internal/db/postgres.go
git commit -m "fix(db): add explicit auth_providers delete to DeleteAccount for consistency"
```

---

## Task 2: Add account deletion integration tests (review issue #5)

**Files:**
- Modify: `backend/internal/api/api_test.go`

These tests exercise `DELETE /api/v1/account` through the HTTP layer, covering the fix from Task 1 and the previously untested handler at `backend/internal/api/auth.go:96-119`.

**Step 1: Write failing test — account deletion succeeds and clears data**

Add to `backend/internal/api/api_test.go`:

```go
func TestDeleteAccount_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "delete-me@test.com")

	// Create a goal so there's data to delete
	createBody := bytes.NewBufferString(`{"name": "Exercise", "color": "#4CAF50"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup: failed to create goal: %d %s", createW.Code, createW.Body.String())
	}

	// Delete the account
	req := httptest.NewRequest("DELETE", "/api/v1/account", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "account deleted" {
		t.Errorf("expected status 'account deleted', got %q", resp["status"])
	}

	// Verify session is invalidated — goals request should fail
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after account deletion, got %d", listW.Code)
	}
}
```

**Step 2: Write failing test — unauthenticated deletion returns 401**

```go
func TestDeleteAccount_Unauthenticated(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/api/v1/account", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 3: Run the tests**

Run: `cd backend && go test ./internal/api/ -v -run TestDeleteAccount`
Expected: PASS (Task 1's fix ensures no FK errors)

**Step 4: Commit**

```bash
git add backend/internal/api/api_test.go
git commit -m "test(api): add integration tests for DELETE /api/v1/account"
```

---

## Task 3: Fix `reorder_goals` silently dropped during sync (review issue #2)

**Files:**
- Modify: `frontend/src/lib/sync.ts:148-200`
- Test: `frontend/src/lib/__tests__/sync.test.ts`

The `reorder_goals` operation falls through the if/else chain in `sync.ts` and is never converted to `GoalChange` entries. Since `GoalChange` already has a `position` field, the fix is to convert each reordered goal into a `GoalChange` with the updated position — no backend changes needed.

**Step 1: Write the failing test**

Add to `frontend/src/lib/__tests__/sync.test.ts`:

```typescript
import { initStorage, saveQueuedOperation, saveLocalGoal, getAllLocalGoals, clearQueuedOperations } from '../storage';

describe('SyncManager - reorder_goals', () => {
  beforeEach(async () => {
    await initStorage();
    await clearQueuedOperations();
  });

  it('should convert reorder_goals operations to goal changes with updated positions', async () => {
    // Save local goals with positions
    await saveLocalGoal({
      id: 'goal-a', name: 'A', color: '#FF0000', position: 1,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });
    await saveLocalGoal({
      id: 'goal-b', name: 'B', color: '#00FF00', position: 2,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });
    await saveLocalGoal({
      id: 'goal-c', name: 'C', color: '#0000FF', position: 3,
      target_count: 1, target_period: 'week', created_at: '2026-01-01T00:00:00Z',
    });

    // Queue a reorder that moves C to position 1: C, A, B
    await saveQueuedOperation({
      id: 'reorder-1',
      type: 'reorder_goals',
      entityId: 'reorder',
      payload: { goal_ids: ['goal-c', 'goal-a', 'goal-b'] },
      timestamp: '2026-01-02T00:00:00Z',
      retryCount: 0,
    });

    // Trigger sync — intercept the fetch call to capture the request body
    let capturedBody: any = null;
    globalThis.fetch = vi.fn().mockImplementation(async (_url: string, init: any) => {
      capturedBody = JSON.parse(init.body);
      return new Response(JSON.stringify({
        server_time: '2026-01-02T00:00:01Z',
        goals: [],
        completions: [],
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });

    await syncManager.sync();

    expect(capturedBody).not.toBeNull();
    expect(capturedBody.goals).toHaveLength(3);

    // Verify positions match the reorder: C=1, A=2, B=3
    const goalById = Object.fromEntries(capturedBody.goals.map((g: any) => [g.id, g]));
    expect(goalById['goal-c'].position).toBe(1);
    expect(goalById['goal-a'].position).toBe(2);
    expect(goalById['goal-b'].position).toBe(3);
  });
});
```

**Step 2: Run the test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/__tests__/sync.test.ts`
Expected: FAIL — `capturedBody.goals` will be empty (0 goals) because `reorder_goals` falls through.

**Step 3: Implement the fix**

In `frontend/src/lib/sync.ts`, inside the `for (const op of operations)` loop (around line 191, after the `delete_completion` block), add:

```typescript
        } else if (op.type === 'reorder_goals') {
          // Convert reorder to individual goal changes with updated positions
          const goals = await getAllLocalGoals();
          const orderedIds: string[] = op.payload.goal_ids;
          for (let i = 0; i < orderedIds.length; i++) {
            const goal = goals.find(g => g.id === orderedIds[i]);
            if (goal) {
              goalChanges.push({
                id: goal.id,
                name: goal.name,
                color: goal.color,
                position: i + 1,
                target_count: goal.target_count,
                target_period: goal.target_period as 'week' | 'month' | undefined,
                updated_at: op.timestamp,
                deleted: !!goal.archived_at,
              });
            }
          }
```

Note: positions use `i + 1` to match the 1-indexed convention in `api.ts:282`.

**Step 4: Deduplicate goal changes**

A subtle issue: if the user reorders AND updates a goal before sync, the same goal ID could appear twice in `goalChanges`. The server will process both, but the order is undefined. Add deduplication after the loop, replacing duplicates with the latest entry:

In `sync.ts`, after the `for (const op of operations)` loop closes (around line 201) and before `// Send to server`:

```typescript
      // Deduplicate goal changes — keep last entry per ID (later ops win)
      const goalChangeMap = new Map<string, GoalChange>();
      for (const change of goalChanges) {
        goalChangeMap.set(change.id, change);
      }
      const deduplicatedGoalChanges = Array.from(goalChangeMap.values());
```

Then update the `SyncRequest` construction to use `deduplicatedGoalChanges`:

```typescript
      const req: SyncRequest = {
        last_synced_at: this.lastSyncedAt?.toISOString() ?? null,
        goals: deduplicatedGoalChanges,
        completions: completionChanges,
      };
```

**Step 5: Run the test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/__tests__/sync.test.ts`
Expected: PASS

**Step 6: Run full frontend test suite**

Run: `cd frontend && npx vitest run`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add frontend/src/lib/sync.ts frontend/src/lib/__tests__/sync.test.ts
git commit -m "fix(sync): convert reorder_goals operations to goal changes during sync

Previously, reorder_goals operations were queued but fell through the
sync conversion loop, silently discarding position changes. Now they are
converted to individual GoalChange entries with updated positions."
```

---

## Task 4: Add sync round-trip integration test (review issue #4)

**Files:**
- Modify: `backend/internal/api/api_test.go`

**Step 1: Write the test — sync creates goals and returns them**

```go
func TestSync_RoundTrip_GoalsAndCompletions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "sync@test.com")

	now := time.Now().UTC()

	// Send goals and completions via sync
	syncBody, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": nil,
		"goals": []map[string]interface{}{
			{
				"id": "sync-goal-1", "name": "Read", "color": "#FF0000",
				"position": 1, "updated_at": now.Format(time.RFC3339Nano), "deleted": false,
			},
			{
				"id": "sync-goal-2", "name": "Write", "color": "#00FF00",
				"position": 2, "updated_at": now.Format(time.RFC3339Nano), "deleted": false,
			},
		},
		"completions": []map[string]interface{}{
			{
				"goal_id": "sync-goal-1", "date": "2026-03-28",
				"completed": true, "updated_at": now.Format(time.RFC3339Nano),
			},
		},
	})

	req := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync request failed: %d %s", w.Code, w.Body.String())
	}

	var syncResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&syncResp)

	// Verify server_time is present
	if _, ok := syncResp["server_time"]; !ok {
		t.Error("response missing server_time")
	}

	// Verify goals are now accessible via REST API
	listReq := httptest.NewRequest("GET", "/api/v1/goals", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	server.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list goals failed: %d %s", listW.Code, listW.Body.String())
	}

	var goals []models.Goal
	json.NewDecoder(listW.Body).Decode(&goals)
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(goals))
	}

	// Verify second sync with last_synced_at returns changes from other devices
	serverTime := syncResp["server_time"].(string)

	// Simulate a change from "another device" — create a goal via REST
	createBody := bytes.NewBufferString(`{"name": "Meditate", "color": "#0000FF"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/goals", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create goal failed: %d %s", createW.Code, createW.Body.String())
	}

	// Sync again with last_synced_at — should receive the new goal
	syncBody2, _ := json.Marshal(map[string]interface{}{
		"last_synced_at": serverTime,
		"goals":          []interface{}{},
		"completions":    []interface{}{},
	})

	req2 := httptest.NewRequest("POST", "/api/v1/sync/", bytes.NewReader(syncBody2))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second sync failed: %d %s", w2.Code, w2.Body.String())
	}

	var syncResp2 map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&syncResp2)

	serverGoals, ok := syncResp2["goals"].([]interface{})
	if !ok {
		t.Fatal("response missing goals array")
	}
	if len(serverGoals) != 1 {
		t.Errorf("expected 1 server change (the new goal), got %d", len(serverGoals))
	}
}
```

**Step 2: Run the test**

Run: `cd backend && go test ./internal/api/ -v -run TestSync_RoundTrip`
Expected: PASS

**Step 3: Run full backend test suite**

Run: `cd backend && go test ./...`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add backend/internal/api/api_test.go
git commit -m "test(api): add sync round-trip integration test

Covers ApplyChanges through the HTTP layer, last_synced_at filtering,
and server-side changes from other devices."
```

---

## Validation Checklist

After all tasks are complete:

1. **Backend tests:** `cd backend && go test ./... -v` — all pass
2. **Frontend tests:** `cd frontend && npx vitest run` — all pass
3. **Manual spot-check on the sync fix:** Queue a reorder locally, trigger sync, inspect network request in devtools to confirm position fields are sent
4. **Verify no regressions in CI:** Push to a branch and check GitHub Actions

## Task Dependency Graph

```
Task 1 (DeleteAccount fix) ──→ Task 2 (account deletion tests)
                                  │
Task 3 (reorder_goals fix) ──────┼──→ Validation
                                  │
Task 4 (sync round-trip test) ───┘
```

Tasks 1 and 3 are independent and can run in parallel.
Task 2 depends on Task 1 (the fix must be in place for the test to pass).
Task 4 is independent of Tasks 1-3.
