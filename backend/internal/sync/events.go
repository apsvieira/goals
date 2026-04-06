package sync

import (
	"fmt"
	"sort"
	"time"
)

// EventRequest represents a single event from the client.
type EventRequest struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   EventPayload    `json:"payload"`
}

// EventPayload contains the event-type-specific data.
type EventPayload struct {
	// Goal fields
	ID           string  `json:"id,omitempty"`
	Name         string  `json:"name,omitempty"`
	Color        string  `json:"color,omitempty"`
	Position     int     `json:"position,omitempty"`
	TargetCount  *int    `json:"target_count,omitempty"`
	TargetPeriod *string `json:"target_period,omitempty"`

	// Completion fields
	GoalID string `json:"goal_id,omitempty"`
	Date   string `json:"date,omitempty"`
}

// EventsRequest is the top-level request body for the events endpoint.
type EventsRequest struct {
	Events []EventRequest `json:"events"`
}

// EventsResponse is the response from the events endpoint.
type EventsResponse struct {
	Processed []string `json:"processed"`
}

// Valid event types.
const (
	EventTypeGoalUpsert     = "goal_upsert"
	EventTypeGoalDelete     = "goal_delete"
	EventTypeCompletionSet  = "completion_set"
	EventTypeCompletionUnset = "completion_unset"
)

// ProcessEvents processes a batch of events for a user.
// Events are sorted by timestamp and processed in order.
// Duplicate event IDs (already processed) are skipped but still reported as processed.
func (s *Service) ProcessEvents(userID string, events []EventRequest) (*EventsResponse, error) {
	userLock := s.getUserLock(userID)
	userLock.Lock()
	defer userLock.Unlock()

	// Lazily prune old processed events (older than 30 days)
	pruneThreshold := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if err := s.db.PruneProcessedEvents(pruneThreshold); err != nil {
		return nil, fmt.Errorf("prune processed events: %w", err)
	}

	// Sort events by timestamp to process in order
	sorted := make([]EventRequest, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	processed := make([]string, 0, len(sorted))

	for _, event := range sorted {
		// Check idempotency
		alreadyProcessed, err := s.db.IsEventProcessed(event.ID)
		if err != nil {
			return nil, fmt.Errorf("check event idempotency: %w", err)
		}
		if alreadyProcessed {
			processed = append(processed, event.ID)
			continue
		}

		// Process based on event type
		switch event.Type {
		case EventTypeGoalUpsert:
			if err := s.processGoalUpsert(userID, event); err != nil {
				return nil, fmt.Errorf("process goal_upsert event %s: %w", event.ID, err)
			}
		case EventTypeGoalDelete:
			if err := s.processGoalDelete(userID, event); err != nil {
				return nil, fmt.Errorf("process goal_delete event %s: %w", event.ID, err)
			}
		case EventTypeCompletionSet:
			if err := s.processCompletionSet(userID, event); err != nil {
				return nil, fmt.Errorf("process completion_set event %s: %w", event.ID, err)
			}
		case EventTypeCompletionUnset:
			if err := s.processCompletionUnset(userID, event); err != nil {
				return nil, fmt.Errorf("process completion_unset event %s: %w", event.ID, err)
			}
		default:
			return nil, fmt.Errorf("unknown event type: %s", event.Type)
		}

		// Mark as processed
		if err := s.db.MarkEventProcessed(event.ID); err != nil {
			return nil, fmt.Errorf("mark event processed: %w", err)
		}

		processed = append(processed, event.ID)
	}

	return &EventsResponse{Processed: processed}, nil
}

func (s *Service) processGoalUpsert(userID string, event EventRequest) error {
	p := event.Payload

	// Validate target_period if provided
	if p.TargetPeriod != nil && *p.TargetPeriod != "week" && *p.TargetPeriod != "month" {
		return fmt.Errorf("invalid target_period: %s", *p.TargetPeriod)
	}

	change := GoalChange{
		ID:           p.ID,
		Name:         p.Name,
		Color:        p.Color,
		Position:     p.Position,
		TargetCount:  p.TargetCount,
		TargetPeriod: p.TargetPeriod,
		UpdatedAt:    event.Timestamp,
		Deleted:      false,
	}

	serverGoal, err := s.db.GetGoalByID(p.ID)
	if err != nil {
		return err
	}

	// Verify ownership if goal exists
	if serverGoal != nil && (serverGoal.UserID == nil || *serverGoal.UserID != userID) {
		return fmt.Errorf("goal %s not owned by user", p.ID)
	}

	mergedGoal, shouldApply := MergeGoal(change, serverGoal)
	if shouldApply {
		if serverGoal == nil {
			mergedGoal.UserID = &userID
		}
		if err := s.db.UpsertGoal(mergedGoal); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) processGoalDelete(userID string, event EventRequest) error {
	p := event.Payload

	change := GoalChange{
		ID:        p.ID,
		UpdatedAt: event.Timestamp,
		Deleted:   true,
	}

	serverGoal, err := s.db.GetGoalByID(p.ID)
	if err != nil {
		return err
	}

	// Verify ownership if goal exists
	if serverGoal != nil && (serverGoal.UserID == nil || *serverGoal.UserID != userID) {
		return fmt.Errorf("goal %s not owned by user", p.ID)
	}

	mergedGoal, shouldApply := MergeGoal(change, serverGoal)
	if shouldApply {
		if serverGoal == nil {
			mergedGoal.UserID = &userID
		}
		if err := s.db.UpsertGoal(mergedGoal); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) processCompletionSet(userID string, event EventRequest) error {
	p := event.Payload

	// Verify goal ownership
	goal, err := s.db.GetGoalByID(p.GoalID)
	if err != nil {
		return err
	}
	if goal == nil || goal.UserID == nil || *goal.UserID != userID {
		return fmt.Errorf("goal %s not owned by user", p.GoalID)
	}

	change := CompletionChange{
		GoalID:    p.GoalID,
		Date:      p.Date,
		Completed: true,
		UpdatedAt: event.Timestamp,
	}

	serverCompletion, err := s.db.GetCompletionByGoalAndDateIncludingDeleted(p.GoalID, p.Date)
	if err != nil {
		return err
	}

	mergedCompletion, shouldApply := MergeCompletion(change, serverCompletion)
	if shouldApply && mergedCompletion != nil {
		if err := s.db.UpsertCompletion(mergedCompletion); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) processCompletionUnset(userID string, event EventRequest) error {
	p := event.Payload

	// Verify goal ownership
	goal, err := s.db.GetGoalByID(p.GoalID)
	if err != nil {
		return err
	}
	if goal == nil || goal.UserID == nil || *goal.UserID != userID {
		return fmt.Errorf("goal %s not owned by user", p.GoalID)
	}

	change := CompletionChange{
		GoalID:    p.GoalID,
		Date:      p.Date,
		Completed: false,
		UpdatedAt: event.Timestamp,
	}

	serverCompletion, err := s.db.GetCompletionByGoalAndDateIncludingDeleted(p.GoalID, p.Date)
	if err != nil {
		return err
	}

	mergedCompletion, shouldApply := MergeCompletion(change, serverCompletion)
	if shouldApply && mergedCompletion != nil {
		if err := s.db.UpsertCompletion(mergedCompletion); err != nil {
			return err
		}
	}

	return nil
}
