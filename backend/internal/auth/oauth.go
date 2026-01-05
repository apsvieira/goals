package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported OAuth provider")
	ErrOAuthStateMismatch  = errors.New("OAuth state mismatch")
	ErrOAuthExchange       = errors.New("OAuth token exchange failed")
	ErrGetUserInfo         = errors.New("failed to get user info")
)

// OAuthHandler handles OAuth authentication
type OAuthHandler struct {
	db            db.Database
	authManager   *Manager
	googleConfig  *oauth2.Config
	githubConfig  *oauth2.Config
	baseURL       string
}

// GoogleUserInfo represents the user info response from Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GitHubUserInfo represents the user info response from GitHub
type GitHubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubEmail represents an email from GitHub's emails API
type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(database db.Database, authManager *Manager, baseURL string) *OAuthHandler {
	h := &OAuthHandler{
		db:          database,
		authManager: authManager,
		baseURL:     baseURL,
	}

	// Configure Google OAuth
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientID != "" && googleClientSecret != "" {
		h.googleConfig = &oauth2.Config{
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			RedirectURL:  baseURL + "/api/v1/auth/oauth/google/callback",
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}

	// Configure GitHub OAuth
	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if githubClientID != "" && githubClientSecret != "" {
		h.githubConfig = &oauth2.Config{
			ClientID:     githubClientID,
			ClientSecret: githubClientSecret,
			RedirectURL:  baseURL + "/api/v1/auth/oauth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
	}

	return h
}

// StartOAuth initiates the OAuth flow for the specified provider
func (h *OAuthHandler) StartOAuth(w http.ResponseWriter, r *http.Request, provider string) error {
	var config *oauth2.Config

	switch provider {
	case ProviderGoogle:
		config = h.googleConfig
	case ProviderGitHub:
		config = h.githubConfig
	default:
		return ErrUnsupportedProvider
	}

	if config == nil {
		return fmt.Errorf("%s OAuth not configured", provider)
	}

	// Generate random state
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("generate state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// Store state in cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	// Redirect to OAuth provider
	url := config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

// HandleCallback handles the OAuth callback and creates/finds the user
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request, provider string) (string, error) {
	var config *oauth2.Config

	switch provider {
	case ProviderGoogle:
		config = h.googleConfig
	case ProviderGitHub:
		config = h.githubConfig
	default:
		return "", ErrUnsupportedProvider
	}

	if config == nil {
		return "", fmt.Errorf("%s OAuth not configured", provider)
	}

	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		return "", ErrOAuthStateMismatch
	}
	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		return "", ErrOAuthStateMismatch
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOAuthExchange, err)
	}

	// Get user info based on provider
	var providerUserID, email, name, avatarURL string

	switch provider {
	case ProviderGoogle:
		userInfo, err := h.getGoogleUserInfo(token)
		if err != nil {
			return "", err
		}
		providerUserID = userInfo.ID
		email = userInfo.Email
		name = userInfo.Name
		avatarURL = userInfo.Picture

	case ProviderGitHub:
		userInfo, err := h.getGitHubUserInfo(token)
		if err != nil {
			return "", err
		}
		providerUserID = fmt.Sprintf("%d", userInfo.ID)
		email = userInfo.Email
		name = userInfo.Name
		if name == "" {
			name = userInfo.Login
		}
		avatarURL = userInfo.AvatarURL

		// If email is empty, try to get it from the emails API
		if email == "" {
			emails, err := h.getGitHubEmails(token)
			if err == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
			}
		}
	}

	if email == "" {
		return "", fmt.Errorf("%w: email not available", ErrGetUserInfo)
	}

	// Get or create user
	user, err := h.db.GetOrCreateUserByProvider(provider, providerUserID, email, name, avatarURL)
	if err != nil {
		return "", fmt.Errorf("get or create user: %w", err)
	}

	// Create session
	sessionToken, err := h.authManager.CreateSession(user.ID)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return sessionToken, nil
}

// getGoogleUserInfo fetches user info from Google
func (h *OAuthHandler) getGoogleUserInfo(token *oauth2.Token) (*GoogleUserInfo, error) {
	client := h.googleConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGetUserInfo, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrGetUserInfo, err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("%w: parse response: %v", ErrGetUserInfo, err)
	}

	return &userInfo, nil
}

// getGitHubUserInfo fetches user info from GitHub
func (h *OAuthHandler) getGitHubUserInfo(token *oauth2.Token) (*GitHubUserInfo, error) {
	client := h.githubConfig.Client(context.Background(), token)
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrGetUserInfo, err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGetUserInfo, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrGetUserInfo, err)
	}

	var userInfo GitHubUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("%w: parse response: %v", ErrGetUserInfo, err)
	}

	return &userInfo, nil
}

// getGitHubEmails fetches user emails from GitHub
func (h *OAuthHandler) getGitHubEmails(token *oauth2.Token) ([]GitHubEmail, error) {
	client := h.githubConfig.Client(context.Background(), token)
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get emails: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var emails []GitHubEmail
	if err := json.Unmarshal(body, &emails); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return emails, nil
}

// IsGoogleConfigured returns true if Google OAuth is configured
func (h *OAuthHandler) IsGoogleConfigured() bool {
	return h.googleConfig != nil
}

// IsGitHubConfigured returns true if GitHub OAuth is configured
func (h *OAuthHandler) IsGitHubConfigured() bool {
	return h.githubConfig != nil
}
