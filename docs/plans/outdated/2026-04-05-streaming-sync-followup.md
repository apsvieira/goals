# Streaming Sync — Follow-up Cleanup

**Context:** The streaming sync migration (2026-04-05-streaming-sync-design.md) is implemented. This task covers leftover cleanup items flagged during the final code review. None are blocking, but all are worth addressing before the codebase accumulates more layers on top.

---

## 1. Wire `syncStatus` into the event sync system

**Problem:** The `syncStatus` Svelte store (`sync.ts`) is subscribed to in `App.svelte:50` and drives the syncing cloud icon in `Header.svelte:77-86`. After the migration, nothing writes to this store — it's permanently `{ state: 'idle' }`, so the syncing indicator never shows.

**Fix:** In `event-sync.ts`, set `syncStatus` at appropriate points:
- `syncing` at the start of `flushPendingEvents()`
- `idle` on success
- `error` with `canRetry: true` on persistent failure

For `sendEvent()` (single immediate sends), don't set `syncing` — the operation is too fast to be worth a UI flash. Only `flushPendingEvents()` (batch on startup/reconnect) should drive the indicator.

**Files:**
- Modify: `frontend/src/lib/event-sync.ts`

**Tests:** Verify `syncStatus` is set to `syncing` during flush and back to `idle` on completion.

## 2. Remove the no-op `migrateOldQueue` function and its test

**Problem:** `migrateOldQueue()` in `event-sync.ts:106` is an empty function kept "for one release cycle." Since the `operations` store was already dropped in the same IndexedDB version bump (v4), no client will ever have old entries to migrate. The function can be removed along with its test file.

**Fix:**
- Remove `migrateOldQueue()` from `event-sync.ts` and its call in `startEventSync()`
- Delete `frontend/src/lib/__tests__/queue-migration.test.ts`

## 3. Add client-side event pruning

**Problem:** Synced events accumulate indefinitely in IndexedDB. At ~20 events/day this takes years to matter, but it's simple to prevent.

**Fix:** At the end of `flushPendingEvents()` (after marking events synced), delete synced events older than 7 days from the `events` store. Add a `pruneSyncedEvents(olderThanDays: number)` function to `storage.ts`.

**Files:**
- Modify: `frontend/src/lib/storage.ts` — add prune function
- Modify: `frontend/src/lib/event-sync.ts` — call prune after flush

## 4. Remove `sync.ts` entirely

**Problem:** After fix #1 is done, evaluate whether `sync.ts` (which only exports `syncStatus` and `SyncStatus`) should be merged into `event-sync.ts`. Having a 9-line file that exists solely because of historical naming is unnecessary indirection.

**Fix:** Move `syncStatus` and `SyncStatus` into `event-sync.ts`. Update imports in `App.svelte`.

**Files:**
- Delete: `frontend/src/lib/sync.ts`
- Modify: `frontend/src/lib/event-sync.ts`
- Modify: `frontend/src/App.svelte`
