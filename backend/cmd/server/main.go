package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/api"
	"github.com/apsv/goal-tracker/backend/internal/db"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbType := flag.String("db-type", "sqlite", "Database type: sqlite or postgres")
	dbConn := flag.String("db", "", "Database connection (path for sqlite, URL for postgres). Defaults to sqlite path if empty.")
	flag.Parse()

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
		if *dbConn == "" {
			log.Fatal("PostgreSQL connection string required (use -db flag)")
		}
		log.Printf("Connecting to PostgreSQL database")
		database, err = db.NewPostgres(*dbConn)
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
	server := api.NewServer(database, staticFS)

	log.Printf("Starting server on %s", *addr)
	if staticFS != nil {
		log.Printf("Open http://localhost%s in your browser", *addr)
	} else {
		log.Printf("Running in dev mode (no embedded frontend)")
	}
	if err := http.ListenAndServe(*addr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
