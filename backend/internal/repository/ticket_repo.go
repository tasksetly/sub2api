package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ticketRepository struct {
	db *sql.DB
}

func NewTicketRepository(db *sql.DB) service.TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) Create(ctx context.Context, item *service.Ticket, message *service.TicketMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tickets (user_id, subject, category, status, priority, last_message_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at`,
		item.UserID, item.Subject, item.Category, item.Status, item.Priority, item.LastMessageAt,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return err
	}
	attachments, err := json.Marshal(message.Attachments)
	if err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ticket_messages (ticket_id, author_id, author_role, content, attachments, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		item.ID, message.AuthorID, message.AuthorRole, message.Content, attachments, message.CreatedAt,
	).Scan(&message.ID)
	if err != nil {
		return err
	}
	message.TicketID = item.ID
	return tx.Commit()
}

func (r *ticketRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.TicketListFilters) ([]service.Ticket, *pagination.PaginationResult, error) {
	where, args := ticketWhere(filters)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets t JOIN users u ON u.id = t.user_id `+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	orderField := ticketOrderField(params.SortBy)
	order := "DESC"
	if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
		order = "ASC"
	}
	args = append(args, params.Limit(), params.Offset())
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := fmt.Sprintf(`
		SELECT t.id, t.user_id, u.email, u.username, t.subject, t.category, t.status, t.priority,
		       t.last_message_at, t.closed_at, t.created_at, t.updated_at
		FROM tickets t JOIN users u ON u.id = t.user_id
		%s ORDER BY %s %s, t.id %s LIMIT $%d OFFSET $%d`, where, orderField, order, order, limitArg, offsetArg)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.Ticket, 0)
	for rows.Next() {
		var item service.Ticket
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Username, &item.Subject, &item.Category, &item.Status, &item.Priority, &item.LastMessageAt, &item.ClosedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, err
		}
		item.Messages = []service.TicketMessage{}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *ticketRepository) GetByID(ctx context.Context, id int64) (*service.Ticket, error) {
	item := &service.Ticket{Messages: []service.TicketMessage{}}
	err := r.db.QueryRowContext(ctx, `
		SELECT t.id, t.user_id, u.email, u.username, t.subject, t.category, t.status, t.priority,
		       t.last_message_at, t.closed_at, t.created_at, t.updated_at
		FROM tickets t JOIN users u ON u.id = t.user_id WHERE t.id = $1`, id,
	).Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Username, &item.Subject, &item.Category, &item.Status, &item.Priority, &item.LastMessageAt, &item.ClosedAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrTicketNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.ticket_id, m.author_id, m.author_role, COALESCE(NULLIF(u.username, ''), u.email),
		       m.content, m.attachments, m.created_at
		FROM ticket_messages m JOIN users u ON u.id = m.author_id
		WHERE m.ticket_id = $1 ORDER BY m.created_at ASC, m.id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var message service.TicketMessage
		var attachments []byte
		if err := rows.Scan(&message.ID, &message.TicketID, &message.AuthorID, &message.AuthorRole, &message.AuthorName, &message.Content, &attachments, &message.CreatedAt); err != nil {
			return nil, err
		}
		if len(attachments) > 0 {
			if err := json.Unmarshal(attachments, &message.Attachments); err != nil {
				return nil, fmt.Errorf("decode ticket attachments: %w", err)
			}
		}
		if message.Attachments == nil {
			message.Attachments = []service.TicketAttachment{}
		}
		item.Messages = append(item.Messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *ticketRepository) AddMessage(ctx context.Context, message *service.TicketMessage, nextStatus string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE tickets SET status = $1, last_message_at = $2, closed_at = NULL, updated_at = NOW()
		WHERE id = $3 AND status <> 'closed'`, nextStatus, message.CreatedAt, message.TicketID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets WHERE id = $1)`, message.TicketID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return service.ErrTicketNotFound
		}
		return service.ErrTicketClosed
	}
	attachments, err := json.Marshal(message.Attachments)
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO ticket_messages (ticket_id, author_id, author_role, content, attachments, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		message.TicketID, message.AuthorID, message.AuthorRole, message.Content, attachments, message.CreatedAt,
	).Scan(&message.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ticketRepository) Update(ctx context.Context, id int64, status, priority string, closedAt *time.Time) (*service.Ticket, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE tickets SET status = $1, priority = $2, closed_at = $3, updated_at = NOW() WHERE id = $4`,
		status, priority, closedAt, id)
	if err != nil {
		return nil, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if updated == 0 {
		return nil, service.ErrTicketNotFound
	}
	return r.GetByID(ctx, id)
}

func ticketWhere(filters service.TicketListFilters) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	add := func(column string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filters.UserID > 0 {
		add("t.user_id", filters.UserID)
	}
	if filters.Status != "" {
		add("t.status", filters.Status)
	}
	if filters.Priority != "" {
		add("t.priority", filters.Priority)
	}
	if filters.Category != "" {
		add("t.category", filters.Category)
	}
	if filters.Search != "" {
		args = append(args, "%"+filters.Search+"%")
		clauses = append(clauses, fmt.Sprintf("(t.subject ILIKE $%d OR u.email ILIKE $%d OR u.username ILIKE $%d)", len(args), len(args), len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func ticketOrderField(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "id":
		return "t.id"
	case "created_at":
		return "t.created_at"
	case "updated_at":
		return "t.updated_at"
	case "status":
		return "t.status"
	case "priority":
		return "t.priority"
	default:
		return "t.last_message_at"
	}
}
