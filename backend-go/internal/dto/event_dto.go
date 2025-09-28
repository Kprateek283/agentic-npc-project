package dto

// EventMessage struct is updated to include the question text.
type EventMessage struct {
	EventType      string `json:"event_type"`
	SourceEntityId string `json:"source_entity_id"`
	TargetNpcName  string `json:"target_npc_name"`
	QuestionText   string `json:"question_text,omitempty"` // <-- ADD THIS LINE
}
