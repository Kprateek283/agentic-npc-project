package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InventoryItem holds the schema definition for the join-table
// that represents a stack of items in a player's inventory.
type InventoryItem struct {
	ent.Schema
}

// Fields of the InventoryItem.
func (InventoryItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int("quantity").
			Default(1).
			Comment("The number of items in this stack"),
	}
}

// Edges of the InventoryItem.
func (InventoryItem) Edges() []ent.Edge {
	return []ent.Edge{
		// An inventory item must belong to one Player.
		edge.From("player", Player.Type).
			Ref("inventory").
			Unique().
			Required(),

		// An inventory item must be of one Item type.
		edge.From("item", Item.Type).
			Ref("inventory_items").
			Unique().
			Required(),
	}
}

// Indexes ensure that a player can only have one "stack" of each item.
func (InventoryItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("player", "item").
			Unique(),
	}
}
