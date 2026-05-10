package handlers

import (
	"agentic-npc-backend/internal/db/ent"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	_ "agentic-npc-backend/internal/domain/npc_logic"
	_ "agentic-npc-backend/internal/dto"
	pb "agentic-npc-backend/internal/proto"
	"context"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// HandleGameEvent processes all in-game logic for an authenticated player.
func (h *WebSocketHandler) HandleGameEvent(conn *websocket.Conn, ctx context.Context, event EventMessage) {
	// 1. Process all game logic (quests, gifting, AND admin commands)
	failResponse, err := h.questManager.ProcessEvent(ctx, h.dbClient, h.redisClient, event)
	if err != nil {
		log.Printf("Error processing event in QuestManager: %v", err)
		h.sendError(conn, err.Error())
		return
	}
	
	// 2. Check for "Fail Response" from quest preconditions
	if failResponse != nil {
		log.Printf("Dungeon Master: Precondition failed. Sending fail-response: %s", failResponse.Content)
		// Send the ActionType and Content from the FailResponseAction struct
		h.sendSimpleResponse(conn, failResponse.ActionType, failResponse.Content)
		return
	}

	// 3. Check if it was an Admin command that succeeded
	if strings.HasPrefix(event.EventType, "ADMIN_") {
		log.Println("Dungeon Master: Admin command processed successfully.")
		h.sendSimpleResponse(conn, "ADMIN_ACK", "Admin command received and processed.")
		return
	}

	// 4. If it was a normal event, proceed to call the AI
	actionResponse, err := h.callAI(ctx, event)
	if err != nil {
		log.Println("Error calling AI service:", err)
		h.sendError(conn, "The AI is currently unavailable.")
		return
	}

	// 5. Send the AI's response back to the client.
	log.Printf("Received action from AI service: %s", actionResponse.Content)
	h.sendSimpleResponse(conn, actionResponse.ActionType, actionResponse.Content)
}

// callAI is a helper function to gather context and call the gRPC service
func (h *WebSocketHandler) callAI(ctx context.Context, event EventMessage) (*pb.ActionResponse, error) {
	// 4a. Get Target NPC
	targetNPC, err := h.questManager.GetNpc(ctx, h.dbClient, h.redisClient, event.TargetNpcName)
	if err != nil {
		log.Printf("Error finding NPC: %v", err)
		return nil, fmt.Errorf("target NPC not found")
	}

	// 4b. Get Player (needed for relationship)
	player, err := h.questManager.GetPlayer(ctx, h.dbClient, h.redisClient, event.SourceEntityId)
	if err != nil {
		log.Printf("Error finding Player: %v", err)
		return nil, fmt.Errorf("player not found")
	}

	// 4c. Modify NPC's base emotions
	newEmotions := h.emotionManager.ModifyEmotionsOnEvent(event, targetNPC.Emotions)
	updatedNPC, err := targetNPC.Update().SetEmotions(newEmotions).Save(ctx)
	if err != nil {
		log.Printf("Error updating NPC emotions: %v", err)
	}

	// 4d. Create Memory
	memoryDesc := fmt.Sprintf("%s triggered %s on %s", event.SourceEntityId, event.EventType, updatedNPC.Name)
	if event.EventType == "PLAYER_GAVE_GIFT" {
		memoryDesc = fmt.Sprintf("%s gave %s to %s", event.SourceEntityId, event.Keyword, updatedNPC.Name)
	}
	newMemory, err := h.dbClient.Memory.Create().
		SetEventType(event.EventType).
		SetParticipants([]string{event.SourceEntityId, updatedNPC.ID.String()}).
		SetDescription(memoryDesc). // <-- Use more descriptive memory
		SetOwner(updatedNPC).
		Save(ctx)
	if err != nil {
		log.Printf("Error creating memory: %v", err)
	} else {
		log.Printf("Successfully saved Memory ID %d.", newMemory.ID)
	}

	// 4e. Get Recent Memories
	recentMemories, err := updatedNPC.QueryMemories().Order(ent.Desc("created_at")).Limit(5).All(ctx)
	if err != nil {
		log.Printf("Error fetching recent memories: %v", err)
	}

	// 4f. Get Player Quest State
	currentQuestStep, completionRate := h.getPlayerQuestState(ctx, player) // Pass player obj

	// 4g. Get Player-Specific Trust
	rel, err := h.questManager.GetOrCreateRelationship(ctx, h.dbClient, h.redisClient, player, targetNPC)
	if err != nil {
		log.Printf("Error getting relationship: %v", err)
		return nil, err
	}

	// 4h. Get the NPC's emotions (the correct *schema.EmotionState type)
	aiEmotions := updatedNPC.Emotions
	// 4i. Overwrite the Trust field with the correct, player-specific value
	aiEmotions.Trust = rel.TrustLevel

	// Determine what text to send as the "main subject" of the event
	var textToSend string
	if event.EventType == "PLAYER_GAVE_GIFT" || event.EventType == "PLAYER_SUBMITTED_QUEST_ITEM" {
		textToSend = event.Keyword // Use the item name (keyword) for gifts and item submissions
	} else {
		textToSend = event.QuestionText // Use the question text for questions
	}

	// 5. Call the AI Service
	return h.aiClient.CallAIThink(
		ctx,
		updatedNPC.PersonalityPath,
		updatedNPC.BackstoryPath,
		updatedNPC.LorePath,
		aiEmotions, // <-- Pass the modified *schema.EmotionState struct
		recentMemories,
		event.EventType,
		textToSend, // <-- Pass the correct text
		event.SourceEntityId,
		currentQuestStep,
		completionRate,
	)
}

// getPlayerQuestState is a helper to find the active quest state for the AI context
func (h *WebSocketHandler) getPlayerQuestState(ctx context.Context, player *ent.Player) (int, float32) {
	// Player object is now passed in
	if player == nil {
		return 0, 0.0
	}

	activeQuest, err := player.QueryQuestStates().Where(entplayerqueststate.IsCompletedEQ(false)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		log.Printf("Error fetching active quest state: %v", err)
	} else if activeQuest != nil {
		return activeQuest.CurrentStep, activeQuest.CompletionRate
	}

	return 0, 0.0 // Default if no active quest
}
