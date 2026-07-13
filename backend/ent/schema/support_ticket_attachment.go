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

type SupportTicketAttachment struct {
	ent.Schema
}

func (SupportTicketAttachment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_attachments"}}
}

func (SupportTicketAttachment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("message_id"),
		field.Int64("uploader_id"),
		field.String("object_key").MaxLen(1024).NotEmpty().Unique(),
		field.String("file_name").MaxLen(255).NotEmpty(),
		field.String("content_type").MaxLen(100).NotEmpty(),
		field.Int64("size_bytes").Positive(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketAttachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).Ref("attachments").Field("ticket_id").Unique().Required(),
		edge.From("message", SupportTicketMessage.Type).Ref("attachments").Field("message_id").Unique().Required(),
		edge.From("uploader", User.Type).Ref("support_ticket_attachments").Field("uploader_id").Unique().Required(),
	}
}

func (SupportTicketAttachment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id"),
		index.Fields("message_id", "created_at"),
		index.Fields("uploader_id"),
	}
}
