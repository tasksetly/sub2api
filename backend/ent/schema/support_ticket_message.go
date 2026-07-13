package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupportTicketMessage struct {
	ent.Schema
}

func (SupportTicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_messages"}}
}

func (SupportTicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("sender_id"),
		field.String("sender_role").MaxLen(16),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).NotEmpty(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).Ref("messages").Field("ticket_id").Unique().Required(),
	}
}

func (SupportTicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id", "created_at"),
		index.Fields("sender_id"),
	}
}
