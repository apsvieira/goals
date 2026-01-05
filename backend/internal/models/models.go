package models

import "time"

type Goal struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	Position   int        `json:"position"`
	CreatedAt  time.Time  `json:"created_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

type Completion struct {
	ID        string    `json:"id"`
	GoalID    string    `json:"goal_id"`
	Date      string    `json:"date"` // YYYY-MM-DD format
	CreatedAt time.Time `json:"created_at"`
}

type CalendarResponse struct {
	Goals       []Goal       `json:"goals"`
	Completions []Completion `json:"completions"`
}

// Request types

type CreateGoalRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type UpdateGoalRequest struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
}

type CreateCompletionRequest struct {
	GoalID string `json:"goal_id"`
	Date   string `json:"date"` // YYYY-MM-DD format
}

type ReorderGoalsRequest struct {
	GoalIDs []string `json:"goal_ids"` // Goal IDs in desired order
}
