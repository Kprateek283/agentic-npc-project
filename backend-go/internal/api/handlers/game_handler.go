package handlers

import (
	"agentic-npc-backend/internal/db/ent"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	"agentic-npc-backend/internal/db/ent/schema"
	_ "agentic-npc-backend/internal/domain/npc_logic"
	_ "agentic-npc-backend/internal/dto"
	pb "agentic-npc-backend/internal/proto"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	_ "github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"
)

// newRequestID returns a short random hex id used to correlate the Go and Python
// log lines for one conversation event.
func newRequestID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// HandleGameEvent processes all in-game logic for an authenticated player.
func (h *WebSocketHandler) HandleGameEvent(conn *websocket.Conn, ctx context.Context, event EventMessage) {
	reqID := newRequestID()
	start := time.Now()

	// 1. Process all game logic (quests, gifting, AND admin commands)
	questStart := time.Now()
	failResponse, err := h.questManager.ProcessEvent(ctx, h.dbClient, h.redisClient, event)
	questMs := time.Since(questStart).Milliseconds()
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

	// 4. Normal event -> stream the AI response (C3). RAG answers stream token-by-token
	// (SPEAK_PARTIAL frames) and close with a SPEAK frame; other events arrive as a single
	// frame. streamAI returns a non-nil error only when nothing streamed at all — in that
	// case fall back to a graceful in-character line (C2), so the player is never left with
	// an error or silence and the NPC recovers automatically once the service is back.
	grpcMs, err := h.streamAI(conn, ctx, reqID, event)
	if err != nil {
		slog.Warn("ai_stream_failed", "req_id", reqID, "npc", event.TargetNpcName,
			"event", event.EventType, "err", err.Error())
		h.sendSimpleResponse(conn, "SPEAK",
			"Hmm? Forgive me — my mind wandered just now. Ask me again in a moment.")
		return
	}

	// One correlated summary line per event: the M4 latency attribution, made continuous.
	// grpc_ms is the streaming RPC (first token to last); total_ms is the whole handler.
	slog.Info("game_event",
		"req_id", reqID,
		"player", event.SourceEntityId,
		"npc", event.TargetNpcName,
		"event", event.EventType,
		"quest_ms", questMs,
		"grpc_ms", grpcMs,
		"total_ms", time.Since(start).Milliseconds(),
	)
}

// aiRequestArgs is the fully-gathered per-event context the AI service needs.
type aiRequestArgs struct {
	personalityPath string
	backstoryPath   string
	lorePath        string
	emotions        *schema.EmotionState
	memories        []*ent.Memory
	eventType       string
	text            string
	sourceEntityId  string
	questStep       int
	completionRate  float32
}

// gatherAIContext runs the per-event side effects (emotion update, memory write) and
// collects everything the AI service needs. Shared by the unary and streaming paths so
// they cannot drift; it must run exactly once per event.
func (h *WebSocketHandler) gatherAIContext(ctx context.Context, event EventMessage) (*aiRequestArgs, error) {
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
		SetDescription(memoryDesc).
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
	currentQuestStep, completionRate := h.getPlayerQuestState(ctx, player)

	// 4g. Get Player-Specific Trust
	rel, err := h.questManager.GetOrCreateRelationship(ctx, h.dbClient, h.redisClient, player, targetNPC)
	if err != nil {
		log.Printf("Error getting relationship: %v", err)
		return nil, err
	}

	// 4h/4i. The NPC's emotions with the player-specific trust overlaid.
	aiEmotions := updatedNPC.Emotions
	aiEmotions.Trust = rel.TrustLevel

	// Main subject text: the item name for gifts/submissions, else the question text.
	text := event.QuestionText
	if event.EventType == "PLAYER_GAVE_GIFT" || event.EventType == "PLAYER_SUBMITTED_QUEST_ITEM" {
		text = event.Keyword
	}

	return &aiRequestArgs{
		personalityPath: updatedNPC.PersonalityPath,
		backstoryPath:   updatedNPC.BackstoryPath,
		lorePath:        updatedNPC.LorePath,
		emotions:        aiEmotions,
		memories:        recentMemories,
		eventType:       event.EventType,
		text:            text,
		sourceEntityId:  event.SourceEntityId,
		questStep:       currentQuestStep,
		completionRate:  completionRate,
	}, nil
}

// callAI is the unary path: gather context, call Think, return the response and the RPC's
// own duration. Kept for the benchmark harness and as the non-streaming fallback.
func (h *WebSocketHandler) callAI(ctx context.Context, reqID string, event EventMessage) (*pb.ActionResponse, int64, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "req-id", reqID)
	a, err := h.gatherAIContext(ctx, event)
	if err != nil {
		return nil, 0, err
	}
	grpcStart := time.Now()
	resp, err := h.aiClient.CallAIThink(ctx, a.personalityPath, a.backstoryPath, a.lorePath,
		a.emotions, a.memories, a.eventType, a.text, a.sourceEntityId, a.questStep, a.completionRate)
	return resp, time.Since(grpcStart).Milliseconds(), err
}

// streamAI is the streaming path (C3): gather context, open ThinkStream, forward each text
// delta as a SPEAK_PARTIAL frame, and close with a SPEAK frame carrying the full text.
// It returns a non-nil error only when the stream never produced anything (so the caller
// can fall back); a mid-stream failure is finalized best-effort with the partial text.
func (h *WebSocketHandler) streamAI(conn *websocket.Conn, ctx context.Context, reqID string, event EventMessage) (int64, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "req-id", reqID)
	a, err := h.gatherAIContext(ctx, event)
	if err != nil {
		return 0, err
	}

	sent := 0
	grpcStart := time.Now()
	full, err := h.aiClient.CallAIThinkStream(ctx, a.personalityPath, a.backstoryPath, a.lorePath,
		a.emotions, a.memories, a.eventType, a.text, a.sourceEntityId, a.questStep, a.completionRate,
		func(tok string) {
			sent++
			h.sendSimpleResponse(conn, "SPEAK_PARTIAL", tok)
		})
	grpcMs := time.Since(grpcStart).Milliseconds()

	if err != nil && sent == 0 {
		// Nothing streamed — signal the caller to fall back to the in-character line.
		return grpcMs, err
	}
	if err != nil {
		log.Printf("[stream] errored after %d partial(s); finalizing with partial text: %v", sent, err)
	}
	// Final frame carries the whole content (non-streaming clients can ignore partials).
	h.sendSimpleResponse(conn, "SPEAK", full)
	return grpcMs, nil
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
