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

	token, err := s.oauthHandler.HandleCallback(w, r, provider)
	if err != nil {
		// Redirect to frontend with error
		http.Redirect(w, r, "/?auth_error="+err.Error(), http.StatusTemporaryRedirect)
		return
	}

	// Set session cookie
	auth.SetSessionCookie(w, r, token)

	// Redirect to frontend
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
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
