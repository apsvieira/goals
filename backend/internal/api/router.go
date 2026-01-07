package api

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Logger is the structured logger for the application
var Logger *slog.Logger

func init() {
	initLogger()
}

// initLogger initializes the structured logger based on LOG_FORMAT env var
func initLogger() {
	logFormat := os.Getenv("LOG_FORMAT")
	var handler slog.Handler

	if logFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		// Default to JSON format for production
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

type Server struct {
	db              db.Database
	router          chi.Router
	staticFS        fs.FS
	authManager     *auth.Manager
	oauthHandler    *auth.OAuthHandler
	authRateLimiter *RateLimiter
	apiRateLimiter  *RateLimiter
	syncRateLimiter *RateLimiter
	frontendURL     string
}

func NewServer(database db.Database, staticFS fs.FS) *Server {
	// Get base URL from environment, default to localhost for dev
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Frontend URL for redirects after OAuth (different in dev vs prod)
	frontendURL := os.Getenv("FRONTEND_URL")
	// In production, frontend is served from same origin, so empty means "/"
	// In dev, set FRONTEND_URL=http://localhost:5173

	authManager := auth.NewManager(database)
	oauthHandler := auth.NewOAuthHandler(database, authManager, baseURL)

	// Create rate limiters per IP
	// Auth endpoints: 10 requests per minute (strict - prevent brute force)
	authRateLimiter := NewRateLimiter(10, time.Minute)
	// General API endpoints: 100 requests per minute (generous for normal use)
	apiRateLimiter := NewRateLimiter(100, time.Minute)
	// Sync endpoint: 30 requests per minute (moderate - more expensive operation)
	syncRateLimiter := NewRateLimiter(30, time.Minute)

	s := &Server{
		db:              database,
		staticFS:        staticFS,
		authManager:     authManager,
		oauthHandler:    oauthHandler,
		authRateLimiter: authRateLimiter,
		apiRateLimiter:  apiRateLimiter,
		syncRateLimiter: syncRateLimiter,
		frontendURL:     frontendURL,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(requestTimeout(30 * time.Second))
	r.Use(corsMiddleware)
	r.Use(auth.Middleware(s.authManager))

	// Health endpoint
	r.Get("/health", s.healthCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes with strict rate limiting (10/min - prevent brute force)
		r.Route("/auth", func(r chi.Router) {
			r.Use(RateLimitMiddleware(s.authRateLimiter))
			r.Get("/me", s.getCurrentUser)
			r.Get("/oauth/{provider}", s.startOAuth)
			r.Get("/oauth/{provider}/callback", s.oauthCallback)
			r.Post("/logout", s.logout)
		})

		// Sync endpoint with moderate rate limiting (30/min - expensive operation)
		r.Route("/sync", func(r chi.Router) {
			r.Use(RateLimitMiddleware(s.syncRateLimiter))
			r.Post("/", s.handleSync)
		})

		// Data endpoints with generous rate limiting (100/min - normal API use)
		r.Group(func(r chi.Router) {
			r.Use(RateLimitMiddleware(s.apiRateLimiter))

			// Goals
			r.Get("/goals", s.listGoals)
			r.Post("/goals", s.createGoal)
			r.Patch("/goals/{id}", s.updateGoal)
			r.Delete("/goals/{id}", s.archiveGoal)
			r.Put("/goals/reorder", s.reorderGoals)

			// Completions
			r.Get("/completions", s.listCompletions)
			r.Post("/completions", s.createCompletion)
			r.Delete("/completions/{id}", s.deleteCompletion)

			// Calendar convenience endpoint
			r.Get("/calendar", s.getCalendar)
		})
	})

	// Serve embedded frontend if available
	if s.staticFS != nil {
		r.Get("/*", s.serveStatic)
	}

	s.router = r
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try to open the file
	file, err := s.staticFS.Open(path)
	if err != nil {
		// For SPA routing, serve index.html for missing files
		path = "index.html"
		file, err = s.staticFS.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer file.Close()

	// Check if it's a directory
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if stat.IsDir() {
		path = path + "/index.html"
		file.Close()
		file, err = s.staticFS.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		stat, _ = file.Stat()
	}

	// Serve the file
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file.(io.ReadSeeker))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		// Get request ID from context
		requestID := middleware.GetReqID(r.Context())

		// Get user ID if authenticated
		var userID string
		if user := auth.GetUserFromContext(r.Context()); user != nil {
			userID = user.ID
		}

		// Log with structured fields
		Logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", requestID),
			slog.String("user_id", userID),
		)
	})
}

// securityHeaders adds security-related HTTP headers to all responses
func securityHeaders(next http.Handler) http.Handler {
	// Content Security Policy
	// - default-src 'self': Only load resources from same origin by default
	// - script-src 'self': Scripts only from same origin
	// - style-src 'self' 'unsafe-inline': Styles from same origin + inline (needed for dynamic colors in Svelte)
	// - img-src 'self' https://lh3.googleusercontent.com data:: Images from self, Google avatars, and data URIs
	// - connect-src 'self': API/fetch calls only to same origin
	// - font-src 'self': Fonts only from same origin
	// - object-src 'none': Disallow plugins (Flash, etc.)
	// - frame-ancestors 'none': Prevent embedding in iframes (aligns with X-Frame-Options)
	// - base-uri 'self': Prevent base tag hijacking
	// - form-action 'self': Forms only submit to same origin
	// - worker-src 'self': Service workers from same origin
	csp := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' https://lh3.googleusercontent.com data:; " +
		"connect-src 'self'; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"worker-src 'self'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

// requestTimeout adds a timeout to all requests
func requestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	// Get allowed origins from environment
	// SECURITY: Defaults to no CORS (same-origin only) if not set
	// Set CORS_ORIGINS=* for development or specific origins for production
	allowedOrigins := os.Getenv("CORS_ORIGINS")

	// Mobile app origins that are always allowed (Capacitor/Cordova apps)
	mobileOrigins := map[string]bool{
		"capacitor://localhost": true,
		"http://localhost":      true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		originAllowed := false

		// Always allow mobile app origins for Capacitor/Cordova support
		if mobileOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			originAllowed = true
		} else if allowedOrigins == "" {
			// If no CORS_ORIGINS configured, don't set CORS headers (same-origin only)
			next.ServeHTTP(w, r)
			return
		} else if allowedOrigins == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			originAllowed = true
			// Note: Cannot use credentials with wildcard origin per CORS spec
			// Don't set Access-Control-Allow-Credentials with "*"
		} else {
			// Parse comma-separated origins
			origins := strings.Split(allowedOrigins, ",")
			for _, allowed := range origins {
				if strings.TrimSpace(allowed) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					originAllowed = true
					break
				}
			}
		}

		if originAllowed {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// StartSessionCleanup starts a background goroutine that periodically cleans up
// expired sessions. It runs every cleanupInterval and stops when the context is cancelled.
func (s *Server) StartSessionCleanup(ctx context.Context, cleanupInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		// Run cleanup immediately on startup
		if err := s.authManager.CleanupExpiredSessions(); err != nil {
			Logger.Error("session cleanup failed", slog.String("error", err.Error()))
		} else {
			Logger.Info("session cleanup completed")
		}

		for {
			select {
			case <-ctx.Done():
				Logger.Info("session cleanup stopped")
				return
			case <-ticker.C:
				if err := s.authManager.CleanupExpiredSessions(); err != nil {
					Logger.Error("session cleanup failed", slog.String("error", err.Error()))
				} else {
					Logger.Info("session cleanup completed")
				}
			}
		}
	}()
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	// Ping the database to check connectivity
	if err := s.db.Ping(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
