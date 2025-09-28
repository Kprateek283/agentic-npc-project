package main

import (
	"agentic-npc-backend/internal/db/ent"
	"fmt"
	"log"

	"agentic-npc-backend/internal/config"
	"agentic-npc-backend/internal/infra/database" // Import our new package
	"agentic-npc-backend/internal/infra/grpc_client"
	"agentic-npc-backend/internal/infra/httpserver"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}
	//

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	log.Printf("Configuration loaded successfully")

	// Create the database client and run migrations
	dbClient := database.NewClient(cfg)
	defer func(dbClient *ent.Client) {
		err := dbClient.Close()
		if err != nil {

		}
	}(dbClient) // Ensure the client is closed gracefully on exit

	// Create the AI gRPC client
	aiClient, err := grpc_client.NewAIClient("localhost:50051")
	if err != nil {
		panic(fmt.Sprintf("failed to create AI client: %v", err))
	}
	log.Printf("Successfully connected to AI gRPC server")

	// Pass BOTH clients to the server
	server := httpserver.NewServer(dbClient, aiClient)

	err = server.Run(":8080")
	if err != nil {
		panic("failed to start server")
	}
}
