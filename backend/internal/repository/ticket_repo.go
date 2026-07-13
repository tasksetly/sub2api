package repository

import (
	"context"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/ticket"
	"github.com/Wei-Shaw/sub2api/ent/ticketmessage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ticketRepository struct{ client *dbent.Client }

func NewTicketRepository(client *dbent.Client) service.TicketRepository {
	return &ticketRepository{client}
}
func toTicket(v *dbent.Ticket) service.Ticket {
	return service.Ticket{ID: v.ID, UserID: v.UserID, Subject: v.Subject, Category: v.Category, Priority: v.Priority, Status: v.Status, LastActivityAt: v.LastActivityAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toMessage(v *dbent.TicketMessage) service.TicketMessage {
	return service.TicketMessage{ID: v.ID, TicketID: v.TicketID, SenderUserID: v.SenderUserID, SenderRole: v.SenderRole, Content: v.Content, CreatedAt: v.CreatedAt}
}
func (r *ticketRepository) Create(ctx context.Context, t *service.Ticket, m *service.TicketMessage) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	created, err := tx.Ticket.Create().SetUserID(t.UserID).SetSubject(t.Subject).SetCategory(t.Category).SetPriority(t.Priority).SetStatus(t.Status).SetLastActivityAt(t.LastActivityAt).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	msg, err := tx.TicketMessage.Create().SetTicketID(created.ID).SetSenderUserID(m.SenderUserID).SetSenderRole(m.SenderRole).SetContent(m.Content).SetCreatedAt(m.CreatedAt).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	*t = toTicket(created)
	*m = toMessage(msg)
	return nil
}
func (r *ticketRepository) GetDetail(ctx context.Context, id int64) (*service.TicketDetail, error) {
	v, err := r.client.Ticket.Query().Where(ticket.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, service.ErrTicketNotFound
	}
	msgs, err := r.client.TicketMessage.Query().Where(ticketmessage.TicketIDEQ(id)).Order(dbent.Asc(ticketmessage.FieldCreatedAt), dbent.Asc(ticketmessage.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.TicketMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessage(m))
	}
	return &service.TicketDetail{Ticket: toTicket(v), Messages: out}, nil
}
func (r *ticketRepository) ListByUser(ctx context.Context, userID int64, p pagination.PaginationParams) ([]service.Ticket, *pagination.PaginationResult, error) {
	q := r.client.Ticket.Query().Where(ticket.UserIDEQ(userID))
	return r.list(ctx, q, p)
}
func (r *ticketRepository) List(ctx context.Context, p pagination.PaginationParams, f service.TicketListFilters) ([]service.Ticket, *pagination.PaginationResult, error) {
	q := r.client.Ticket.Query()
	if f.Status != "" {
		q = q.Where(ticket.StatusEQ(f.Status))
	}
	if f.Category != "" {
		q = q.Where(ticket.CategoryEQ(f.Category))
	}
	if f.Priority != "" {
		q = q.Where(ticket.PriorityEQ(f.Priority))
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		q = q.Where(ticket.SubjectContainsFold(s))
	}
	return r.list(ctx, q, p)
}
func (r *ticketRepository) list(ctx context.Context, q *dbent.TicketQuery, p pagination.PaginationParams) ([]service.Ticket, *pagination.PaginationResult, error) {
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.Order(dbent.Desc(ticket.FieldLastActivityAt), dbent.Desc(ticket.FieldID)).Offset(p.Offset()).Limit(p.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.Ticket, 0, len(rows))
	for _, v := range rows {
		out = append(out, toTicket(v))
	}
	return out, &pagination.PaginationResult{Total: int64(total), Page: p.Page, PageSize: p.PageSize}, nil
}
func (r *ticketRepository) AddMessage(ctx context.Context, m *service.TicketMessage, status string, at time.Time) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	created, err := tx.TicketMessage.Create().SetTicketID(m.TicketID).SetSenderUserID(m.SenderUserID).SetSenderRole(m.SenderRole).SetContent(m.Content).SetCreatedAt(m.CreatedAt).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.Ticket.UpdateOneID(m.TicketID).SetStatus(status).SetLastActivityAt(at).Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	*m = toMessage(created)
	return nil
}
func (r *ticketRepository) Update(ctx context.Context, t *service.Ticket) error {
	v, err := r.client.Ticket.UpdateOneID(t.ID).SetStatus(t.Status).SetPriority(t.Priority).SetUpdatedAt(t.UpdatedAt).SetLastActivityAt(t.LastActivityAt).Save(ctx)
	if err != nil {
		return service.ErrTicketNotFound
	}
	*t = toTicket(v)
	return nil
}
