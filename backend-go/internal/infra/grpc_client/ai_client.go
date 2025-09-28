package grpc_client

import (
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/schema"
	"context"
	//"log"

	pb "agentic-npc-backend/internal/proto" // <-- THIS IS THE CRITICAL FIX

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AIClient is a client that communicates with the Python AI service.
type AIClient struct {
	client pb.AIBrainClient
}

// NewAIClient creates a new gRPC client for the AI service.
func NewAIClient(address string) (*AIClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewAIBrainClient(conn)
	return &AIClient{client: client}, nil
}

// CallAIThink sends an event to the AI service and returns the resulting action.
// Replace the existing CallAIThink function with this one.
// Replace the existing CallAIThink function with this one
// Replace the entire CallAIThink function.
func (c *AIClient) CallAIThink(
	eventType string,
	npcID string,
	memories []*ent.Memory,
	emotions *schema.EmotionState,
	questionText string, // <-- ADD THE NEW ARGUMENT HERE
) (*pb.ActionResponse, error) {

	var grpcMemories []*pb.MemoryMessage
	for _, mem := range memories {
		grpcMemories = append(grpcMemories, &pb.MemoryMessage{
			Description:  mem.Description,
			Importance:   mem.Importance,
			EventType:    mem.EventType,
			Participants: mem.Participants,
		})
	}

	grpcEmotions := &pb.EmotionStateMessage{
		Joy:     emotions.Joy,
		Sadness: emotions.Sadness,
		Anger:   emotions.Anger,
		Fear:    emotions.Fear,
		Trust:   emotions.Trust,
	}

	req := &pb.EventRequest{
		EventType:       eventType,
		TargetNpcId:     npcID,
		RecentMemories:  grpcMemories,
		CurrentEmotions: grpcEmotions,
		QuestionText:    questionText, // <-- ADD THE QUESTION TO THE REQUEST
	}

	res, err := c.client.Think(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
