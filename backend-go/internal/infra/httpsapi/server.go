package httpsapi

import (
	"agentic-npc-backend/internal/api/handlers"
	"agentic-npc-backend/internal/api/router"

	"github.com/gin-gonic/gin"
)

// NewServer now accepts the handlers instead of all the clients.
func NewServer(
	healthHandler gin.HandlerFunc,
	wsHandler *handlers.WebSocketHandler,
) *gin.Engine {
	r := gin.Default()
	// Pass the handlers directly to the router
	router.SetupRouter(r, healthHandler, wsHandler)
	return r
}
