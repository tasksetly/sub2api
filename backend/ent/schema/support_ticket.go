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

type SupportTicket struct {
	ent.Schema
}

func (SupportTicket) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_tickets"}}
}

func (SupportTicket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("subject").MaxLen(200).NotEmpty(),
		field.String("category").MaxLen(32).Default("other"),
		field.String("priority").MaxLen(16).Default("normal"),
		field.String("status").MaxLen(24).Default("open"),
		field.Bool("admin_unread").Default(true),
		field.Bool("user_unread").Default(false),
		field.Time("last_message_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("closed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("support_tickets").Field("user_id").Unique().Required(),
		edge.To("messages", SupportTicketMessage.Type),
	}
}

func (SupportTicket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "last_message_at"),
		index.Fields("status", "last_message_at"),
		index.Fields("category"),
		index.Fields("priority"),
		index.Fields("admin_unread"),
		index.Fields("user_unread"),
	}
}
