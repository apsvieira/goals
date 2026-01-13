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
// from the Authorization header (Bearer token) or cookie and validates it.
// If valid, the user is stored in the request context.
// The request proceeds even without authentication (for guest mode support).
//
// Authentication methods (in order of priority):
// 1. Authorization: Bearer <token> header (for mobile apps)
// 2. Session cookie (for web apps)
func Middleware(authManager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			var fromHeader bool

			// First, check Authorization header (Bearer token)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
				fromHeader = true
			}

			// Fall back to session cookie if no Authorization header
			if token == "" {
				cookie, err := r.Cookie(SessionCookieName)
				if err == nil && cookie.Value != "" {
					token = cookie.Value
				}
			}

			// No token found, proceed without user (guest mode)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Validate session
			user, err := authManager.ValidateSession(token)
			if err != nil {
				// Invalid session, proceed without user (guest mode)
				// Only clear cookie if it was used (not for header-based auth)
				if !fromHeader {
					http.SetCookie(w, &http.Cookie{
						Name:     SessionCookieName,
						Value:    "",
						Path:     "/",
						MaxAge:   -1,
						HttpOnly: true,
					})
				}
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
	secure := shouldUseSecureCookie(r)

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

// shouldUseSecureCookie determines if the Secure flag should be set on cookies.
// It returns false for localhost/development environments to support HTTP-only testing.
// Priority:
// 1. COOKIE_SECURE env var (explicit override)
// 2. Auto-detect localhost (127.0.0.1, localhost) -> false
// 3. Default to true for production safety
func shouldUseSecureCookie(r *http.Request) bool {
	// Allow explicit override via environment variable
	if secureEnv := os.Getenv("COOKIE_SECURE"); secureEnv != "" {
		return strings.ToLower(secureEnv) != "false"
	}

	// Auto-detect localhost - WebKit enforces Secure cookies strictly,
	// so we need to disable Secure for localhost HTTP testing
	host := r.Host
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		return false
	}

	// Default to secure for production
	return true
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

// RequireAuth is a middleware that requires authentication.
// Returns 401 Unauthorized if no valid session is present.
// Must be used after the main Middleware which populates the user context.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
