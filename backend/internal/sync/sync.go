package sync

import (
	"sync"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Service handles sync operations
type Service struct {
	db            db.Database
	mu            sync.Mutex
	locks         map[string]*sync.Mutex
	lastPruneTime time.Time
}

// NewService creates a new sync service
func NewService(database db.Database) *Service {
	return &Service{
		db:    database,
		locks: make(map[string]*sync.Mutex),
	}
}

// getUserLock returns a mutex for the given user ID, creating one if needed.
func (s *Service) getUserLock(userID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.locks[userID]; !ok {
		s.locks[userID] = &sync.Mutex{}
	}
	return s.locks[userID]
}

// getChangesSince returns all goals and completions modified since the given timestamp.
// Must be called from within a user-locked context (e.g., ApplyChanges).
func (s *Service) getChangesSince(userID string, since *time.Time) (*SyncResponse, error) {
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
	userLock := s.getUserLock(userID)
	userLock.Lock()
	defer userLock.Unlock()

	serverTime := time.Now().UTC()

	// Track what changes to send back to client (server updates that override client changes)
	// Initialize as empty slices (not nil) to ensure JSON encodes as [] not null
	serverGoalChanges := []GoalChange{}
	serverCompletionChanges := []CompletionChange{}

	// Process goal changes from client
	for _, clientGoal := range req.Goals {
		// Validate target_period if provided
		if clientGoal.TargetPeriod != nil && *clientGoal.TargetPeriod != "week" && *clientGoal.TargetPeriod != "month" {
			continue // Skip goals with invalid target_period
		}

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
		serverChanges, err := s.getChangesSince(userID, req.LastSyncedAt)
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
	return s.db.GetCompletionByGoalAndDateIncludingDeleted(goalID, date)
}
