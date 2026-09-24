// Package grpc_client handles the gRPC communication with the Python AI service.
package grpc_client

import (
	"context"
	"io"
	"log"
	"strings"
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
// set by the timeout parameter (typically configured via AI_CALL_TIMEOUT).
func NewAIClient(address string, timeout time.Duration) (*AIClient, error) {
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

// buildEventRequest converts the Go-side context into the gRPC EventRequest. Shared by
// the unary (CallAIThink) and streaming (CallAIThinkStream) paths so they cannot drift.
func buildEventRequest(
	personalityPath string,
	backstoryPath string,
	lorePath string,
	speakerEmotions map[string]float64,
	generalMood map[string]float64,
	memoryLines []string,
	eventType string,
	questionText string,
	sourceEntityId string,
	currentQuestStep int,
	completionRate float32,
) *pb.EventRequest {
	protoSpeakerEmotions := make(map[string]float32, len(speakerEmotions))
	for k, v := range speakerEmotions {
		protoSpeakerEmotions[k] = float32(v)
	}

	protoGeneralMood := make(map[string]float32, len(generalMood))
	for k, v := range generalMood {
		protoGeneralMood[k] = float32(v)
	}

	return &pb.EventRequest{
		PersonalityPath:  personalityPath,
		BackstoryPath:    backstoryPath,
		LorePath:         lorePath,
		SpeakerEmotions:  protoSpeakerEmotions,
		GeneralMood:      protoGeneralMood,
		MemoryLines:      memoryLines,
		EventType:        eventType,
		QuestionText:     questionText,
		SourceEntityId:   sourceEntityId,
		CurrentQuestStep: int32(currentQuestStep),
		CompletionRate:   completionRate,
	}
}

// CallAIThink sends all dynamic context and the agent config paths to the AI service
// and returns the single unary response.
func (c *AIClient) CallAIThink(
	ctx context.Context,
	personalityPath string,
	backstoryPath string,
	lorePath string,
	speakerEmotions map[string]float64,
	generalMood map[string]float64,
	memoryLines []string,
	eventType string,
	questionText string,
	sourceEntityId string,
	currentQuestStep int,
	completionRate float32,
) (*pb.ActionResponse, error) {
	req := buildEventRequest(personalityPath, backstoryPath, lorePath, speakerEmotions, generalMood, memoryLines,
		eventType, questionText, sourceEntityId, currentQuestStep, completionRate)

	res, err := c.callThink(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] Error from AI service: %v", err)
		return nil, err
	}
	return res, nil
}

// CallAIThinkStream sends the same request but consumes a server stream, invoking onToken
// for each text delta as it arrives (C3). It returns the full concatenated content once
// the stream completes, and the action type from the done frame ("SPEAK" if none arrives). No retry: a mid-stream failure returns whatever streamed so far
// plus the error, and the caller decides how to finish.
func (c *AIClient) CallAIThinkStream(
	ctx context.Context,
	personalityPath string,
	backstoryPath string,
	lorePath string,
	speakerEmotions map[string]float64,
	generalMood map[string]float64,
	memoryLines []string,
	eventType string,
	questionText string,
	sourceEntityId string,
	currentQuestStep int,
	completionRate float32,
	onToken func(string),
) (string, string, error) {
	req := buildEventRequest(personalityPath, backstoryPath, lorePath, speakerEmotions, generalMood, memoryLines,
		eventType, questionText, sourceEntityId, currentQuestStep, completionRate)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	action := "SPEAK"
	stream, err := c.client.ThinkStream(ctx, req)
	if err != nil {
		return "", action, err
	}

	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return full.String(), action, err
		}
		// The done frame may carry text too; keep it before stopping.
		if chunk.Text != "" {
			full.WriteString(chunk.Text)
			onToken(chunk.Text)
		}
		if chunk.Done {
			if chunk.ActionType != "" {
				action = chunk.ActionType
			}
			break
		}
	}
	return full.String(), action, nil
}
