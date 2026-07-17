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

// Ticket holds a support request opened by a user.
type Ticket struct {
	ent.Schema
}

func (Ticket) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "tickets"}}
}

func (Ticket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("subject").MaxLen(200).NotEmpty(),
		field.String("category").MaxLen(32).Default("other"),
		field.String("status").MaxLen(24).Default(domain.TicketStatusOpen),
		field.String("priority").MaxLen(16).Default(domain.TicketPriorityNormal),
		field.Time("last_message_at").Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("closed_at").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Ticket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("tickets").Field("user_id").Unique().Required(),
		edge.To("messages", TicketMessage.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Ticket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "last_message_at"),
		index.Fields("status", "last_message_at"),
		index.Fields("priority", "last_message_at"),
	}
}
