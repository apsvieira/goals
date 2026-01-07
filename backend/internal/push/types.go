package push

import "context"

// Notification represents a push notification to be sent to a device.
type Notification struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
}

// PushService defines the interface for sending push notifications.
type PushService interface {
	// Send sends a notification to a specific device token.
	// Returns an error if the notification could not be sent.
	Send(ctx context.Context, token string, notification *Notification) error

	// SendMultiple sends a notification to multiple device tokens.
	// Returns a map of token to error for any failed sends.
	SendMultiple(ctx context.Context, tokens []string, notification *Notification) map[string]error
}
