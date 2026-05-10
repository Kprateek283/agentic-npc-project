package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// EmotionState holds the emotional values for an NPC.
// This will be stored as a JSON object in the database.
type EmotionState struct {
	Joy     float64 `json:"joy"`
	Sadness float64 `json:"sadness"`
	Anger   float64 `json:"anger"`
	Fear    float64 `json:"fear"`
	Trust   float64 `json:"trust"`
}

// NPC holds the schema definition for the NPC entity.
type NPC struct {
	ent.Schema
}

// Fields of the NPC.
func (NPC) Fields() []ent.Field {
	return []ent.Field{
		// The unique ID for this NPC
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Unique(),

		// The unique in-game name, e.g., "Elian", "Baelor"
		field.String("name").
			NotEmpty().
			Unique(),

		// The "occupation" or role, e.g., "Herbalist", "Blacksmith"
		field.String("npc_type").
			Default("villager"),

		// --- STATIC DATA PATHS (Your New Design) ---
		// The Python AI service will load these files on startup.

		// Path to the .txt file containing the NPC's personality.
		field.String("personality_path").
			NotEmpty().
			Comment("The file path to the NPC's static personality.json file."),

		// Path to the .txt file containing the NPC's backstory.
		field.String("backstory_path").
			NotEmpty().
			Comment("The file path to the NPC's static backstory.json file."),

		// Path to the .txt file containing the NPC's specific lore knowledge.
		field.String("lore_path").
			NotEmpty().
			Comment("The file path to the NPC's static lore.json file."),

		// --- DYNAMIC STATE ---

		// The NPC's current, changing emotional state.
		field.JSON("emotions", new(EmotionState)).
			Default(new(EmotionState)),

		// The NPC's current, changing list of active goals.
		field.Strings("current_goals").
			Optional(),
	}
}

// Edges of the NPC.
func (NPC) Edges() []ent.Edge {
	return []ent.Edge{
		// An NPC can have many memories.
		edge.To("memories", Memory.Type),

		// --- THIS IS THE FIX ---
		// An NPC can have many relationships (one for each player).
		edge.To("player_relationships", PlayerNPCRelationship.Type),
	}
}
