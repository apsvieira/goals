package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/sync"
)

// handleSync processes a sync request from an authenticated client
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "authentication required for sync",
		})
		return
	}

	// Parse sync request
	var req sync.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid request body",
		})
		return
	}

	// Validate sync request size to prevent abuse
	const maxSyncGoals = 500
	const maxSyncCompletions = 5000

	if len(req.Goals) > maxSyncGoals {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("too many goals in sync request (max %d)", maxSyncGoals),
		})
		return
	}
	if len(req.Completions) > maxSyncCompletions {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("too many completions in sync request (max %d)", maxSyncCompletions),
		})
		return
	}

	// Process sync
	resp, err := s.syncService.ApplyChanges(user.ID, &req)
	if err != nil {
		Logger.Error("sync failed",
			"user_id", user.ID,
			"error", err,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "sync failed",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
