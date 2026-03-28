package auth

import (
	"testing"
	"time"
)

func TestAuthCodeStore_StoreAndExchange(t *testing.T) {
	store := NewAuthCodeStore(30 * time.Second)

	code := store.Generate("session-token-123")

	token, ok := store.Exchange(code)
	if !ok {
		t.Fatal("expected exchange to succeed")
	}
	if token != "session-token-123" {
		t.Errorf("expected session-token-123, got %s", token)
	}

	// Second exchange should fail (one-time use)
	_, ok = store.Exchange(code)
	if ok {
		t.Fatal("expected second exchange to fail (one-time use)")
	}
}

func TestAuthCodeStore_ExpiredCode(t *testing.T) {
	store := NewAuthCodeStore(1 * time.Millisecond)

	code := store.Generate("session-token-456")

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	_, ok := store.Exchange(code)
	if ok {
		t.Fatal("expected exchange to fail for expired code")
	}
}
