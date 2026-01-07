# Authentication and Sync Architecture

## Overview

Goal Tracker requires authentication via Google OAuth. All data is stored on the server and cached locally for offline access.

## Authentication Flow

1. User opens app
2. If no valid session: redirect to Google OAuth
3. After OAuth: create session, start sync
4. Session valid for 30 days

## Offline Sync

### Operation Queue
- All user actions write to IndexedDB immediately (instant UI)
- Operations are queued for background sync
- Queue persists across app restarts

### Sync Process (every 2 minutes)
1. Check online and authenticated
2. Get queued operations from IndexedDB
3. Send to `/api/v1/sync` endpoint
4. Receive server changes since last sync
5. Apply CRDT merge (server wins conflicts)
6. Clear successfully synced operations
7. Update `lastSyncedAt` timestamp

### Sync Triggers
- Every 2 minutes when online
- On network reconnection (online event)
- On app resume (Capacitor)
- After user actions (opportunistic)

### Conflict Resolution
- Last-Write-Wins strategy
- Server timestamp used for comparison
- Server wins on ties
- Silent resolution (no user prompt)

## Offline Behavior

When offline:
- All operations work normally
- Changes cached in IndexedDB
- Operations queued for sync
- UI shows "offline" indicator
- Data readable from local cache

When online:
- Automatic sync every 2 minutes
- Queued operations sent to server
- Server changes applied locally
- Queue cleared on success

## Data Flow

```
User Action
    ↓
IndexedDB Write (immediate)
    ↓
UI Update (instant)
    ↓
Queue Operation
    ↓
Background Sync (2 min interval)
    ↓
Server API Call
    ↓
CRDT Merge
    ↓
Apply Server Changes
    ↓
Clear Queue
```
