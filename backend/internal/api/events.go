package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/sync"
)

const maxEventsPerRequest = 100

// handleEvents processes a batch of events from an authenticated client.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "authentication required",
		})
		return
	}

	var req sync.EventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if len(req.Events) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "events array is required and must not be empty",
		})
		return
	}

	if len(req.Events) > maxEventsPerRequest {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("too many events (max %d per request)", maxEventsPerRequest),
		})
		return
	}

	// Validate each event has an ID and type
	for _, event := range req.Events {
		if event.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "each event must have an id",
			})
			return
		}
		if event.Type == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "each event must have a type",
			})
			return
		}
	}

	resp, err := s.syncService.ProcessEvents(user.ID, req.Events)
	if err != nil {
		Logger.Error("events processing failed",
			"user_id", user.ID,
			"error", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "events processing failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
