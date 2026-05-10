package npc_logic

import (
	"agentic-npc-backend/internal/db/ent/schema"
	"agentic-npc-backend/internal/dto"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
)

// clamp is a helper function to ensure emotion values stay between 0.0 and 1.0
func clamp(value float64) float64 {
	return math.Max(0.0, math.Min(1.0, value))
}

// EmotionDelta defines the changes for a single event from our JSON file
type EmotionDelta struct {
	Joy     float64 `json:"joy,omitempty"`
	Sadness float64 `json:"sadness,omitempty"`
	Anger   float64 `json:"anger,omitempty"`
	Fear    float64 `json:"fear,omitempty"`
	Trust   float64 `json:"trust,omitempty"`
}

// EmotionManager holds the loaded rules from the JSON file
type EmotionManager struct {
	EventRules map[string]EmotionDelta
}

// NewEmotionManager loads the event emotion rules from gamedata
func NewEmotionManager(gamedataPath string) (*EmotionManager, error) {
	filePath := filepath.Join(gamedataPath, "event_emotions.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read event_emotions.json: %w", err)
	}

	var rules map[string]EmotionDelta
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse event_emotions.json: %w", err)
	}

	log.Printf("Successfully loaded %d event emotion rules.", len(rules))
	return &EmotionManager{EventRules: rules}, nil
}

// ModifyEmotionsOnEvent is now a method on EmotionManager that uses the loaded rules
func (em *EmotionManager) ModifyEmotionsOnEvent(event dto.EventMessage, currentEmotions *schema.EmotionState) *schema.EmotionState {
	newEmotions := *currentEmotions

	// Look up the rule for the event
	delta, ok := em.EventRules[event.EventType]
	if !ok {
		// No rule found for this event, return unchanged emotions
		return &newEmotions
	}

	// Apply the deltas from the JSON file
	newEmotions.Joy = clamp(newEmotions.Joy + delta.Joy)
	newEmotions.Sadness = clamp(newEmotions.Sadness + delta.Sadness)
	newEmotions.Anger = clamp(newEmotions.Anger + delta.Anger)
	newEmotions.Fear = clamp(newEmotions.Fear + delta.Fear)
	newEmotions.Trust = clamp(newEmotions.Trust + delta.Trust)

	return &newEmotions
}
