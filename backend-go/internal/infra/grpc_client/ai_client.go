// Package grpc_client handles the gRPC communication with the Python AI service.
package grpc_client

import (
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/schema"
	"context"
	"log"

	pb "agentic-npc-backend/internal/proto" // Our generated gRPC code

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AIClient communicates with the Python AI service.
type AIClient struct {
	client pb.AIBrainClient
}

// NewAIClient creates a new gRPC client for the AI service.
func NewAIClient(address string) (*AIClient, error) {
	// Use grpc.Dial as grpc.NewClient is deprecated.
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewAIBrainClient(conn)
	return &AIClient{client: client}, nil
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

	// 4. Call the gRPC service and return the response
	res, err := c.client.Think(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] Error from AI service: %v", err)
		return nil, err
	}
	log.Println("[gRPC Client] Received response from AI service.")

	return res, nil
}
