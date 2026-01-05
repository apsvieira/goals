package models

import "time"

type Goal struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	Position   int        `json:"position"`
	UserID     *string    `json:"user_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type Completion struct {
	ID        string     `json:"id"`
	GoalID    string     `json:"goal_id"`
	Date      string     `json:"date"` // YYYY-MM-DD format
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
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

// Auth types

type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
