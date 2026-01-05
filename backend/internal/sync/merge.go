package sync

import (
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// MergeGoal merges a client goal change with a server goal using Last-Write-Wins strategy.
// Returns the merged goal and whether it should be applied (updated/created).
func MergeGoal(clientChange GoalChange, serverGoal *models.Goal) (*models.Goal, bool) {
	// If no server goal exists, client wins (new goal)
	if serverGoal == nil {
		now := time.Now().UTC()
		goal := &models.Goal{
			ID:        clientChange.ID,
			Name:      clientChange.Name,
			Color:     clientChange.Color,
			Position:  clientChange.Position,
			UpdatedAt: clientChange.UpdatedAt,
			CreatedAt: now,
		}
		if clientChange.Deleted {
			goal.DeletedAt = &clientChange.UpdatedAt
		}
		return goal, true
	}

	// Last-Write-Wins: client wins if its timestamp is newer
	if clientChange.UpdatedAt.After(serverGoal.UpdatedAt) {
		serverGoal.Name = clientChange.Name
		serverGoal.Color = clientChange.Color
		serverGoal.Position = clientChange.Position
		serverGoal.UpdatedAt = clientChange.UpdatedAt
		if clientChange.Deleted {
			serverGoal.DeletedAt = &clientChange.UpdatedAt
		} else {
			serverGoal.DeletedAt = nil
		}
		return serverGoal, true
	}

	// Server wins, no update needed
	return serverGoal, false
}

// MergeCompletion merges a client completion change with a server completion using Last-Write-Wins strategy.
// For ties, ADD wins (bias toward completion).
// Returns the merged completion and whether it should be applied (updated/created).
func MergeCompletion(clientChange CompletionChange, serverCompletion *models.Completion) (*models.Completion, bool) {
	// If no server completion exists
	if serverCompletion == nil {
		// Only create if client is marking as completed
		if clientChange.Completed {
			now := time.Now().UTC()
			completion := &models.Completion{
				ID:        generateCompletionID(clientChange.GoalID, clientChange.Date),
				GoalID:    clientChange.GoalID,
				Date:      clientChange.Date,
				UpdatedAt: clientChange.UpdatedAt,
				CreatedAt: now,
			}
			return completion, true
		}
		// Client wants to delete but nothing exists, no action needed
		return nil, false
	}

	// Handle timestamp comparison
	clientNewer := clientChange.UpdatedAt.After(serverCompletion.UpdatedAt)
	sameTime := clientChange.UpdatedAt.Equal(serverCompletion.UpdatedAt)

	// Check if server completion is deleted
	serverDeleted := serverCompletion.DeletedAt != nil

	// If client is newer, or same time and client is completing (ADD wins ties)
	if clientNewer || (sameTime && clientChange.Completed && serverDeleted) {
		if clientChange.Completed {
			// Mark as completed (remove deleted_at if it exists)
			serverCompletion.DeletedAt = nil
			serverCompletion.UpdatedAt = clientChange.UpdatedAt
			return serverCompletion, true
		}
		// Mark as deleted (soft delete)
		serverCompletion.DeletedAt = &clientChange.UpdatedAt
		serverCompletion.UpdatedAt = clientChange.UpdatedAt
		return serverCompletion, true
	}

	// Server wins, no update needed
	return serverCompletion, false
}

// GoalToChange converts a models.Goal to a GoalChange
func GoalToChange(goal *models.Goal) GoalChange {
	return GoalChange{
		ID:        goal.ID,
		Name:      goal.Name,
		Color:     goal.Color,
		Position:  goal.Position,
		UpdatedAt: goal.UpdatedAt,
		Deleted:   goal.DeletedAt != nil,
	}
}

// CompletionToChange converts a models.Completion to a CompletionChange
func CompletionToChange(completion *models.Completion) CompletionChange {
	return CompletionChange{
		GoalID:    completion.GoalID,
		Date:      completion.Date,
		Completed: completion.DeletedAt == nil,
		UpdatedAt: completion.UpdatedAt,
	}
}

// generateCompletionID generates a deterministic ID for a completion based on goal and date
func generateCompletionID(goalID, date string) string {
	return goalID + "-" + date
}
