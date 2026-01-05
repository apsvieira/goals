package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// UserContextKey is the key used to store the user in the request context
	UserContextKey contextKey = "user"
	// SessionCookieName is the name of the session cookie
	SessionCookieName = "session"
)

// Middleware creates an authentication middleware that extracts the session token
// from the cookie and validates it. If valid, the user is stored in the request context.
// The request proceeds even without authentication (for guest mode support).
func Middleware(authManager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to get session cookie
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				// No session cookie, proceed without user (guest mode)
				next.ServeHTTP(w, r)
				return
			}

			// Validate session
			user, err := authManager.ValidateSession(cookie.Value)
			if err != nil {
				// Invalid session, proceed without user (guest mode)
				// Optionally clear the invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     SessionCookieName,
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				next.ServeHTTP(w, r)
				return
			}

			// Store user in context and proceed
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is authenticated (guest mode).
func GetUserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// IsAuthenticated returns true if there is an authenticated user in the context
func IsAuthenticated(ctx context.Context) bool {
	return GetUserFromContext(ctx) != nil
}

// SetSessionCookie sets the session cookie on the response
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	// Determine if we should use secure cookies
	// Default to true in production, can be disabled with COOKIE_SECURE=false
	secure := true
	if secureEnv := os.Getenv("COOKIE_SECURE"); strings.ToLower(secureEnv) == "false" {
		secure = false
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie clears the session cookie
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
