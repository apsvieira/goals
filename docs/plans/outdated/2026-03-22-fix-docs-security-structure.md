# Fix Documentation, Security, and Structural Issues

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all documentation-code mismatches, security concerns, and structural issues identified in the codebase audit — and remove all beads/bv references.

**Architecture:** Pure cleanup — no new features. Documentation is corrected to match actual code behavior. Dead/broken code paths are removed. Stale scaffolding is deleted. Guest-mode backend code is removed since the frontend enforces auth-only. Beads tooling references are fully stripped from the project.

**Tech Stack:** Go, Svelte 5 / TypeScript, Markdown

---

## Task 1: Fix sync documentation — remove false "CRDT" claims

The README and architecture doc claim "CRDT-based sync" and "CRDT merge." The actual
implementation is plain Last-Write-Wins (LWW) using timestamps. The word "CRDT" does not
appear anywhere in the Go source. The architecture doc also says "server wins on ties" but
the actual merge logic has a special case: for completion timestamp ties, ADD wins over
DELETE (see `backend/internal/sync/merge.go:78`).

**Files:**
- Modify: `README.md:39`
- Modify: `docs/architecture/auth-and-sync.md:26-27, 37-40, 72`

**Step 1: Fix README.md**

In `README.md`, change line 39 from:
```
- CRDT-based sync
```
to:
```
- Last-Write-Wins sync
```

**Step 2: Fix architecture doc sync process description**

In `docs/architecture/auth-and-sync.md`, change line 26 from:
```
5. Apply CRDT merge (server wins conflicts)
```
to:
```
5. Apply Last-Write-Wins merge
```

**Step 3: Fix architecture doc conflict resolution section**

In `docs/architecture/auth-and-sync.md`, replace lines 37-40:
```markdown
### Conflict Resolution
- Last-Write-Wins strategy
- Server timestamp used for comparison
- Server wins on ties
- Silent resolution (no user prompt)
```
with:
```markdown
### Conflict Resolution
- Last-Write-Wins (LWW) strategy
- Timestamps used for comparison — newer write wins
- For goals: on timestamp tie, server version is kept (no update applied)
- For completions: on timestamp tie, ADD wins over DELETE (bias toward user completion)
- Silent resolution (no user prompt)
```

**Step 4: Fix architecture doc data flow diagram**

In `docs/architecture/auth-and-sync.md`, change line 72 from:
```
CRDT Merge
```
to:
```
LWW Merge
```

**Step 5: Run backend tests to ensure nothing is broken**

Run: `cd backend && go test -v ./...`
Expected: All tests pass (no code changes, only docs)

**Step 6: Commit**

```bash
git add README.md docs/architecture/auth-and-sync.md
git commit -m "docs: fix sync documentation — replace CRDT claims with accurate LWW description"
```

---

## Task 2: Fix broken OAuth redirect paths in frontend

Three different OAuth redirect paths exist in the frontend. Only `AuthPage.svelte` uses the
correct route (`/api/v1/auth/oauth/google`). The other two are broken:

- `App.svelte:431` → `/api/v1/auth/google` (missing `/oauth/`)
- `mobile-auth.ts:17` → `/api/v1/auth/google` (missing `/oauth/`)

The backend route is `GET /api/v1/auth/oauth/{provider}`.

Additionally, `App.svelte:handleSignIn` has localhost-sniffing logic that bypasses the Vite
proxy — this is unnecessary since the Vite dev server proxies `/api` to `localhost:8080`.
The `AuthPage.svelte` approach (relative path `/api/v1/auth/oauth/google`) is correct for
both dev and prod.

**Files:**
- Modify: `frontend/src/App.svelte:426-432`
- Modify: `frontend/src/lib/mobile-auth.ts:17`

**Step 1: Fix App.svelte handleSignIn**

In `frontend/src/App.svelte`, replace lines 426-432:
```typescript
  function handleSignIn() {
    // Redirect to Google OAuth
    const apiBase = typeof window !== 'undefined' && window.location.hostname !== 'localhost'
      ? '/api/v1'
      : 'http://localhost:8080/api/v1';
    window.location.href = `${apiBase}/auth/google`;
  }
```
with:
```typescript
  function handleSignIn() {
    window.location.href = '/api/v1/auth/oauth/google';
  }
```

**Step 2: Fix mobile-auth.ts OAuth URL**

In `frontend/src/lib/mobile-auth.ts`, change line 17 from:
```typescript
  const oauthUrl = `${PRODUCTION_API_URL}/api/v1/auth/google?mobile=true`;
```
to:
```typescript
  const oauthUrl = `${PRODUCTION_API_URL}/api/v1/auth/oauth/google?mobile=true`;
```

**Step 3: Run frontend type check**

Run: `cd frontend && npm run check`
Expected: No type errors

**Step 4: Commit**

```bash
git add frontend/src/App.svelte frontend/src/lib/mobile-auth.ts
git commit -m "fix(frontend): correct OAuth redirect paths to match backend route"
```

---

## Task 3: Remove backend guest-mode dead code

The backend middleware, DB interface, and goal handlers all reference "guest mode"
(`user_id IS NULL`). But all data endpoints sit behind `RequireAuth()` middleware
(see `backend/internal/api/router.go:132-133`), which returns 401 if unauthenticated.
Guest mode is therefore unreachable — the `getUserID()` helper in `goals.go` can never
return nil for any protected route.

Remove the guest-mode comments and clarify that `getUserID` always returns a non-nil value
for authenticated routes. Do NOT change the auth `Middleware` itself — it correctly
proceeds without a user for public routes like `/auth/me`. The `RequireAuth` middleware
already gates all data routes.

**Files:**
- Modify: `backend/internal/api/goals.go:15-17` (comment only)
- Modify: `backend/internal/db/interface.go:13, 22` (comments only)
- Modify: `backend/internal/auth/middleware.go:25, 51, 60, 83` (comments only)

**Step 1: Fix goals.go getUserID comment**

In `backend/internal/api/goals.go`, change lines 15-17:
```go
// getUserID extracts the user ID from the request context.
// Returns nil for guest mode (no authenticated user).
func getUserID(r *http.Request) *string {
```
to:
```go
// getUserID extracts the user ID from the request context.
// Returns nil if called outside an authenticated route (all data routes require auth).
func getUserID(r *http.Request) *string {
```

**Step 2: Fix interface.go comments**

In `backend/internal/db/interface.go`, change the two comment lines:

Line 13: change `// userID: nil for guest mode (filters by user_id IS NULL), non-nil for authenticated users`
to: `// userID: filters goals by owner; nil filters by user_id IS NULL`

Line 22: change `// userID: nil for guest mode (filters by user_id IS NULL), non-nil for authenticated users`
to: `// userID: filters completions by goal owner; nil filters by user_id IS NULL`

**Step 3: Fix middleware.go comments**

In `backend/internal/auth/middleware.go`:

Line 25: change
`// The request proceeds even without authentication (for guest mode support).`
to:
`// The request proceeds even without authentication (public routes like /auth/me).`

Line 51: change
`// No token found, proceed without user (guest mode)`
to:
`// No token found, proceed without user (public route)`

Line 60: change
`// Invalid session, proceed without user (guest mode)`
to:
`// Invalid session, proceed without user (public route)`

Line 83: change
`// Returns nil if no user is authenticated (guest mode).`
to:
`// Returns nil if no user is authenticated.`

**Step 4: Run backend tests**

Run: `cd backend && go test -v ./...`
Expected: All tests pass (comments-only changes)

**Step 5: Commit**

```bash
git add backend/internal/api/goals.go backend/internal/db/interface.go backend/internal/auth/middleware.go
git commit -m "docs(backend): remove stale guest-mode references from comments"
```

---

## Task 4: Remove beads tooling from project

Per user request, remove all references to beads, `bd`, `bv`, and beads_viewer from the
project. This includes `AGENTS.md` (which is entirely beads instructions), `CLAUDE.md`
(which references beads), and the `.beads/` directory.

**Files:**
- Delete: `AGENTS.md`
- Delete: `.beads/` directory (recursively)
- Modify: `CLAUDE.md:1` (remove beads reference)
- Modify: `.gitignore:24` (remove `.bv/` entry)

**Step 1: Delete AGENTS.md**

Run: `rm AGENTS.md`

**Step 2: Delete .beads directory**

Run: `rm -rf .beads`

**Step 3: Rewrite CLAUDE.md**

Replace entire contents of `CLAUDE.md` with:
```
# currentDate
Today's date is 2026-03-22.
```

(Remove the beads reference line. Keep the date line which is used by the system.)

Wait — the `CLAUDE.md` also sets `currentDate` which is injected by the system. The only
user-authored line is the beads reference. Remove only that line.

Actually, looking again at the file:
```
In this project, we always manage our tasks with beads. Refer to `AGENTS.md` for more guidance.
```

This is the entire file. Since `AGENTS.md` is being deleted and beads is being removed,
this file should be emptied of that content. Keep the file but leave it empty (the system
injects currentDate separately via system-reminder, not from this file).

Replace `CLAUDE.md` contents with an empty project instructions placeholder:
```
```
(empty file)

**Step 4: Remove .bv/ from .gitignore**

In `.gitignore`, remove this line:
```
.bv/
```

**Step 5: Commit**

```bash
git add -A AGENTS.md .beads/ CLAUDE.md .gitignore
git commit -m "chore: remove beads issue tracking from project"
```

---

## Task 5: Fix Makefile test-frontend target

`Makefile:47` defines `test-frontend` as `npm run check`, which runs `svelte-check`
(a type checker). The actual test runners are `npm run test` (vitest unit tests) and
`npm run test:e2e` (Playwright). The Makefile target should run the unit tests.

**Files:**
- Modify: `Makefile:47`

**Step 1: Fix test-frontend target**

In `Makefile`, change line 47-48:
```makefile
test-frontend:
	cd frontend && npm run check
```
to:
```makefile
test-frontend:
	cd frontend && npm run check && npm run test -- --run
```

The `--run` flag tells vitest to run once and exit (not watch mode).

**Step 2: Verify the target works**

Run: `cd frontend && npm run check && npm run test -- --run`
Expected: Type check passes, then unit tests run and pass.

**Step 3: Commit**

```bash
git add Makefile
git commit -m "fix: include unit tests in Makefile test-frontend target"
```

---

## Task 6: Delete empty root e2e/ scaffolding

The root `e2e/` directory contains only empty subdirectories (`fixtures/`, `helpers/`,
`pages/`). The actual E2E tests live in `frontend/e2e/`. This empty scaffolding is
confusing.

**Files:**
- Delete: `e2e/` directory (recursively)

**Step 1: Delete the empty directory**

Run: `rm -rf e2e`

**Step 2: Commit**

```bash
git add -A e2e/
git commit -m "chore: remove empty root e2e/ scaffolding (tests are in frontend/e2e/)"
```

---

## Task 7: Delete unused Counter.svelte template artifact

`frontend/src/lib/Counter.svelte` is the default Vite+Svelte template component. It's
not imported anywhere in the project.

**Files:**
- Delete: `frontend/src/lib/Counter.svelte`

**Step 1: Verify it's unused**

Run: `grep -r "Counter" frontend/src/ --include="*.svelte" --include="*.ts"`
Expected: No imports or references (only the file itself if grep searches it)

**Step 2: Delete the file**

Run: `rm frontend/src/lib/Counter.svelte`

**Step 3: Run type check**

Run: `cd frontend && npm run check`
Expected: No errors

**Step 4: Commit**

```bash
git add frontend/src/lib/Counter.svelte
git commit -m "chore: remove unused Counter.svelte template artifact"
```

---

## Task 8: Replace boilerplate frontend README

`frontend/README.md` is the default Svelte+Vite template README with no project-specific
content.

**Files:**
- Modify: `frontend/README.md`

**Step 1: Replace with project-specific content**

Replace `frontend/README.md` with:
```markdown
# tiny tracker — frontend

Svelte 5 + TypeScript SPA with offline-first architecture.

## Development

```bash
npm install
npm run dev
```

Dev server runs at http://localhost:5173 and proxies `/api` requests to the backend at `:8080`.

## Testing

```bash
npm run check          # Type checking (svelte-check + tsc)
npm run test           # Unit tests (vitest, watch mode)
npm run test -- --run  # Unit tests (single run)
npm run test:e2e       # E2E tests (Playwright)
```

## Build

```bash
npm run build    # Production build to dist/
```

## Mobile (Capacitor)

```bash
npm run build
npm run cap:sync
npm run cap:android
```
```

**Step 2: Commit**

```bash
git add frontend/README.md
git commit -m "docs(frontend): replace boilerplate README with project-specific content"
```

---

## Task 9: Delete stale test results document

`docs/testing/oauth-offline-sync-results.md` is a 324-line test plan from 2026-01-07 where
every manual test is still marked "REQUIRES MANUAL TESTING" with zero actual results. It
provides no value — the test cases it describes are now covered by the Playwright E2E suite
in `frontend/e2e/` (auth, goals, completions, offline-sync specs).

**Files:**
- Delete: `docs/testing/oauth-offline-sync-results.md`
- Delete: `docs/testing/` directory (if empty after deletion)

**Step 1: Delete the stale document**

Run: `rm docs/testing/oauth-offline-sync-results.md && rmdir docs/testing`

**Step 2: Commit**

```bash
git add -A docs/testing/
git commit -m "chore: remove stale test plan (cases now covered by Playwright E2E suite)"
```

---

## Task 10: Final verification

**Step 1: Run full backend test suite**

Run: `cd backend && go test -v ./...`
Expected: All tests pass

**Step 2: Run full frontend checks and tests**

Run: `cd frontend && npm run check && npm run test -- --run`
Expected: Type check passes, unit tests pass

**Step 3: Verify git state is clean**

Run: `git status`
Expected: Clean working tree, all changes committed
