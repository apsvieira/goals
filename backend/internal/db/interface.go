package db

import "github.com/apsv/goal-tracker/backend/internal/models"

// Database defines the interface for database operations.
// Both SQLite and PostgreSQL implementations must satisfy this interface.
type Database interface {
	// Goals
	ListGoals(includeArchived bool) ([]models.Goal, error)
	GetGoal(id string) (*models.Goal, error)
	CreateGoal(goal *models.Goal) error
	UpdateGoal(id string, name, color *string) error
	ArchiveGoal(id string) error
	ReorderGoals(goalIDs []string) error

	// Completions
	ListCompletions(from, to string, goalID *string) ([]models.Completion, error)
	GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error)
	CreateCompletion(c *models.Completion) error
	DeleteCompletion(id string) error

	// Lifecycle
	Migrate() error
	Close() error
}
