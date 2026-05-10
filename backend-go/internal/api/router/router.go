package router

import (
	"agentic-npc-backend/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRouter now accepts the specific handlers it needs, not all the clients.
func SetupRouter(
	router *gin.Engine,
	healthHandler gin.HandlerFunc,
	wsHandler *handlers.WebSocketHandler, // Pass the handler instance
) {
	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/health", healthHandler)
		apiV1.GET("/ws", wsHandler.Handle) // Use the passed handler
	}
}
