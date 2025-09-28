package npc_logic

import (
	"agentic-npc-backend/internal/db/ent/schema"
	"agentic-npc-backend/internal/dto" // <-- CORRECTED IMPORT
	"math"
)

// clamp is a helper function to ensure emotion values stay between 0.0 and 1.0
func clamp(value float64) float64 {
	return math.Max(0.0, math.Min(1.0, value))
}

// ModifyEmotionsOnEvent now uses dto.EventMessage
func ModifyEmotionsOnEvent(event dto.EventMessage, currentEmotions *schema.EmotionState) *schema.EmotionState {
	newEmotions := *currentEmotions

	switch event.EventType {
	case "PLAYER_INTERACT":
		newEmotions.Trust = clamp(newEmotions.Trust + 0.05)
		newEmotions.Joy = clamp(newEmotions.Joy + 0.02)

	case "PLAYER_GAVE_GIFT":
		newEmotions.Trust = clamp(newEmotions.Trust + 0.2)
		newEmotions.Joy = clamp(newEmotions.Joy + 0.15)

	case "PLAYER_ATTACKED":
		newEmotions.Trust = clamp(newEmotions.Trust - 0.5)
		newEmotions.Fear = clamp(newEmotions.Fear + 0.4)
		newEmotions.Anger = clamp(newEmotions.Anger + 0.6)
		newEmotions.Joy = 0
	}

	return &newEmotions
}
