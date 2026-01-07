package push

import (
	"context"
	"log/slog"
)

// StubService is a stub implementation of PushService that logs notifications
// instead of sending them. This is useful for development and testing, or when
// Firebase credentials are not yet configured.
type StubService struct {
	logger *slog.Logger
}

// NewStubService creates a new stub push notification service.
func NewStubService(logger *slog.Logger) *StubService {
	if logger == nil {
		logger = slog.Default()
	}
	return &StubService{logger: logger}
}

// Send logs the notification instead of sending it.
func (s *StubService) Send(ctx context.Context, token string, notification *Notification) error {
	s.logger.Info("push notification (stub)",
		slog.String("token", maskToken(token)),
		slog.String("title", notification.Title),
		slog.String("body", notification.Body),
		slog.Any("data", notification.Data),
	)
	return nil
}

// SendMultiple logs notifications for multiple tokens.
func (s *StubService) SendMultiple(ctx context.Context, tokens []string, notification *Notification) map[string]error {
	for _, token := range tokens {
		s.Send(ctx, token, notification)
	}
	return nil
}

// maskToken masks all but the first and last 4 characters of a token for logging.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// Ensure StubService implements PushService
var _ PushService = (*StubService)(nil)
