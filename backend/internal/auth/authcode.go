package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type authCodeEntry struct {
	sessionToken string
	expiresAt    time.Time
}

// AuthCodeStore provides short-lived, one-time-use authorization codes
// that can be exchanged for session tokens.
type AuthCodeStore struct {
	mu    sync.Mutex
	codes map[string]authCodeEntry
	ttl   time.Duration
}

func NewAuthCodeStore(ttl time.Duration) *AuthCodeStore {
	return &AuthCodeStore{
		codes: make(map[string]authCodeEntry),
		ttl:   ttl,
	}
}

// Generate creates a new auth code for the given session token.
func (s *AuthCodeStore) Generate(sessionToken string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	code := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up expired codes opportunistically
	now := time.Now()
	for k, v := range s.codes {
		if now.After(v.expiresAt) {
			delete(s.codes, k)
		}
	}

	s.codes[code] = authCodeEntry{
		sessionToken: sessionToken,
		expiresAt:    now.Add(s.ttl),
	}
	return code
}

// Exchange validates and consumes a one-time auth code, returning the
// associated session token. Returns ("", false) if the code is invalid,
// expired, or already used.
func (s *AuthCodeStore) Exchange(code string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.codes[code]
	if !ok {
		return "", false
	}
	delete(s.codes, code) // One-time use

	if time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.sessionToken, true
}
