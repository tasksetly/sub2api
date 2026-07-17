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

// TicketMessage is one message in a ticket conversation.
type TicketMessage struct {
	ent.Schema
}

func (TicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ticket_messages"}}
}

func (TicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("author_id"),
		field.String("author_role").MaxLen(16),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.JSON("attachments", []domain.TicketAttachment{}).
			Default([]domain.TicketAttachment{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").Immutable().Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", Ticket.Type).Ref("messages").Field("ticket_id").Unique().Required(),
		edge.From("author", User.Type).Ref("ticket_messages").Field("author_id").Unique().Required(),
	}
}

func (TicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id", "created_at"),
		index.Fields("author_id"),
	}
}
