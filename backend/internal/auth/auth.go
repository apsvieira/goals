package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

const (
	// SessionDuration is how long a session is valid
	SessionDuration = 30 * 24 * time.Hour // 30 days
	// TokenLength is the length of the raw session token
	TokenLength = 32
)

var (
	ErrInvalidSession = errors.New("invalid or expired session")
)

// Manager handles session management
type Manager struct {
	db db.Database
}

// NewManager creates a new auth manager
func NewManager(database db.Database) *Manager {
	return &Manager{db: database}
}

// CreateSession creates a new session for a user and returns the raw token
func (m *Manager) CreateSession(userID string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Hash token for storage
	tokenHash := hashToken(token)

	// Generate session ID
	sessionIDBytes := make([]byte, 16)
	if _, err := rand.Read(sessionIDBytes); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	sessionID := hex.EncodeToString(sessionIDBytes)

	now := time.Now().UTC()
	session := &models.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(SessionDuration),
		CreatedAt: now,
	}

	if err := m.db.CreateSession(session); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

// ValidateSession validates a session token and returns the associated user
func (m *Manager) ValidateSession(token string) (*models.User, error) {
	if token == "" {
		return nil, ErrInvalidSession
	}

	tokenHash := hashToken(token)

	session, err := m.db.GetSessionByTokenHash(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, ErrInvalidSession
	}

	// Check if session is expired
	if time.Now().UTC().After(session.ExpiresAt) {
		// Delete expired session
		_ = m.db.DeleteSession(session.ID)
		return nil, ErrInvalidSession
	}

	// Get user
	user, err := m.db.GetUserByID(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		// User no longer exists, delete session
		_ = m.db.DeleteSession(session.ID)
		return nil, ErrInvalidSession
	}

	return user, nil
}

// DeleteSession invalidates a session by its token
func (m *Manager) DeleteSession(token string) error {
	if token == "" {
		return nil
	}

	tokenHash := hashToken(token)

	session, err := m.db.GetSessionByTokenHash(tokenHash)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil // Session doesn't exist, nothing to delete
	}

	return m.db.DeleteSession(session.ID)
}

// CleanupExpiredSessions removes all expired sessions
func (m *Manager) CleanupExpiredSessions() error {
	return m.db.DeleteExpiredSessions()
}

// hashToken creates a SHA256 hash of the token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
