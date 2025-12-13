package handlers

import (
	"context"
	"database/sql"
	"log"
	
	"strings"
	
	"maragu.dev/goqite"
)

// Queue is the persistent job queue
var Queue *goqite.Queue

// InitQueue initializes the persistent queue
func InitQueue(db *sql.DB) {
	// 1. Setup Table
	// 1. Setup Table
	if err := goqite.Setup(context.Background(), db); err != nil {
		// Ignore if table already exists
		if !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("Failed to setup queue table: %v", err)
		}
	}

	// 2. Initialize Queue
	Queue = goqite.New(goqite.NewOpts{
		DB:   db,
		Name: "regis_jobs",
	})
}
