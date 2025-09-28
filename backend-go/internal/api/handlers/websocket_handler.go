package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/npc"
	"agentic-npc-backend/internal/domain/npc_logic"
	"agentic-npc-backend/internal/dto"
	"agentic-npc-backend/internal/infra/grpc_client"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type EventMessage = dto.EventMessage

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	dbClient *ent.Client
	aiClient *grpc_client.AIClient
}

func NewWebSocketHandler(dbClient *ent.Client, aiClient *grpc_client.AIClient) *WebSocketHandler {
	return &WebSocketHandler{dbClient: dbClient, aiClient: aiClient}
}

func (h *WebSocketHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
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
	log.Println("Client connected via WebSocket")

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		var event EventMessage
		if err := json.Unmarshal(p, &event); err != nil {
			log.Println("Error unmarshalling event:", err)
			continue
		}
		log.Printf("Received event '%s' from '%s' for NPC '%s'", event.EventType, event.SourceEntityId, event.TargetNpcName)

		ctx := context.Background()

		targetNPC, err := h.dbClient.NPC.Query().Where(npc.NameEQ(event.TargetNpcName)).Only(ctx)
		if ent.IsNotFound(err) {
			log.Printf("NPC '%s' not found, creating new one...", event.TargetNpcName)
			targetNPC, err = h.dbClient.NPC.Create().SetName(event.TargetNpcName).Save(ctx)
		}
		if err != nil {
			log.Printf("Error finding/creating NPC: %v", err)
			continue
		}

		newEmotions := npc_logic.ModifyEmotionsOnEvent(event, targetNPC.Emotions)
		updatedNPC, err := targetNPC.Update().SetEmotions(newEmotions).Save(ctx)
		if err != nil {
			log.Printf("Error updating NPC emotions: %v", err)
			continue
		}
		log.Printf("Emotion state for '%s' updated.", updatedNPC.Name)

		// This is the full, correct code for creating a memory
		newMemory, err := h.dbClient.Memory.
			Create().
			SetEventType(event.EventType).
			SetParticipants([]string{event.SourceEntityId, updatedNPC.ID.String()}).
			SetDescription(fmt.Sprintf("%s triggered %s on %s", event.SourceEntityId, event.EventType, updatedNPC.Name)).
			SetOwner(updatedNPC).
			Save(ctx)
		if err != nil {
			log.Printf("Error creating memory: %v", err)
			continue
		}
		log.Printf("Successfully saved Memory ID %d.", newMemory.ID)

		recentMemories, err := updatedNPC.QueryMemories().Order(ent.Desc("created_at")).Limit(5).All(ctx)
		if err != nil {
			log.Printf("Error fetching recent memories: %v", err)
			continue
		}
		log.Printf("Fetched %d recent memories for NPC '%s'", len(recentMemories), updatedNPC.Name)

		actionResponse, err := h.aiClient.CallAIThink(event.EventType, updatedNPC.ID.String(), recentMemories, updatedNPC.Emotions, event.QuestionText)
		if err != nil {
			log.Println("Error calling AI service:", err)
			continue
		}
		log.Printf("Received action from AI service: %s", actionResponse.Content)

		responseMap := map[string]string{
			"action_type": actionResponse.ActionType,
			"content":     actionResponse.Content,
		}
		if err := conn.WriteJSON(responseMap); err != nil {
			log.Println("WebSocket write error:", err)
			break
		}
	}
}
