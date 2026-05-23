package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/apsv/goal-tracker/backend/internal/api"
	"github.com/apsv/goal-tracker/backend/internal/db"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// Defaults to "dev" for local builds without the flag.
var version = "dev"

func buildLoggerProvider(ctx context.Context) (*sdklog.LoggerProvider, error) {
	exp, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithFromEnv())
	if err != nil {
		return nil, err
	}

	// Merge in the build-time service version so every log record carries
	// service.version in PostHog Logs and other OTel backends.
	versionRes, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceVersion(version)),
	)
	if err != nil {
		return nil, err
	}
	res, err = resource.Merge(res, versionRes)
	if err != nil {
		return nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
		sdklog.WithResource(res),
	)
	return lp, nil
}

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

	var loggerProvider *sdklog.LoggerProvider
	if os.Getenv("POSTHOG_PROJECT_TOKEN") != "" {
		if os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT") == "" {
			// Keep on stdlib log — this message is *about* the bridge being absent.
			log.Printf("posthog token set but OTEL_EXPORTER_OTLP_LOGS_ENDPOINT missing; OTel bridge disabled")
		} else {
			ctx := context.Background()
			lp, err := buildLoggerProvider(ctx)
			if err != nil {
				log.Printf("OTel logger provider init failed: %v", err)
			} else {
				loggerProvider = lp
				api.SetOTelLoggerProvider(lp)
			}
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
		api.Logger.Info("opening database", slog.String("driver", "sqlite"), slog.String("path", dbPath))
		database, err = db.NewSQLite(dbPath)
	case "postgres":
		connStr := *dbConn
		if connStr == "" {
			connStr = os.Getenv("DATABASE_URL")
		}
		if connStr == "" {
			// Fatal before the database is open — stdlib log is fine here.
			log.Fatal("PostgreSQL connection string required (use -db flag or set DATABASE_URL env var)")
		}
		api.Logger.Info("opening database", slog.String("driver", "postgres"))
		database, err = db.NewPostgres(connStr)
	default:
		// Fatal before the database is open — stdlib log is fine here.
		log.Fatalf("Unknown database type: %s (use 'sqlite' or 'postgres')", *dbType)
	}

	if err != nil {
		api.Logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	api.Logger.Info("running migrations")
	if err := database.Migrate(); err != nil {
		api.Logger.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	staticFS := getStaticFS()
	handler := api.NewServer(database, staticFS)

	// Create HTTP server with the handler
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Channel to receive shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start background session cleanup (runs every hour)
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	handler.StartSessionCleanup(cleanupCtx, time.Hour)
	// Debug reports retention: delete rows older than 90 days, check once a day
	handler.StartDebugReportsCleanup(cleanupCtx, 24*time.Hour)

	// Start server in a goroutine
	go func() {
		api.Logger.Info("server starting", slog.String("addr", serverAddr))
		if staticFS != nil {
			api.Logger.Info("frontend embedded", slog.String("url", "http://localhost"+serverAddr))
		} else {
			api.Logger.Info("running in dev mode (no embedded frontend)")
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			api.Logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	api.Logger.Info("shutdown signal received, initiating graceful shutdown")

	// Stop background session cleanup
	cleanupCancel()

	// Create context with timeout for graceful HTTP shutdown
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer httpCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(httpCtx); err != nil {
		api.Logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	// Flush buffered OTel logs before exit using a dedicated context so that a
	// slow HTTP drain cannot consume the flush budget.
	if loggerProvider != nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer flushCancel()
		if err := loggerProvider.Shutdown(flushCtx); err != nil {
			api.Logger.Error("OTel logger provider shutdown error", slog.String("error", err.Error()))
		}
	}

	// Close database connection
	api.Logger.Info("closing database connection")
	if err := database.Close(); err != nil {
		api.Logger.Error("error closing database", slog.String("error", err.Error()))
	}

	api.Logger.Info("server shutdown complete")
}
