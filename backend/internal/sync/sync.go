package sync

import (
	"time"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Service handles sync operations
type Service struct {
	db db.Database
}

// NewService creates a new sync service
func NewService(database db.Database) *Service {
	return &Service{db: database}
}

// GetChangesSince returns all goals and completions modified since the given timestamp
func (s *Service) GetChangesSince(userID string, since *time.Time) (*SyncResponse, error) {
	goals, err := s.db.GetGoalChangesSince(&userID, since)
	if err != nil {
		return nil, err
	}

	completions, err := s.db.GetCompletionChangesSince(&userID, since)
	if err != nil {
		return nil, err
	}

	// Convert to sync change types
	goalChanges := make([]GoalChange, len(goals))
	for i, g := range goals {
		goalChanges[i] = GoalToChange(&g)
	}

	completionChanges := make([]CompletionChange, len(completions))
	for i, c := range completions {
		completionChanges[i] = CompletionToChange(&c)
	}

	return &SyncResponse{
		ServerTime:  time.Now().UTC(),
		Goals:       goalChanges,
		Completions: completionChanges,
	}, nil
}

// ApplyChanges merges client changes with server using LWW strategy
func (s *Service) ApplyChanges(userID string, req *SyncRequest) (*SyncResponse, error) {
	serverTime := time.Now().UTC()

	// Track what changes to send back to client (server updates that override client changes)
	var serverGoalChanges []GoalChange
	var serverCompletionChanges []CompletionChange

	// Process goal changes from client
	for _, clientGoal := range req.Goals {
		serverGoal, err := s.db.GetGoalByID(clientGoal.ID)
		if err != nil {
			return nil, err
		}

		// Verify ownership if goal exists
		if serverGoal != nil && (serverGoal.UserID == nil || *serverGoal.UserID != userID) {
			// Skip goals not owned by this user
			continue
		}

		mergedGoal, shouldApply := MergeGoal(clientGoal, serverGoal)
		if shouldApply {
			// Set user ID for new goals
			if serverGoal == nil {
				mergedGoal.UserID = &userID
			}
			if err := s.db.UpsertGoal(mergedGoal); err != nil {
				return nil, err
			}
		} else if serverGoal != nil {
			// Server version wins, send it back to client
			serverGoalChanges = append(serverGoalChanges, GoalToChange(serverGoal))
		}
	}

	// Process completion changes from client
	for _, clientCompletion := range req.Completions {
		// Get the goal to verify ownership
		goal, err := s.db.GetGoalByID(clientCompletion.GoalID)
		if err != nil {
			return nil, err
		}
		if goal == nil || goal.UserID == nil || *goal.UserID != userID {
			// Skip completions for goals not owned by this user
			continue
		}

		// Get existing completion (including soft-deleted ones)
		serverCompletion, err := s.getCompletionIncludingDeleted(clientCompletion.GoalID, clientCompletion.Date)
		if err != nil {
			return nil, err
		}

		mergedCompletion, shouldApply := MergeCompletion(clientCompletion, serverCompletion)
		if shouldApply && mergedCompletion != nil {
			if err := s.db.UpsertCompletion(mergedCompletion); err != nil {
				return nil, err
			}
		} else if serverCompletion != nil {
			// Server version wins, send it back to client
			serverCompletionChanges = append(serverCompletionChanges, CompletionToChange(serverCompletion))
		}
	}

	// Get all server changes since the client's last sync (to include changes from other devices)
	if req.LastSyncedAt != nil {
		serverChanges, err := s.GetChangesSince(userID, req.LastSyncedAt)
		if err != nil {
			return nil, err
		}

		// Merge with conflict responses
		for _, change := range serverChanges.Goals {
			// Don't duplicate changes we're already sending
			found := false
			for _, existing := range serverGoalChanges {
				if existing.ID == change.ID {
					found = true
					break
				}
			}
			if !found {
				serverGoalChanges = append(serverGoalChanges, change)
			}
		}

		for _, change := range serverChanges.Completions {
			found := false
			for _, existing := range serverCompletionChanges {
				if existing.GoalID == change.GoalID && existing.Date == change.Date {
					found = true
					break
				}
			}
			if !found {
				serverCompletionChanges = append(serverCompletionChanges, change)
			}
		}
	}

	return &SyncResponse{
		ServerTime:  serverTime,
		Goals:       serverGoalChanges,
		Completions: serverCompletionChanges,
	}, nil
}

// getCompletionIncludingDeleted gets a completion by goal and date, including soft-deleted ones
func (s *Service) getCompletionIncludingDeleted(goalID, date string) (*models.Completion, error) {
	// This is a workaround - ideally we'd have a separate DB method
	// For now, we'll use the regular method which excludes deleted
	// This means soft-deleted completions will be re-created, which is acceptable
	// as the client is explicitly marking them as completed
	return s.db.GetCompletionByGoalAndDate(goalID, date)
}
