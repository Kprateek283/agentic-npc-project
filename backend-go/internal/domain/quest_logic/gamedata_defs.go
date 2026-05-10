package quest_logic

// --- Structs to hold our static gamedata ---

// ItemDefinition holds the static definition of a game item from items.json
type ItemDefinition struct {
	ItemID         string  `json:"item_id"`
	Name           string  `json:"name"`
	Rarity         string  `json:"rarity"`
	BaseTrustValue float64 `json:"base_trust_value"`
	QuestItem      bool    `json:"quest_item"`
}

// QuestDefinition holds the static definition of a quest from sq_*.json
type QuestDefinition struct {
	QuestID     string               `json:"quest_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	QuestGiver  string               `json:"quest_giver_name"`
	Steps       map[string]QuestStep `json:"steps"`
}

// FailResponseAction --- ADD THIS NEW STRUCT ---
// FailResponseAction defines the structure for fail responses in JSON
type FailResponseAction struct {
	ActionType string `json:"action_type"`
	Content    string `json:"content"`
}

// QuestStep defines a single step within a quest
type QuestStep struct {
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	CompletionTrigger CompletionTrigger  `json:"completion_trigger"`
	Preconditions     []Precondition     `json:"preconditions,omitempty"`
	FailResponse      FailResponseAction `json:"fail_response,omitempty"` // <-- FIX: Use the new struct
	Rewards           Rewards            `json:"rewards,omitempty"`
}

// CompletionTrigger defines what event completes a quest step
type CompletionTrigger struct {
	EventType     string `json:"event_type"`
	TargetNPCName string `json:"target_npc_name"`
	Keyword       string `json:"keyword"`
}

// Precondition defines a check that must pass for a step to be completed
type Precondition struct {
	Type     string  `json:"type"`
	QuestID  string  `json:"quest_id,omitempty"`
	Emotion  string  `json:"emotion,omitempty"`
	Operator string  `json:"operator,omitempty"` // Renamed from "comparison" in old files
	Value    float64 `json:"value,omitempty"`
}

// Rewards defines what the player receives for completing a step
type Rewards struct {
	XP                 int                `json:"xp"`
	UnlocksQuestID     string             `json:"unlocks_quest_id,omitempty"`
	UnlocksStep        string             `json:"unlocks_step,omitempty"`
	RelationshipChange RelationshipChange `json:"relationship_change,omitempty"`
}

// RelationshipChange defines a trust modification reward
type RelationshipChange struct {
	TargetNPCName string `json:"target_npc_name"`
	Trust         string `json:"trust"`
}
