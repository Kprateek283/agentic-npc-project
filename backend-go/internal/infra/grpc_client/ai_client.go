// Package grpc_client handles the gRPC communication with the Python AI service.
package grpc_client

import (
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/schema"
	"context"
	"log"
	"os"
	"strconv"
	"time"

	pb "agentic-npc-backend/internal/proto" // Our generated gRPC code

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// AIClient communicates with the Python AI service.
type AIClient struct {
	client  pb.AIBrainClient
	timeout time.Duration
}

// NewAIClient creates a new gRPC client for the AI service. The per-call deadline is
// AI_CALL_TIMEOUT seconds (default 40 — a few seconds beyond worst-case local Ollama
// inference, ~26s, so slow local runs are not cut off).
func NewAIClient(address string) (*AIClient, error) {
	// grpc.NewClient is the current constructor (grpc.Dial is deprecated); it dials
	// lazily, so a down AI service surfaces as an Unavailable RPC error, not here.
	// Cap reconnect backoff at 5s (default max is 120s) so the channel re-establishes
	// within a few seconds of the AI service coming back — no Go restart needed.
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.Config{BaseDelay: time.Second, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 5 * time.Second},
			MinConnectTimeout: 5 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}

	timeout := 40 * time.Second
	if v := os.Getenv("AI_CALL_TIMEOUT"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	client := pb.NewAIBrainClient(conn)
	return &AIClient{client: client, timeout: timeout}, nil
}

// callThink enforces the per-call deadline and retries once on Unavailable (a transient
// blip or an AI-service restart). It never retries DeadlineExceeded — the LLM call is
// expensive and a hung call must not be double-billed.
func (c *AIClient) callThink(ctx context.Context, req *pb.EventRequest) (*pb.ActionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	res, err := c.client.Think(ctx, req)
	if err != nil && status.Code(err) == codes.Unavailable {
		log.Printf("[gRPC Client] AI service Unavailable, retrying once: %v", err)
		time.Sleep(200 * time.Millisecond)
		res, err = c.client.Think(ctx, req)
	}
	return res, err
}

// CallAIThink sends all dynamic context and the agent config paths to the AI service.
// This function is now refactored to match our new 'proto/ai.proto' contract.
func (c *AIClient) CallAIThink(
	ctx context.Context, // Add context for better request management
	personalityPath string,
	backstoryPath string,
	lorePath string,
	emotions *schema.EmotionState,
	memories []*ent.Memory,
	eventType string,
	questionText string,
	sourceEntityId string,
	currentQuestStep int, // <-- NEW (Task 5.2)
	completionRate float32, // <-- NEW (Task 5.2)
) (*pb.ActionResponse, error) {

	log.Println("[gRPC Client] Packing dynamic context for AI service...")

	// 1. Convert Go's *schema.EmotionState struct to the gRPC *pb.EmotionStateMessage
	grpcEmotions := &pb.EmotionStateMessage{
		Joy:     emotions.Joy,
		Sadness: emotions.Sadness,
		Anger:   emotions.Anger,
		Fear:    emotions.Fear,
		Trust:   emotions.Trust,
	}
	log.Printf("[gRPC Client] Packed Emotions: Anger=%.2f, Trust=%.2f", grpcEmotions.Anger, grpcEmotions.Trust)

	// 2. Convert Go's []*ent.Memory slice to the gRPC []*pb.MemoryMessage slice
	var grpcMemories []*pb.MemoryMessage
	for _, mem := range memories {
		grpcMemories = append(grpcMemories, &pb.MemoryMessage{
			Description:  mem.Description,
			Importance:   mem.Importance,
			EventType:    mem.EventType,
			Participants: mem.Participants,
		})
	}
	log.Printf("[gRPC Client] Packed %d recent memories.", len(grpcMemories))

	// 3. Construct the new, lightweight EventRequest
	// This sends the "keys" (the paths) and the dynamic "payload".
	req := &pb.EventRequest{
		PersonalityPath:  personalityPath,
		BackstoryPath:    backstoryPath,
		LorePath:         lorePath,
		CurrentEmotions:  grpcEmotions,
		RecentMemories:   grpcMemories,
		EventType:        eventType,
		QuestionText:     questionText,
		SourceEntityId:   sourceEntityId,
		CurrentQuestStep: int32(currentQuestStep), // <-- NEW (Task 5.2)
		CompletionRate:   completionRate,          // <-- NEW (Task 5.2)
	}
	log.Printf("[gRPC Client] Sending request to Python agent for Event: %s", eventType)

	// 4. Call the gRPC service (deadline + retry-once handled in callThink) and return.
	res, err := c.callThink(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] Error from AI service: %v", err)
		return nil, err
	}
	log.Println("[gRPC Client] Received response from AI service.")

	return res, nil
}
