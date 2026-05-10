package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PlayerQuestState holds the schema definition for the join-table
// that tracks a player's progress on a specific quest.
type PlayerQuestState struct {
	ent.Schema
}

// Fields of the PlayerQuestState.
func (PlayerQuestState) Fields() []ent.Field {
	return []ent.Field{
		// --- THIS IS THE FIX ---
		// Renamed from "quest_id" to "quest_identifier" to avoid conflict with the "quest" edge.
		field.String("quest_identifier").
			NotEmpty().
			Comment("The static ID of the quest (from gamedata)"),
		// ---------------------

		// The player's current step number in this quest.
		field.Int("current_step").
			Default(1),

		// Has the quest been fully completed?
		field.Bool("is_completed").
			Default(false),

		// The completion rate of the last step (for dynamic difficulty).
		field.Float32("completion_rate").
			Default(0.0),
	}
}

// Edges of the PlayerQuestState.
func (PlayerQuestState) Edges() []ent.Edge {
	return []ent.Edge{
		// A quest state must belong to one player.
		edge.From("player", Player.Type).
			Ref("quest_states").
			Unique().
			Required(),

		// A quest state must reference one quest.
		// We use .Unique() to make this a one-to-one link.
		edge.From("quest", Quest.Type).
			Ref("player_quest_states").
			Unique().
			Required(),
	}
}

// Indexes ensure a player can only have one state per quest.
func (PlayerQuestState) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("player", "quest").
			Unique(),
	}
}
