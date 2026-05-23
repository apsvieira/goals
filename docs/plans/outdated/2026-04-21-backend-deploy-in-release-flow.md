# Backend Deploy In Release Flow Implementation Plan

> **Status:** Implemented and shipped in **v1.2.0** on 2026-04-21. Phase 1 via PR #3 (Fly release v79); Phase 2 via PR #4; real-tag validation (`v1.2.0`) confirmed `deploy-backend` → `release` ordering and Play Store internal publish.

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship the unreleased backend (debug_reports endpoint + migration) to production now, and make every future version tag atomically deploy the backend alongside the Android release so the two can never drift.

**Architecture:**
- Phase 1 is a one-off: merge `feat/debug-log-collection` to `main`, which triggers the existing `deploy-backend.yml` workflow. Verify the migration ran and the endpoint is reachable.
- Phase 2 adds a `deploy-backend` job to `.github/workflows/android-build.yml`'s tag-triggered path. The existing `release` job gains `needs: [deploy-backend]` so a tag publishes the AAB only after the backend at that SHA is live. The branch-push `deploy-backend.yml` workflow stays as-is (main-branch CD). The tag path is idempotent with main's deploy — flyctl no-ops when there are no changes.

**Tech Stack:** GitHub Actions, flyctl, Fly.io (`goal-tracker-app` + `goal-tracker-db`), Go backend with embedded SQL migrations, Gradle/Capacitor Android release pipeline.

---

## Pre-flight: context the executor needs

**Repository shape:**
- `.github/workflows/deploy-backend.yml` — current backend CD. Runs tests then `flyctl deploy --remote-only` on push to `main` (or `workflow_dispatch`). Uses `FLY_API_TOKEN` secret.
- `.github/workflows/android-build.yml` — split into two jobs by `github.ref_type`:
  - `build` (branch/PR push): debug APK artifact.
  - `release` (tag push matching `v[0-9]+.[0-9]+.[0-9]+`): signed AAB, uploads artifact, `publishBundle` to Play Store internal track.
- `fly.toml` — app `goal-tracker-app`, region `gru`, auto-stopped machines.
- Backend auto-runs `Migrate()` on startup (see `backend/internal/db/postgres.go:43-90`), applying any new `postgres_migrations/*.sql` in lexical order. The new migration is `009_add_debug_reports.sql`.
- CLI to read reports: `backend/cmd/debug-reports` (commit `bb30554`). Requires `DATABASE_URL` pointing at the same Postgres the server uses.

**What's unreleased on prod today (v78 release, 2026-04-10):**
- `b823c9a feat(backend): add debug_reports endpoint, schema, rate limiter, cleanup`
- `bb30554 feat(backend): add debug-reports CLI viewer`
- `fd83907 feat(frontend): debug log pipeline, report modal, shake trigger, Sentry`
- `27203b7 chore(ci): pass Sentry env vars to Android build`
- `cf32243 docs: debug-log collection design updates and privacy disclosures`
- `8e984b4 chore(ci): build debug APK on all branch pushes`
- plus notification UX work from earlier in the branch.

**Why this plan exists:** the client was submitting debug reports against a prod backend that has no `/api/v1/debug-reports` route and no `debug_reports` table. The submit looked successful on-device; nothing persisted. After Phase 1 the endpoint is live. Phase 2 prevents the same class of drift from happening again when the next version ships.

---

## Phase 1 — Deploy now (one-off)

### Task 1: Confirm branch is ready to merge

**Step 1.1: Verify the current branch and its state**

Run:
```bash
git status
git log --oneline origin/main..HEAD | head -40
```

Expected: clean working tree (aside from untracked plan docs listed in initial status), current branch `feat/debug-log-collection`, and a commit list that includes `b823c9a`, `bb30554`, `fd83907`.

If the working tree has uncommitted source changes (not docs/plans), stop and surface them to the user before proceeding — don't try to tidy them up silently.

**Step 1.2: Run the backend tests locally**

Run:
```bash
cd backend && go test -race ./...
```

Expected: all tests pass. `deploy-backend.yml` blocks the deploy on this same command, so a failure here means CI will also block.

**Step 1.3: Stop**

Do not commit or push anything yet. Report results to the user and wait for the go-ahead before Task 2.

---

### Task 2: Merge to main via PR

**Rationale:** the existing `deploy-backend.yml` triggers on push to `main`. Merging the branch is the normal path; it also runs Android-build CI for the merge commit.

**Step 2.1: Push the branch (if not already pushed)**

Run:
```bash
git push -u origin feat/debug-log-collection
```

If the branch already tracks the remote, this is a no-op.

**Step 2.2: Open a PR**

Use the `pr-create` skill. Title should reflect the actual scope (it's larger than just debug logs — includes notifications polish). Body should list both:
- debug-log collection feature (backend endpoint, migration, CLI, client pipeline, Sentry wiring, shake trigger)
- notifications UX improvements from earlier on the branch

The PR body **must** flag under "Deployment notes":
- "Merging this runs migration `009_add_debug_reports.sql` on prod Postgres."
- "Triggers auto-deploy via `deploy-backend.yml`."

**Step 2.3: Pause for user review**

Wait for the user to review and merge the PR themselves. Do not merge it for them.

---

### Task 3: Verify the deploy

Executed only after the user confirms the PR is merged.

**Step 3.1: Watch the deploy workflow**

Run:
```bash
gh run list --workflow=deploy-backend.yml --limit 3
gh run watch $(gh run list --workflow=deploy-backend.yml --limit 1 --json databaseId -q '.[0].databaseId')
```

Expected: the latest run is `in_progress` then `completed success`. If tests fail, stop and surface the output.

**Step 3.2: Confirm the release landed**

Run:
```bash
fly releases -a goal-tracker-app | head -5
```

Expected: a new release higher than v78, dated today, by `aps.vieira95@gmail.com` (or `FLY_API_TOKEN` owner).

**Step 3.3: Hit the endpoint**

Run:
```bash
curl -i -X POST https://goal-tracker-app.fly.dev/api/v1/debug-reports \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Expected: `401 Unauthorized` (or whatever the auth middleware returns for an unauthenticated request) — **not** `404`. A 404 means the route wasn't deployed. Any response that proves the route is registered counts as success.

**Step 3.4: Confirm the table exists**

With the proxied `$LOCAL_URL` the user already has configured:
```bash
DATABASE_URL="$LOCAL_URL" go run ./cmd/debug-reports list --since 1d
```

Expected: either an empty listing (no reports yet) or rows from a post-deploy resubmission. Crucially, **no** `relation "debug_reports" does not exist` error.

**Step 3.5: Ask the user to re-submit a test report**

Since the original submission was against the old backend, it never landed. The user should shake-trigger a new report from the device, then re-run the `list` command to see it.

**Step 3.6: Commit nothing; Phase 1 is verify-only.**

---

## Phase 2 — Wire backend deploy into tag/release

### Task 4: Decide placement (thin decision step)

Two possible shapes:

1. **Add a `deploy-backend` job to `android-build.yml` release path.** Single workflow owns "releasing a version." `release` job gains `needs: [deploy-backend]`.
2. **Extend `deploy-backend.yml` to also trigger on tag push.** Cleaner separation of concerns, but requires cross-workflow gating for the Android release (GitHub Actions has no clean cross-workflow `needs`).

**Choose shape 1.** Single workflow = atomic gating. The plan below uses shape 1. If the user prefers shape 2, stop and ask them to re-spec.

---

### Task 5: Add deploy-backend job to release workflow (TDD-lite)

There is no runtime test harness for workflow YAML, so we substitute `actionlint` + a dry-run of the logic for "tests."

**Files:**
- Modify: `.github/workflows/android-build.yml`
- Read: `.github/workflows/deploy-backend.yml` (to copy the deploy step verbatim)

**Step 5.1: Install actionlint locally (if not present)**

Run:
```bash
which actionlint || go install github.com/rhysd/actionlint/cmd/actionlint@latest
actionlint -version
```

If `go install` is unavailable in this environment, skip actionlint and rely on the GitHub-side check — but say so in the commit message.

**Step 5.2: Baseline-lint the current workflow**

Run:
```bash
actionlint .github/workflows/android-build.yml
```

Expected: no output (clean). If it already has warnings, note them and do not fix them as part of this change — flag to the user separately.

**Step 5.3: Add the `deploy-backend` job**

Insert a new job **between** `build` and `release` (order doesn't matter for execution, only readability). The job:

```yaml
  deploy-backend:
    runs-on: ubuntu-latest
    if: github.ref_type == 'tag'
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum

      - name: Run backend tests
        working-directory: backend
        run: go test -race ./...

      - name: Setup flyctl
        uses: superfly/flyctl-actions/setup-flyctl@v1

      - name: Deploy to Fly.io
        run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

Keep it one job (not split test/deploy into two like `deploy-backend.yml` does) — the tag path is already long; one machine is faster and simpler. If the user prefers to mirror `deploy-backend.yml`'s split for consistency, that's a trivial follow-up.

**Step 5.4: Gate the Android release job on backend deploy**

Change the `release` job header from:
```yaml
  release:
    runs-on: ubuntu-latest
    if: github.ref_type == 'tag'
```
to:
```yaml
  release:
    runs-on: ubuntu-latest
    needs: [deploy-backend]
    if: github.ref_type == 'tag'
```

**Step 5.5: Lint the modified workflow**

Run:
```bash
actionlint .github/workflows/android-build.yml
```

Expected: no output.

**Step 5.6: Manual read-through check**

Re-read the whole file top-to-bottom. Verify:
- `build` job still has `if: github.ref_type != 'tag'` — unchanged.
- `deploy-backend` has `if: github.ref_type == 'tag'` — only runs on tags.
- `release` has `needs: [deploy-backend]` — blocks on backend.
- No accidental changes to indentation, env blocks, or secrets.

**Step 5.7: Commit**

Use the `commit` skill. Suggested message:

```
ci(release): deploy backend before Android release on tag

Tag pushes now deploy the backend via flyctl before the Android
release job publishes the AAB to Play Store internal. Prevents
shipping an APK that depends on backend changes not yet live.

The main-branch CD workflow (deploy-backend.yml) is unchanged;
the new job runs only on refs matching v*.*.*, and flyctl is
idempotent when the code at the tag already matches what's on
main.
```

---

### Task 6: Document the release procedure

**Files:**
- Modify or create: `docs/release-process.md` (check if it exists first)

**Step 6.1: Check for existing docs**

Run:
```bash
ls docs/ | grep -i release
```

If a file exists, **read it first** and edit in-place. Do not create a parallel doc.

**Step 6.2: Document the new flow**

The doc must state:
1. Tags must be cut from `main` (so the tag SHA equals the head of main).
2. Pushing a `vX.Y.Z` tag triggers, in order: backend tests → backend deploy to Fly → Android signed release → Play Store internal publish.
3. If backend deploy fails, the Android release is skipped — rerun is safe (flyctl is idempotent; gradle `publishBundle` rejects duplicate versionCodes).
4. The separate `main`-push `deploy-backend.yml` workflow still exists — merges to main still deploy the backend without waiting for a tag.

Keep it short — a page at most. No preamble, no history, just the procedure.

**Step 6.3: Commit**

```
docs(release): describe tag-triggered coordinated release
```

---

### Task 7: Validate the new flow end-to-end

This is the only way to know the YAML change actually works.

**Step 7.1: Pick a validation strategy with the user**

Two options — ask the user which:

- **A. Dry-run via `workflow_dispatch`.** Adds a `workflow_dispatch` trigger to the tag path temporarily, runs the new `deploy-backend` job from the Actions UI, observes it, reverts the trigger. Safe but requires an extra commit/revert.
- **B. Wait for the next real release tag.** Zero extra code; first real bump validates the pipeline.

Recommend B unless the user wants to ship another tag soon and doesn't want the first real one to be the smoke test.

**Step 7.2: Execute the chosen option**

If A: implement, run, observe, revert. If B: stop here and note in the plan's completion message that validation is deferred to the next tag push.

---

### Task 8: Close the loop

**Step 8.1: Final PR for Phase 2 changes**

If Phase 2 was done on a branch (preferred) rather than directly on main, open a PR using the `pr-create` skill. Title: `ci(release): deploy backend alongside Android on tag`. Body links to this plan.

**Step 8.2: Update the project memory**

Add or update a memory entry noting that tag pushes now coordinate a backend + Android release, so future conversations understand the invariant.

---

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Tag cut from a non-main SHA deploys unexpected backend code | Doc says tags must come from main; reinforced by the fact that `release` also always builds Android from the tagged SHA, so both halves come from the same commit. |
| Migration fails on prod (Phase 1) | `Migrate()` errors abort server startup, Fly health check fails, Fly rolls back to v78. The migration is additive (new table, no destructive DDL) so rollback is clean. |
| Flaky backend test blocks an urgent Android-only hotfix | Same risk already exists on the `main` CD path. Acceptable — same mitigation (fix the test, or use the Play Store console's bypass options). |
| Two deploys race on a fast merge→tag sequence | flyctl handles concurrent deploys serially per app; worst case the tag's deploy is a no-op because main's finished first. |

---

## Out of scope

- Splitting the Android build into debug vs release artifacts in a different way.
- Automating tag creation (e.g., release-please). The user creates tags manually today; keep it that way.
- Staged Fly deploys (canary, multi-region). Single-region `gru` is fine for current scale.
- Changing the Play Store track (internal → production promotion). Separate concern.
