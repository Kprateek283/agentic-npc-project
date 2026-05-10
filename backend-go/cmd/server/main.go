package main

import (
	"agentic-npc-backend/internal/app" // <-- Import the new app package
	"log"
)

func main() {
	// 1. Create the application
	application, err := app.New()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// 2. Register deferred shutdown for all services
	defer application.Shutdown()

	// 3. Run the application
	if err := application.Run(); err != nil {
		log.Fatalf("Application failed to run: %v", err)
	}
}
