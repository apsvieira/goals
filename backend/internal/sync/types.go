package sync

import "time"

// SyncRequest represents a client sync request
type SyncRequest struct {
	LastSyncedAt *time.Time         `json:"last_synced_at"`
	Goals        []GoalChange       `json:"goals"`
	Completions  []CompletionChange `json:"completions"`
}

// SyncResponse represents a server sync response
type SyncResponse struct {
	ServerTime  time.Time          `json:"server_time"`
	Goals       []GoalChange       `json:"goals"`
	Completions []CompletionChange `json:"completions"`
}

// GoalChange represents a goal change for sync
type GoalChange struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	UpdatedAt time.Time `json:"updated_at"`
	Deleted   bool      `json:"deleted"`
}

// CompletionChange represents a completion change for sync
type CompletionChange struct {
	GoalID    string    `json:"goal_id"`
	Date      string    `json:"date"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updated_at"`
}
