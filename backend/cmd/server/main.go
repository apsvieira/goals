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
	dbPath := flag.String("db", db.DefaultDBPath(), "SQLite database path")
	flag.Parse()

	log.Printf("Opening database at %s", *dbPath)
	database, err := db.New(*dbPath)
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
