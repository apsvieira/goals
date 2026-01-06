package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/api"
	"github.com/apsv/goal-tracker/backend/internal/db"
)

func main() {
	addr := flag.String("addr", "", "HTTP server address (defaults to :8080 or PORT env var)")
	dbType := flag.String("db-type", "sqlite", "Database type: sqlite or postgres")
	dbConn := flag.String("db", "", "Database connection (path for sqlite, URL for postgres). Defaults to sqlite path if empty, or DATABASE_URL env var for postgres.")
	flag.Parse()

	// Determine server address
	serverAddr := *addr
	if serverAddr == "" {
		if port := os.Getenv("PORT"); port != "" {
			serverAddr = ":" + port
		} else {
			serverAddr = ":8080"
		}
	}

	var database db.Database
	var err error

	switch *dbType {
	case "sqlite":
		dbPath := *dbConn
		if dbPath == "" {
			dbPath = db.DefaultDBPath()
		}
		log.Printf("Opening SQLite database at %s", dbPath)
		database, err = db.NewSQLite(dbPath)
	case "postgres":
		connStr := *dbConn
		if connStr == "" {
			connStr = os.Getenv("DATABASE_URL")
		}
		if connStr == "" {
			log.Fatal("PostgreSQL connection string required (use -db flag or set DATABASE_URL env var)")
		}
		log.Printf("Connecting to PostgreSQL database")
		database, err = db.NewPostgres(connStr)
	default:
		log.Fatalf("Unknown database type: %s (use 'sqlite' or 'postgres')", *dbType)
	}

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	log.Println("Running migrations...")
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	staticFS := getStaticFS()
	handler := api.NewServer(database, staticFS)

	// Create HTTP server with the handler
	server := &http.Server{
		Addr:    serverAddr,
		Handler: handler,
	}

	// Channel to receive shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start background session cleanup (runs every hour)
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	handler.StartSessionCleanup(cleanupCtx, time.Hour)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", serverAddr)
		if staticFS != nil {
			log.Printf("Open http://localhost%s in your browser", serverAddr)
		} else {
			log.Printf("Running in dev mode (no embedded frontend)")
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	log.Println("Shutdown signal received, initiating graceful shutdown...")

	// Stop background session cleanup
	cleanupCancel()

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	log.Println("Closing database connection...")
	if err := database.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	log.Println("Server shutdown complete")
}
