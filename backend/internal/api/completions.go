package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (s *Server) listCompletions(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		http.Error(w, "from and to query parameters are required", http.StatusBadRequest)
		return
	}

	if !dateRegex.MatchString(from) || !dateRegex.MatchString(to) {
		http.Error(w, "from and to must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}

	var goalID *string
	if g := r.URL.Query().Get("goal_id"); g != "" {
		goalID = &g
	}

	userID := getUserID(r)
	completions, err := s.db.ListCompletions(userID, from, to, goalID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if completions == nil {
		completions = []models.Completion{}
	}

	writeJSON(w, http.StatusOK, completions)
}

func (s *Server) createCompletion(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.GoalID == "" {
		http.Error(w, "goal_id is required", http.StatusBadRequest)
		return
	}
	if req.Date == "" {
		http.Error(w, "date is required", http.StatusBadRequest)
		return
	}
	if !dateRegex.MatchString(req.Date) {
		http.Error(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}

	// Validate date is not in the future (using UTC)
	reqDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if reqDate.After(today) {
		http.Error(w, "cannot create completions for future dates", http.StatusBadRequest)
		return
	}

	// Check goal exists and belongs to user
	userID := getUserID(r)
	goal, err := s.db.GetGoal(userID, req.GoalID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "goal not found", http.StatusNotFound)
		return
	}

	// Check for existing completion (idempotent)
	existing, err := s.db.GetCompletionByGoalAndDate(req.GoalID, req.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	completion := &models.Completion{
		ID:        uuid.New().String(),
		GoalID:    req.GoalID,
		Date:      req.Date,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.db.CreateCompletion(completion); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, completion)
}

func (s *Server) deleteCompletion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)

	// Get the completion to verify it exists
	completion, err := s.db.GetCompletionByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if completion == nil {
		http.Error(w, "completion not found", http.StatusNotFound)
		return
	}

	// Verify the completion's goal belongs to the current user
	goal, err := s.db.GetGoal(userID, completion.GoalID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if goal == nil {
		http.Error(w, "completion not found", http.StatusNotFound)
		return
	}

	if err := s.db.DeleteCompletion(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getCalendar(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		// Default to current month
		month = time.Now().Format("2006-01")
	}

	// Parse month to get date range
	t, err := time.Parse("2006-01", month)
	if err != nil {
		http.Error(w, "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	from := t.Format("2006-01-02")
	to := t.AddDate(0, 1, -1).Format("2006-01-02")

	userID := getUserID(r)

	goals, err := s.db.ListGoals(userID, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	completions, err := s.db.ListCompletions(userID, from, to, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if goals == nil {
		goals = []models.Goal{}
	}
	if completions == nil {
		completions = []models.Completion{}
	}

	writeJSON(w, http.StatusOK, models.CalendarResponse{
		Goals:       goals,
		Completions: completions,
	})
}
