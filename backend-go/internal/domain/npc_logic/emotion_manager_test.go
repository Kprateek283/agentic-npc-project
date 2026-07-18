package npc_logic

import (
	"agentic-npc-backend/internal/db/ent/schema"
	"agentic-npc-backend/internal/dto"
	"math"
	"testing"
)

const eps = 1e-9

// emotionsEqual compares within a tolerance — emotion deltas are float64 sums, so exact
// == fails on values like 0.1+0.6 that are not representable.
func emotionsEqual(a, b schema.EmotionState) bool {
	return math.Abs(a.Joy-b.Joy) < eps && math.Abs(a.Sadness-b.Sadness) < eps &&
		math.Abs(a.Anger-b.Anger) < eps && math.Abs(a.Fear-b.Fear) < eps &&
		math.Abs(a.Trust-b.Trust) < eps
}

// fixture rules, so the test does not depend on the on-disk event_emotions.json.
func fixtureManager() *EmotionManager {
	return &EmotionManager{EventRules: map[string]EmotionDelta{
		"PLAYER_GAVE_GIFT": {Joy: 0.15},
		"PLAYER_ATTACKED":  {Joy: -1.0, Anger: 0.6, Fear: 0.4, Trust: -0.5},
		"PLAYER_INTERACT":  {Joy: 0.02, Trust: 0.01},
	}}
}

func TestModifyEmotionsOnEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		start     schema.EmotionState
		want      schema.EmotionState
	}{
		{
			name:      "gift adds joy",
			eventType: "PLAYER_GAVE_GIFT",
			start:     schema.EmotionState{Joy: 0.5, Trust: 0.5},
			want:      schema.EmotionState{Joy: 0.65, Trust: 0.5},
		},
		{
			name:      "attack applies multiple deltas",
			eventType: "PLAYER_ATTACKED",
			start:     schema.EmotionState{Joy: 0.5, Anger: 0.1, Fear: 0.1, Trust: 0.8},
			want:      schema.EmotionState{Joy: 0.0, Anger: 0.7, Fear: 0.5, Trust: 0.3},
		},
		{
			name:      "joy clamps at upper bound 1.0",
			eventType: "PLAYER_GAVE_GIFT",
			start:     schema.EmotionState{Joy: 0.95},
			want:      schema.EmotionState{Joy: 1.0},
		},
		{
			name:      "trust clamps at lower bound 0.0",
			eventType: "PLAYER_ATTACKED",
			start:     schema.EmotionState{Trust: 0.2},
			want:      schema.EmotionState{Joy: 0.0, Anger: 0.6, Fear: 0.4, Trust: 0.0},
		},
		{
			name:      "unknown event leaves emotions unchanged",
			eventType: "PLAYER_SANG_A_SONG",
			start:     schema.EmotionState{Joy: 0.3, Trust: 0.4},
			want:      schema.EmotionState{Joy: 0.3, Trust: 0.4},
		},
	}

	em := fixtureManager()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := em.ModifyEmotionsOnEvent(dto.EventMessage{EventType: tt.eventType}, &tt.start)
			if !emotionsEqual(*got, tt.want) {
				t.Errorf("got %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestModifyEmotionsDoesNotMutateInput(t *testing.T) {
	em := fixtureManager()
	start := schema.EmotionState{Joy: 0.5}
	em.ModifyEmotionsOnEvent(dto.EventMessage{EventType: "PLAYER_GAVE_GIFT"}, &start)
	if start.Joy != 0.5 {
		t.Errorf("input mutated: start.Joy = %v, want 0.5", start.Joy)
	}
}
