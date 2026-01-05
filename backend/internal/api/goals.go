package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) listGoals(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("archived") == "true"

	goals, err := s.db.ListGoals(includeArchived)
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

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		req.Color = "#4CAF50" // default green
	}

	goal := &models.Goal{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Color:     req.Color,
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

	var req models.UpdateGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Check goal exists
	goal, err := s.db.GetGoal(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "goal not found", http.StatusNotFound)
		return
	}

	if err := s.db.UpdateGoal(id, req.Name, req.Color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch updated goal
	goal, err = s.db.GetGoal(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, goal)
}

func (s *Server) archiveGoal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check goal exists
	goal, err := s.db.GetGoal(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "goal not found", http.StatusNotFound)
		return
	}

	if err := s.db.ArchiveGoal(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reorderGoals(w http.ResponseWriter, r *http.Request) {
	var req models.ReorderGoalsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.GoalIDs) == 0 {
		http.Error(w, "goal_ids is required", http.StatusBadRequest)
		return
	}

	if err := s.db.ReorderGoals(req.GoalIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated list
	goals, err := s.db.ListGoals(false)
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
