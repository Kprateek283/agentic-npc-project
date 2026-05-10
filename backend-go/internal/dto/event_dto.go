package dto

// EventMessage struct is our single data structure for all incoming WebSocket events.
// It includes fields for all possible event types.
type EventMessage struct {
	// Core event fields
	EventType      string `json:"event_type"`
	SourceEntityId string `json:"source_entity_id"` // This will be the Player's username after login
	TargetNpcName  string `json:"target_npc_name"`

	// Fields for "PLAYER_ASKED_QUESTION" or item-related events
	QuestionText string `json:"question_text,omitempty"`
	Keyword      string `json:"keyword,omitempty"` // Used for quest items or gifting

	// --- NEW FIELDS (Task 1.1) ---
	// Fields for "LOGIN_PLAYER" or "REGISTER_PLAYER" events
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// ---------------------------
}
