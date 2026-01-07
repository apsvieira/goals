package api

import (
	"encoding/json"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/go-chi/chi/v5"
)

// registerDevice handles POST /api/v1/devices
// Registers a device token for push notifications.
// Requires authentication.
func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	var req models.RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate token
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	// Validate platform
	if req.Platform != "android" && req.Platform != "ios" {
		http.Error(w, "platform must be 'android' or 'ios'", http.StatusBadRequest)
		return
	}

	// Create or update the device token
	dt, err := s.db.CreateDeviceToken(user.ID, req.Token, req.Platform)
	if err != nil {
		Logger.Error("failed to create device token", "error", err, "user_id", user.ID)
		http.Error(w, "failed to register device", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, dt)
}

// unregisterDevice handles DELETE /api/v1/devices/{id}
// Unregisters a device token for push notifications.
// Requires authentication.
func (s *Server) unregisterDevice(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	tokenID := chi.URLParam(r, "id")
	if tokenID == "" {
		http.Error(w, "device token ID is required", http.StatusBadRequest)
		return
	}

	// Verify the token belongs to the user
	tokens, err := s.db.GetDeviceTokensByUserID(user.ID)
	if err != nil {
		Logger.Error("failed to get device tokens", "error", err, "user_id", user.ID)
		http.Error(w, "failed to unregister device", http.StatusInternalServerError)
		return
	}

	var tokenBelongsToUser bool
	for _, t := range tokens {
		if t.ID == tokenID {
			tokenBelongsToUser = true
			break
		}
	}

	if !tokenBelongsToUser {
		http.Error(w, "device token not found", http.StatusNotFound)
		return
	}

	// Delete the token
	if err := s.db.DeleteDeviceToken(tokenID); err != nil {
		Logger.Error("failed to delete device token", "error", err, "token_id", tokenID)
		http.Error(w, "failed to unregister device", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
