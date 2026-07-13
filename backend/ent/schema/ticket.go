package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Ticket struct {
	ent.Schema
}

func (Ticket) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tickets"},
	}
}

func (Ticket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("subject").
			MaxLen(200),
		field.String("category").
			MaxLen(32),
		field.String("priority").
			MaxLen(20).
			Default(domain.TicketPriorityNormal),
		field.String("status").
			MaxLen(20).
			Default(domain.TicketStatusPending),
		field.Time("last_activity_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Ticket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("tickets").
			Field("user_id").
			Unique().
			Required(),
		edge.To("messages", TicketMessage.Type),
	}
}

func (Ticket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "last_activity_at"),
		index.Fields("status", "last_activity_at"),
		index.Fields("category", "last_activity_at"),
		index.Fields("priority", "last_activity_at"),
	}
}
