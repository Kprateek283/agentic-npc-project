package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PlayerNPCRelationship holds the schema definition for the join-table
// that stores the dynamic state between a Player and an NPC.
type PlayerNPCRelationship struct {
	ent.Schema
}

// Fields of the PlayerNPCRelationship.
func (PlayerNPCRelationship) Fields() []ent.Field {
	return []ent.Field{
		field.Float("trust_level").
			Default(0.0).
			Comment("The trust level between 0.0 and 1.0"),

		field.Int("gift_count").
			Default(0).
			Comment("The number of gifts this player has given this NPC, for diminishing returns."),
	}
}

// Edges of the PlayerNPCRelationship.
func (PlayerNPCRelationship) Edges() []ent.Edge {
	return []ent.Edge{
		// A relationship must have one Player.
		edge.From("player", Player.Type).
			Ref("npc_relationships").
			Unique().
			Required(),

		// A relationship must have one NPC.
		edge.From("npc", NPC.Type).
			Ref("player_relationships").
			Unique().
			Required(),
	}
}

// Indexes ensure that the (player, npc) pair is unique.
func (PlayerNPCRelationship) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("player", "npc").
			Unique(),
	}
}
