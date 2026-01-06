package db

import (
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Database defines the interface for database operations.
// Both SQLite and PostgreSQL implementations must satisfy this interface.
type Database interface {
	// Goals
	// userID: nil for guest mode (filters by user_id IS NULL), non-nil for authenticated users
	ListGoals(userID *string, includeArchived bool) ([]models.Goal, error)
	GetGoal(userID *string, id string) (*models.Goal, error)
	CreateGoal(goal *models.Goal) error
	UpdateGoal(userID *string, id string, name, color *string, targetCount *int, targetPeriod *string) error
	ArchiveGoal(userID *string, id string) error
	ReorderGoals(userID *string, goalIDs []string) error

	// Completions
	// userID: nil for guest mode (filters by user_id IS NULL), non-nil for authenticated users
	ListCompletions(userID *string, from, to string, goalID *string) ([]models.Completion, error)
	GetCompletionByID(id string) (*models.Completion, error)
	GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error)
	CreateCompletion(c *models.Completion) error
	DeleteCompletion(id string) error

	// Users
	GetUserByID(id string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	CreateUser(user *models.User) error
	UpdateUserLastLogin(id string) error
	GetOrCreateUserByProvider(provider, providerUserID, email, name, avatarURL string) (*models.User, error)

	// Sessions
	CreateSession(session *models.Session) error
	GetSessionByTokenHash(tokenHash string) (*models.Session, error)
	DeleteSession(id string) error
	DeleteExpiredSessions() error

	// Sync operations
	GetGoalChangesSince(userID *string, since *time.Time) ([]models.Goal, error)
	GetCompletionChangesSince(userID *string, since *time.Time) ([]models.Completion, error)
	UpsertGoal(goal *models.Goal) error
	UpsertCompletion(c *models.Completion) error
	SoftDeleteGoal(userID *string, id string) error
	SoftDeleteCompletion(goalID, date string) error
	GetGoalByID(id string) (*models.Goal, error) // Get goal without user filter for sync

	// Lifecycle
	Migrate() error
	Close() error
	Ping() error
}
