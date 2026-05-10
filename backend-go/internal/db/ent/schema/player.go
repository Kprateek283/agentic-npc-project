package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Player holds the schema definition for the Player entity.
// This will store core information about the player.
type Player struct {
	ent.Schema
}

// Fields of the Player.
func (Player) Fields() []ent.Field {
	return []ent.Field{
		// We use a UUID for the primary key.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Unique(),

		// This will be the player's unique identifier, e.g., "player_prateek"
		field.String("player_id").
			Unique().
			NotEmpty().
			Comment("A unique, human-readable ID for the player (e.g., username)"),

		// The player's current display name in the game.
		field.String("player_name").
			Default("Adventurer"),

		// --- NEW FIELD ---
		// A simple password field for our demo login system.
		field.String("password").
			NotEmpty().
			Sensitive(), // .Sensitive() tells ent to omit this from logs
	}
}

// Edges of the Player.
func (Player) Edges() []ent.Edge {
	return []ent.Edge{
		// A Player can have many QuestStates (one for each quest they are on).
		edge.To("quest_states", PlayerQuestState.Type),

		// --- NEW EDGES ---
		// A Player can have many InventoryItems (their backpack).
		edge.To("inventory", InventoryItem.Type),

		// A Player can have many relationships (one for each NPC).
		edge.To("npc_relationships", PlayerNPCRelationship.Type),
	}
}
