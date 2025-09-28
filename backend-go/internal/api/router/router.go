package router

import (
	"agentic-npc-backend/internal/api/handlers"
	"agentic-npc-backend/internal/db/ent" // Import ent client
	"agentic-npc-backend/internal/infra/grpc_client"

	"github.com/gin-gonic/gin"
)

// SetupRouter Accept both clients
func SetupRouter(router *gin.Engine, dbClient *ent.Client, aiClient *grpc_client.AIClient) {
	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/health", handlers.HealthHandler)

		// Create a handler instance with both clients
		wsHandler := handlers.NewWebSocketHandler(dbClient, aiClient)
		apiV1.GET("/ws", wsHandler.Handle)
	}
}
