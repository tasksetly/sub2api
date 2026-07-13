package repository

import (
	"context"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketattachment"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type supportTicketRepository struct {
	client *dbent.Client
}

func NewSupportTicketRepository(client *dbent.Client) service.SupportTicketRepository {
	return &supportTicketRepository{client: client}
}

func (r *supportTicketRepository) Create(ctx context.Context, ticket *service.SupportTicket, initialMessage *service.SupportTicketMessage) error {
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		created, err := client.SupportTicket.Create().
			SetUserID(ticket.UserID).
			SetSubject(ticket.Subject).
			SetCategory(ticket.Category).
			SetPriority(ticket.Priority).
			SetStatus(ticket.Status).
			SetAdminUnread(ticket.AdminUnread).
			SetUserUnread(ticket.UserUnread).
			SetLastMessageAt(ticket.LastMessageAt).
			Save(txCtx)
		if err != nil {
			return err
		}

		message, err := client.SupportTicketMessage.Create().
			SetTicketID(created.ID).
			SetSenderID(initialMessage.SenderID).
			SetSenderRole(initialMessage.SenderRole).
			SetContent(initialMessage.Content).
			SetCreatedAt(initialMessage.CreatedAt).
			Save(txCtx)
		if err != nil {
			return err
		}

		ticket.ID = created.ID
		ticket.CreatedAt = created.CreatedAt
		ticket.UpdatedAt = created.UpdatedAt
		initialMessage.ID = message.ID
		initialMessage.TicketID = created.ID
		if err := createSupportTicketAttachments(txCtx, client, created.ID, message.ID, initialMessage.Attachments); err != nil {
			return err
		}
		ticket.Messages = []service.SupportTicketMessage{*initialMessage}
		return nil
	})
}

func (r *supportTicketRepository) GetByID(ctx context.Context, id int64) (*service.SupportTicket, error) {
	model, err := r.client.SupportTicket.Query().
		Where(supportticket.IDEQ(id)).
		WithUser().
		WithMessages(func(q *dbent.SupportTicketMessageQuery) {
			q.WithAttachments(func(aq *dbent.SupportTicketAttachmentQuery) {
				aq.Order(dbent.Asc(supportticketattachment.FieldCreatedAt), dbent.Asc(supportticketattachment.FieldID))
			})
			q.Order(dbent.Asc(supportticketmessage.FieldCreatedAt), dbent.Asc(supportticketmessage.FieldID))
		}).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	return supportTicketEntityToService(model), nil
}

func (r *supportTicketRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	q := r.client.SupportTicket.Query().WithUser()
	if filters.UserID != nil {
		q = q.Where(supportticket.UserIDEQ(*filters.UserID))
	}
	if filters.Status != "" {
		q = q.Where(supportticket.StatusEQ(filters.Status))
	}
	if filters.Category != "" {
		q = q.Where(supportticket.CategoryEQ(filters.Category))
	}
	if filters.Priority != "" {
		q = q.Where(supportticket.PriorityEQ(filters.Priority))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		q = q.Where(supportticket.Or(
			supportticket.SubjectContainsFold(search),
			supportticket.HasUserWith(user.Or(user.EmailContainsFold(search), user.UsernameContainsFold(search))),
		))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, order := range supportTicketListOrders(params) {
		q = q.Order(order)
	}
	models, err := q.Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	items := make([]service.SupportTicket, 0, len(models))
	for _, model := range models {
		items = append(items, *supportTicketEntityToService(model))
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func (r *supportTicketRepository) AddMessage(ctx context.Context, ticket *service.SupportTicket, message *service.SupportTicketMessage) error {
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if err := updateSupportTicketEntity(txCtx, client, ticket); err != nil {
			return err
		}
		created, err := client.SupportTicketMessage.Create().
			SetTicketID(ticket.ID).
			SetSenderID(message.SenderID).
			SetSenderRole(message.SenderRole).
			SetContent(message.Content).
			SetCreatedAt(message.CreatedAt).
			Save(txCtx)
		if err != nil {
			return err
		}
		message.ID = created.ID
		message.TicketID = created.TicketID
		if err := createSupportTicketAttachments(txCtx, client, ticket.ID, created.ID, message.Attachments); err != nil {
			return err
		}
		ticket.Messages = append(ticket.Messages, *message)
		return nil
	})
}

func createSupportTicketAttachments(ctx context.Context, client *dbent.Client, ticketID, messageID int64, attachments []service.SupportTicketAttachment) error {
	for i := range attachments {
		attachment := &attachments[i]
		created, err := client.SupportTicketAttachment.Create().
			SetTicketID(ticketID).
			SetMessageID(messageID).
			SetUploaderID(attachment.UploaderID).
			SetObjectKey(attachment.ObjectKey).
			SetFileName(attachment.FileName).
			SetContentType(attachment.ContentType).
			SetSizeBytes(attachment.SizeBytes).
			SetCreatedAt(attachment.CreatedAt).
			Save(ctx)
		if err != nil {
			return err
		}
		attachment.ID = created.ID
		attachment.TicketID = ticketID
		attachment.MessageID = messageID
	}
	return nil
}

func (r *supportTicketRepository) Update(ctx context.Context, ticket *service.SupportTicket) error {
	return updateSupportTicketEntity(ctx, clientFromContext(ctx, r.client), ticket)
}

func (r *supportTicketRepository) MarkRead(ctx context.Context, ticketID int64, readerRole string) error {
	update := clientFromContext(ctx, r.client).SupportTicket.UpdateOneID(ticketID)
	if readerRole == service.SupportTicketSenderAdmin {
		update.SetAdminUnread(false)
	} else {
		update.SetUserUnread(false)
	}
	_, err := update.Save(ctx)
	return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
}

func updateSupportTicketEntity(ctx context.Context, client *dbent.Client, ticket *service.SupportTicket) error {
	update := client.SupportTicket.UpdateOneID(ticket.ID).
		SetPriority(ticket.Priority).
		SetStatus(ticket.Status).
		SetAdminUnread(ticket.AdminUnread).
		SetUserUnread(ticket.UserUnread).
		SetLastMessageAt(ticket.LastMessageAt)
	if ticket.ClosedAt != nil {
		update.SetClosedAt(*ticket.ClosedAt)
	} else {
		update.ClearClosedAt()
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	ticket.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *supportTicketRepository) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin support ticket transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit support ticket transaction: %w", err)
	}
	return nil
}

func supportTicketListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	field := supportticket.FieldLastMessageAt
	switch strings.ToLower(strings.TrimSpace(params.SortBy)) {
	case "created_at":
		field = supportticket.FieldCreatedAt
	case "updated_at":
		field = supportticket.FieldUpdatedAt
	case "priority":
		field = supportticket.FieldPriority
	case "status":
		field = supportticket.FieldStatus
	}
	if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(supportticket.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(supportticket.FieldID)}
}

func supportTicketEntityToService(model *dbent.SupportTicket) *service.SupportTicket {
	if model == nil {
		return nil
	}
	item := &service.SupportTicket{
		ID: model.ID, UserID: model.UserID, Subject: model.Subject, Category: model.Category,
		Priority: model.Priority, Status: model.Status, AdminUnread: model.AdminUnread,
		UserUnread: model.UserUnread, LastMessageAt: model.LastMessageAt, ClosedAt: model.ClosedAt,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
	if model.Edges.User != nil {
		item.UserEmail = model.Edges.User.Email
		item.Username = model.Edges.User.Username
	}
	if len(model.Edges.Messages) > 0 {
		item.Messages = make([]service.SupportTicketMessage, 0, len(model.Edges.Messages))
		for _, message := range model.Edges.Messages {
			converted := service.SupportTicketMessage{
				ID: message.ID, TicketID: message.TicketID, SenderID: message.SenderID,
				SenderRole: message.SenderRole, Content: message.Content, CreatedAt: message.CreatedAt,
			}
			if len(message.Edges.Attachments) > 0 {
				converted.Attachments = make([]service.SupportTicketAttachment, 0, len(message.Edges.Attachments))
				for _, attachment := range message.Edges.Attachments {
					converted.Attachments = append(converted.Attachments, service.SupportTicketAttachment{
						ID: attachment.ID, TicketID: attachment.TicketID, MessageID: attachment.MessageID,
						UploaderID: attachment.UploaderID, ObjectKey: attachment.ObjectKey, FileName: attachment.FileName,
						ContentType: attachment.ContentType, SizeBytes: attachment.SizeBytes, CreatedAt: attachment.CreatedAt,
					})
				}
			}
			item.Messages = append(item.Messages, converted)
		}
	}
	return item
}
