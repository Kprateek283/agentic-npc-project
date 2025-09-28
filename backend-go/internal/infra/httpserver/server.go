package httpserver

import (
	"agentic-npc-backend/internal/api/router"
	"agentic-npc-backend/internal/db/ent" // Import ent client
	"agentic-npc-backend/internal/infra/grpc_client"

	"github.com/gin-gonic/gin"
)

// NewServer Accept both clients
func NewServer(dbClient *ent.Client, aiClient *grpc_client.AIClient) *gin.Engine {
	r := gin.Default()
	// Pass both clients to the router
	router.SetupRouter(r, dbClient, aiClient)
	return r
}
