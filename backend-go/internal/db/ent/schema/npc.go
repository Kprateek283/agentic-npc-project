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
		// The primary key will be a UUID for uniqueness across systems.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Unique(),

		// The NPC's given name.
		field.String("name").
			NotEmpty(),

		// The NPC's type or role, e.g., "merchant", "guard".
		field.String("npc_type").
			Default("villager"),

		// We will store the complex EmotionState struct as a JSON field.
		// `ent` will automatically handle marshalling/unmarshalling.
		// This is the new version
		field.JSON("emotions", &EmotionState{}).
			Default(new(EmotionState)), // Set a default value for new NPCs

		// We will store the NPC's current goals as an array of strings.
		// This will also be stored as a JSON array.
		field.Strings("current_goals").
			Optional(),
	}
}

// Edges of the NPC.
func (NPC) Edges() []ent.Edge {
	// This defines a "one-to-many" relationship.
	// One NPC can have many memories.
	return []ent.Edge{
		edge.To("memories", Memory.Type), // The edge is named "memories" and connects to the Memory type.
	}
}
