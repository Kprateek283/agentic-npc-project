package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Item holds the schema definition for the Item entity.
// This table stores the "static" definitions of all items in the game.
type Item struct {
	ent.Schema
}

// Fields of the Item.
func (Item) Fields() []ent.Field {
	return []ent.Field{
		// We use the item_id from our JSON file as the primary key.
		field.String("item_id").
			Unique().
			NotEmpty().
			Comment("The unique item ID, e.g., 'apple', 'rotten_fish'"),

		field.String("name").
			NotEmpty().
			Comment("The display name of the item, e.g., 'Apple'"),

		field.String("rarity").
			NotEmpty().
			Comment("The rarity of the item, e.g., 'usable', 'common', 'legendary'"),

		field.Float("base_trust_value").
			Default(0.0).
			Comment("The base trust change this item gives as a gift"),

		field.Bool("quest_item").
			Default(false).
			Comment("Is this item a quest item?"),
	}
}

// Edges of the Item.
func (Item) Edges() []ent.Edge {
	return []ent.Edge{
		// An item can be in many different player's inventories.
		// This defines the "one-to-many" relationship with the join table.
		edge.To("inventory_items", InventoryItem.Type),
	}
}
