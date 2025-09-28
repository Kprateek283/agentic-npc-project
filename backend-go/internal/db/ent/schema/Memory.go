package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Memory holds the schema definition for the Memory entity.
type Memory struct {
	ent.Schema
}

// Fields of the Memory.
func (Memory) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique(),

		// The time the memory was created.
		field.Time("created_at").
			Default(time.Now),

		// A structured description of the event.
		field.String("description"), // We can keep this for human-readable logs

		// The specific type of event, e.g., "PLAYER_INTERACT", "PLAYER_ATTACKED".
		field.String("event_type"),

		// A list of the IDs of the entities involved in this memory.
		// e.g., ["player_prateek", "npc_boris"]
		field.Strings("participants"),

		// A score from 0.0 to 1.0 indicating the memory's importance.
		field.Float("importance").
			Default(0.5),
	}
}

// Edges of the Memory.
func (Memory) Edges() []ent.Edge {
	return []ent.Edge{
		// This defines a "many-to-one" relationship.
		// Many memories can belong to one NPC.
		edge.From("owner", NPC.Type). // A memory has one "owner" of type NPC.
						Ref("memories"). // The reverse edge on the NPC schema is named "memories".
						Unique().        // A memory can only have one owner.
						Required(),      // Every memory MUST have an owner.
	}
}
