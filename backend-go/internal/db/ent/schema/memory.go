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
		field.Time("created_at").
			Default(time.Now),
		field.String("description"),
		field.String("event_type"),
		field.Strings("participants"),
		field.Float("importance").
			Default(0.5),
		field.String("actor").
			Default(""),
		field.String("subject").
			Default(""),
		field.JSON("delta", map[string]float64{}).
			Optional(),
		field.Float("intensity").
			Default(0),
		field.Float("count").
			Default(1),
		field.Bool("harmful").
			Default(false),
		field.Float("forgiven").
			Default(0),
		field.Bool("betrayal").
			Default(false),
		field.String("text").
			Default(""),
		field.Time("first_at").
			Default(time.Now),
		field.Time("last_at").
			Default(time.Now),
		field.JSON("covers", []int{}).
			Optional(),
	}
}

// Edges of the Memory.
func (Memory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", NPC.Type).
			Ref("memories").
			Unique().
			Required(),
	}
}
