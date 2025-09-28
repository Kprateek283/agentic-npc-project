// Package database internal/infra/database/postgres.go
package database

import (
	"context"
	"log"

	"agentic-npc-backend/internal/config"
	"agentic-npc-backend/internal/db/ent" // Import the generated ent client

	_ "github.com/lib/pq" // The PostgreSQL driver
)

// NewClient creates and returns a new ent client.
func NewClient(cfg *config.Config) *ent.Client {
	client, err := ent.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	log.Println("Successfully connected to PostgresSQL")

	// Run the auto migration tool to create all schema resources.
	if err := client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	log.Println("Successfully ran database auto-migration")

	return client
}
