package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrTicketNotFound     = infraerrors.NotFound("TICKET_NOT_FOUND", "ticket not found")
	ErrTicketClosed       = infraerrors.BadRequest("TICKET_CLOSED", "closed tickets cannot receive messages")
	ErrTicketInvalidInput = infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid ticket input")
)

type Ticket = domain.Ticket
type TicketMessage = domain.TicketMessage
type TicketDetail struct {
	Ticket
	Messages []TicketMessage `json:"messages"`
}
type TicketListFilters struct{ Status, Category, Priority, Search string }
type CreateTicketInput struct{ Subject, Category, Content string }
type UpdateTicketInput struct{ Status, Priority *string }

type TicketRepository interface {
	Create(ctx context.Context, ticket *Ticket, message *TicketMessage) error
	GetDetail(ctx context.Context, id int64) (*TicketDetail, error)
	ListByUser(ctx context.Context, userID int64, p pagination.PaginationParams) ([]Ticket, *pagination.PaginationResult, error)
	List(ctx context.Context, p pagination.PaginationParams, f TicketListFilters) ([]Ticket, *pagination.PaginationResult, error)
	AddMessage(ctx context.Context, message *TicketMessage, status string, activityAt time.Time) error
	Update(ctx context.Context, ticket *Ticket) error
}
