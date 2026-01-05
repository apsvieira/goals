package api

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	db       db.Database
	router   chi.Router
	staticFS fs.FS
}

func NewServer(database db.Database, staticFS fs.FS) *Server {
	s := &Server{db: database, staticFS: staticFS}
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
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
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
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file.(fs.File).(interface {
		Seek(offset int64, whence int) (int64, error)
		Read(p []byte) (n int, err error)
	}).(http.File))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, ww.Status(), time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
