package api

import (
	"encoding/json"
	"net/http"
	"strings"

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
		Logger.Error("oauth start failed", "error", err, "provider", provider)
		http.Error(w, "failed to start authentication", http.StatusBadRequest)
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
		Logger.Error("oauth callback failed", "error", err, "provider", provider)
		redirectURL := s.frontendURL + "/?auth_error=authentication_failed"
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// Handle mobile OAuth callback - redirect with one-time auth code
	if result.IsMobile {
		code := s.authCodeStore.Generate(result.SessionToken)
		redirectURL := "goaltracker://auth?code=" + code
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

// deleteAccount permanently deletes the authenticated user's account and all associated data.
func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.db.DeleteAccount(user.ID); err != nil {
		Logger.Error("failed to delete account", "error", err, "user_id", user.ID)
		http.Error(w, "failed to delete account", http.StatusInternalServerError)
		return
	}

	Logger.Info("account deleted", "user_id", user.ID, "email", user.Email)

	// Clear the session cookie
	auth.ClearSessionCookie(w)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "account deleted",
	})
}

// devLogin creates a session for a given email without OAuth.
// This endpoint is only available in development mode.
func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if body.Email == "" {
		body.Email = "dev@localhost"
	}

	// Derive display name from email prefix
	name := body.Email
	if at := strings.Index(body.Email, "@"); at > 0 {
		name = body.Email[:at]
	}

	// Use email as providerUserID so the same email always resolves to the same user
	user, err := s.db.GetOrCreateUserByProvider("dev", body.Email, body.Email, name, "")
	if err != nil {
		http.Error(w, "failed to create dev user", http.StatusInternalServerError)
		return
	}

	sessionToken, err := s.authManager.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, r, sessionToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_token": sessionToken,
		"user":          user,
	})
}

// exchangeAuthCode exchanges a one-time auth code for a session token.
// Used by the mobile app after OAuth redirect.
func (s *Server) exchangeAuthCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	sessionToken, ok := s.authCodeStore.Exchange(body.Code)
	if !ok {
		http.Error(w, "invalid or expired code", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"session_token": sessionToken,
	})
}
