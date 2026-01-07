package api

import (
	"encoding/json"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

// getCurrentUser returns the currently authenticated user
func (s *Server) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "not authenticated",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// startOAuth initiates the OAuth flow
func (s *Server) startOAuth(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	err := s.oauthHandler.StartOAuth(w, r, provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

// oauthCallback handles the OAuth callback
func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	result, err := s.oauthHandler.HandleCallback(w, r, provider)
	if err != nil {
		// Check if this is a mobile request by looking at the error context
		// For mobile, redirect to custom URL scheme with error
		if r.URL.Query().Get("state") != "" {
			// Try to determine if mobile from cookie (may not work if cookie cleared)
			// Fall back to web redirect
		}
		// Redirect to frontend with error
		redirectURL := s.frontendURL + "/?auth_error=" + err.Error()
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// Handle mobile OAuth callback - redirect to custom URL scheme
	if result.IsMobile {
		// Redirect to mobile app with token
		// The mobile app will use this token with Bearer authentication
		redirectURL := "goaltracker://auth?token=" + result.SessionToken
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// Web flow: Set session cookie and redirect to frontend
	auth.SetSessionCookie(w, r, result.SessionToken)

	// Redirect to frontend (frontendURL is empty in prod, so "/" works)
	redirectURL := s.frontendURL + "/"
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// logout invalidates the current session
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	// Get session token from cookie
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// Delete the session
		_ = s.authManager.DeleteSession(cookie.Value)
	}

	// Clear the cookie
	auth.ClearSessionCookie(w)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "logged out",
	})
}
