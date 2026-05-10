package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Quest holds the schema definition for the Quest entity.
// This will store the static definition of a quest.
type Quest struct {
	ent.Schema
}

// Fields of the Quest.
func (Quest) Fields() []ent.Field {
	return []ent.Field{
		// We will use a string ID for this, as we'll define it in our gamedata files.
		// e.g., "main_quest_1", "herbalist_task_1"
		field.String("id").
			Unique().
			NotEmpty().
			Comment("A unique, human-readable string ID for the quest."),

		// The user-facing name of the quest.
		field.String("name").
			NotEmpty().
			Default("Untitled Quest"),

		// A path to the gamedata JSON file that contains the quest's steps
		field.String("static_data_path").
			NotEmpty().
			Comment("The file path to the quest's static JSON data (steps, descriptions)."),
	}
}

// Edges of the Quest.
func (Quest) Edges() []ent.Edge {
	return []ent.Edge{
		// A Quest can have many PlayerQuestStates (one for each player undertaking it).
		// This creates the other side of the "one-to-many" relationship.
		edge.To("player_quest_states", PlayerQuestState.Type),
	}
}
