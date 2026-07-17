package domain

import (
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	TicketStatusOpen        = "open"
	TicketStatusInProgress  = "in_progress"
	TicketStatusWaitingUser = "waiting_user"
	TicketStatusClosed      = "closed"
)

const (
	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"
)

const (
	TicketAuthorUser  = "user"
	TicketAuthorAdmin = "admin"
)

var ErrTicketNotFound = infraerrors.NotFound("TICKET_NOT_FOUND", "ticket not found")

type TicketAttachment struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url,omitempty"`
}

type TicketMessage struct {
	ID          int64
	TicketID    int64
	AuthorID    int64
	AuthorRole  string
	AuthorName  string
	Content     string
	Attachments []TicketAttachment
	CreatedAt   time.Time
}

type Ticket struct {
	ID            int64
	UserID        int64
	UserEmail     string
	Username      string
	Subject       string
	Category      string
	Status        string
	Priority      string
	LastMessageAt time.Time
	ClosedAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Messages      []TicketMessage
}
