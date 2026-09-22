package handlers

import (
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/domain/npc_logic"
	"agentic-npc-backend/internal/domain/quest_logic"
	"agentic-npc-backend/internal/dto"
	"agentic-npc-backend/internal/infra/grpc_client"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

type EventMessage = dto.EventMessage

func originAllowed(origin, host string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	if len(allowed) == 0 {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host != "" && strings.EqualFold(u.Host, host)
	}
	return false
}

// WebSocketHandler holds all clients and services
type WebSocketHandler struct {
	dbClient       *ent.Client
	aiClient       *grpc_client.AIClient
	questManager   *quest_logic.QuestManager
	redisClient    *redis.Client
	emotionManager *npc_logic.EmotionManager
	upgrader       websocket.Upgrader
	llmRateLimit   int
	llmRateWindow  time.Duration
}

// NewWebSocketHandler creates a new handler with all dependencies
func NewWebSocketHandler(
	dbClient *ent.Client,
	aiClient *grpc_client.AIClient,
	questManager *quest_logic.QuestManager,
	redisClient *redis.Client,
	emotionManager *npc_logic.EmotionManager,
	allowedOrigins []string,
	llmRateLimit int,
	llmRateWindow time.Duration,
) *WebSocketHandler {
	return &WebSocketHandler{
		dbClient:       dbClient,
		aiClient:       aiClient,
		questManager:   questManager,
		redisClient:    redisClient,
		emotionManager: emotionManager,
		llmRateLimit:   llmRateLimit,
		llmRateWindow:  llmRateWindow,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return originAllowed(r.Header.Get("Origin"), r.Host, allowedOrigins)
			},
		},
	}
}

// Handle manages the WebSocket connection lifecycle
func (h *WebSocketHandler) Handle(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to set websocket upgrade: %+v", err)
		return
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Printf("Error closing WebSocket connection: %v", err)
		}
	}()
	log.Println("Client connected via WebSocket. Awaiting authentication...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Buffer allows the reader to continue reading to detect client disconnects while a request is in-flight; flooding >16 queued messages simply waits.
	msgChan := make(chan []byte, 16)

	go func() {
		defer close(msgChan)
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				log.Println("Client disconnected; cancelling in-flight work")
				cancel()
				return
			}
			select {
			case msgChan <- p:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Connection-specific state
	var isAuthenticated bool = false
	var currentPlayerID string = ""

	for p := range msgChan {
		var event EventMessage
		if err := json.Unmarshal(p, &event); err != nil {
			log.Println("Error unmarshalling event:", err)
			continue
		}
		log.Printf("Received event '%s'", event.EventType)

		if !isAuthenticated {
			// --- Authentication Logic (Delegated) ---
			playerID, authenticated, authErr := h.HandleAuthEvent(conn, ctx, event)
			if authErr != nil {
				h.sendError(conn, authErr.Error())
				continue
			}
			isAuthenticated = authenticated
			currentPlayerID = playerID

		} else {
			// --- Authenticated Game Logic (Delegated) ---
			event.SourceEntityId = currentPlayerID
			h.HandleGameEvent(conn, ctx, event)
		}
	}
}

// sendError sends a structured error message back to the client.
func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	responseMap := map[string]string{
		"action_type": "ERROR",
		"content":     message,
	}
	if err := conn.WriteJSON(responseMap); err != nil {
		log.Printf("Error sending error to client: %v", err)
	}
}

// sendSimpleResponse sends a non-error message (like SPEAK or ADMIN_ACK)
func (h *WebSocketHandler) sendSimpleResponse(conn *websocket.Conn, actionType string, content string) {
	responseMap := map[string]string{
		"action_type": actionType,
		"content":     content,
	}
	if err := conn.WriteJSON(responseMap); err != nil {
		log.Println("WebSocket write error:", err)
	}
}
