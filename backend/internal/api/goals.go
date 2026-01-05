package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// getUserID extracts the user ID from the request context.
// Returns nil for guest mode (no authenticated user).
func getUserID(r *http.Request) *string {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		return nil
	}
	return &user.ID
}

// colorRegex validates hex color format #RRGGBB
var colorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// validateGoalName checks if the goal name is valid (1-200 chars)
func validateGoalName(name string) (bool, string) {
	if len(name) == 0 {
		return false, "name is required"
	}
	if len(name) > 200 {
		return false, "name must be 200 characters or less"
	}
	return true, ""
}

// validateColor checks if the color is in valid #RRGGBB format
func validateColor(color string) (bool, string) {
	if color == "" {
		return true, "" // empty color is allowed (will use default)
	}
	if !colorRegex.MatchString(color) {
		return false, "color must be in #RRGGBB format (e.g., #4CAF50)"
	}
	return true, ""
}

func (s *Server) listGoals(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("archived") == "true"
	userID := getUserID(r)

	goals, err := s.db.ListGoals(userID, includeArchived)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if goals == nil {
		goals = []models.Goal{}
	}

	writeJSON(w, http.StatusOK, goals)
}

func (s *Server) createGoal(w http.ResponseWriter, r *http.Request) {
	var req models.CreateGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate name
	if valid, errMsg := validateGoalName(req.Name); !valid {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Validate color format if provided
	if valid, errMsg := validateColor(req.Color); !valid {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if req.Color == "" {
		req.Color = "#4CAF50" // default green
	}

	userID := getUserID(r)

	goal := &models.Goal{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Color:     req.Color,
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.db.CreateGoal(goal); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, goal)
}

func (s *Server) updateGoal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)

	var req models.UpdateGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate name if provided
	if req.Name != nil && *req.Name != "" {
		if len(*req.Name) > 200 {
			http.Error(w, "name must be 200 characters or less", http.StatusBadRequest)
			return
		}
	}

	// Validate color format if provided
	if req.Color != nil && *req.Color != "" {
		if !colorRegex.MatchString(*req.Color) {
			http.Error(w, "color must be in #RRGGBB format (e.g., #4CAF50)", http.StatusBadRequest)
			return
		}
	}

	// Check goal exists and belongs to user
	goal, err := s.db.GetGoal(userID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "goal not found", http.StatusNotFound)
		return
	}

	if err := s.db.UpdateGoal(userID, id, req.Name, req.Color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch updated goal
	goal, err = s.db.GetGoal(userID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, goal)
}

func (s *Server) archiveGoal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)

	// Check goal exists and belongs to user
	goal, err := s.db.GetGoal(userID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "goal not found", http.StatusNotFound)
		return
	}

	if err := s.db.ArchiveGoal(userID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reorderGoals(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	var req models.ReorderGoalsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.GoalIDs) == 0 {
		http.Error(w, "goal_ids is required", http.StatusBadRequest)
		return
	}

	if err := s.db.ReorderGoals(userID, req.GoalIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated list
	goals, err := s.db.ListGoals(userID, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, goals)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
