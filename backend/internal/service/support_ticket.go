package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SupportTicketStatusOpen        = "open"
	SupportTicketStatusInProgress  = "in_progress"
	SupportTicketStatusWaitingUser = "waiting_user"
	SupportTicketStatusResolved    = "resolved"
	SupportTicketStatusClosed      = "closed"

	SupportTicketPriorityLow    = "low"
	SupportTicketPriorityNormal = "normal"
	SupportTicketPriorityHigh   = "high"
	SupportTicketPriorityUrgent = "urgent"

	SupportTicketCategoryTechnical = "technical"
	SupportTicketCategoryBilling   = "billing"
	SupportTicketCategoryAccount   = "account"
	SupportTicketCategoryOther     = "other"

	SupportTicketSenderUser  = "user"
	SupportTicketSenderAdmin = "admin"
)

var (
	ErrSupportTicketNotFound          = infraerrors.NotFound("SUPPORT_TICKET_NOT_FOUND", "support ticket not found")
	ErrSupportTicketClosed            = infraerrors.Conflict("SUPPORT_TICKET_CLOSED", "closed support tickets cannot receive replies")
	ErrSupportTicketInvalidTransition = infraerrors.Conflict(
		"SUPPORT_TICKET_INVALID_TRANSITION",
		"support ticket status transition is not allowed",
	)
	ErrSupportTicketInvalidSubject  = infraerrors.BadRequest("SUPPORT_TICKET_SUBJECT_INVALID", "support ticket subject is invalid")
	ErrSupportTicketInvalidContent  = infraerrors.BadRequest("SUPPORT_TICKET_CONTENT_INVALID", "support ticket content is invalid")
	ErrSupportTicketInvalidStatus   = infraerrors.BadRequest("SUPPORT_TICKET_STATUS_INVALID", "support ticket status is invalid")
	ErrSupportTicketInvalidPriority = infraerrors.BadRequest("SUPPORT_TICKET_PRIORITY_INVALID", "support ticket priority is invalid")
	ErrSupportTicketInvalidCategory = infraerrors.BadRequest("SUPPORT_TICKET_CATEGORY_INVALID", "support ticket category is invalid")
)

type SupportTicket struct {
	ID            int64                  `json:"id"`
	UserID        int64                  `json:"user_id"`
	UserEmail     string                 `json:"user_email,omitempty"`
	Username      string                 `json:"username,omitempty"`
	Subject       string                 `json:"subject"`
	Category      string                 `json:"category"`
	Priority      string                 `json:"priority"`
	Status        string                 `json:"status"`
	AdminUnread   bool                   `json:"admin_unread"`
	UserUnread    bool                   `json:"user_unread"`
	LastMessageAt time.Time              `json:"last_message_at"`
	ClosedAt      *time.Time             `json:"closed_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Messages      []SupportTicketMessage `json:"messages,omitempty"`
}

type SupportTicketMessage struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticket_id"`
	SenderID   int64     `json:"sender_id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type SupportTicketListFilters struct {
	UserID   *int64
	Status   string
	Category string
	Priority string
	Search   string
}

type CreateSupportTicketInput struct {
	Subject  string
	Category string
	Priority string
	Content  string
}

type UpdateSupportTicketInput struct {
	Status   *string
	Priority *string
}

type SupportTicketRepository interface {
	Create(ctx context.Context, ticket *SupportTicket, initialMessage *SupportTicketMessage) error
	GetByID(ctx context.Context, id int64) (*SupportTicket, error)
	List(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	AddMessage(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) error
	Update(ctx context.Context, ticket *SupportTicket) error
	MarkRead(ctx context.Context, ticketID int64, readerRole string) error
}

type SupportTicketService struct {
	repo SupportTicketRepository
	now  func() time.Time
}

func NewSupportTicketService(repo SupportTicketRepository) *SupportTicketService {
	return &SupportTicketService{repo: repo, now: time.Now}
}

func (s *SupportTicketService) CreateForUser(ctx context.Context, userID int64, input CreateSupportTicketInput) (*SupportTicket, error) {
	subject := strings.TrimSpace(input.Subject)
	content := strings.TrimSpace(input.Content)
	category := strings.TrimSpace(input.Category)
	priority := strings.TrimSpace(input.Priority)
	if subject == "" || len([]rune(subject)) > 200 {
		return nil, ErrSupportTicketInvalidSubject
	}
	if !validSupportTicketContent(content) {
		return nil, ErrSupportTicketInvalidContent
	}
	if category == "" {
		category = SupportTicketCategoryOther
	}
	if !isValidSupportTicketCategory(category) {
		return nil, ErrSupportTicketInvalidCategory
	}
	if priority == "" {
		priority = SupportTicketPriorityNormal
	}
	if !isValidSupportTicketPriority(priority) {
		return nil, ErrSupportTicketInvalidPriority
	}

	now := s.now().UTC()
	ticket := &SupportTicket{
		UserID:        userID,
		Subject:       subject,
		Category:      category,
		Priority:      priority,
		Status:        SupportTicketStatusOpen,
		AdminUnread:   true,
		LastMessageAt: now,
	}
	message := &SupportTicketMessage{SenderID: userID, SenderRole: SupportTicketSenderUser, Content: content, CreatedAt: now}
	if err := s.repo.Create(ctx, ticket, message); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *SupportTicketService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	filters.UserID = &userID
	return s.repo.List(ctx, params, filters)
}

func (s *SupportTicketService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, params, filters)
}

func (s *SupportTicketService) GetForUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	if ticket.UserUnread {
		if err := s.repo.MarkRead(ctx, ticketID, SupportTicketSenderUser); err != nil {
			return nil, err
		}
		ticket.UserUnread = false
	}
	return ticket, nil
}

func (s *SupportTicketService) GetForAdmin(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.AdminUnread {
		if err := s.repo.MarkRead(ctx, ticketID, SupportTicketSenderAdmin); err != nil {
			return nil, err
		}
		ticket.AdminUnread = false
	}
	return ticket, nil
}

func (s *SupportTicketService) ReplyAsUser(ctx context.Context, userID, ticketID int64, content string) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	if ticket.Status == SupportTicketStatusClosed {
		return nil, ErrSupportTicketClosed
	}
	content = strings.TrimSpace(content)
	if !validSupportTicketContent(content) {
		return nil, ErrSupportTicketInvalidContent
	}
	if ticket.Status == SupportTicketStatusWaitingUser || ticket.Status == SupportTicketStatusResolved {
		ticket.Status = SupportTicketStatusOpen
		ticket.ClosedAt = nil
	}
	now := s.now().UTC()
	ticket.LastMessageAt = now
	ticket.AdminUnread = true
	ticket.UserUnread = false
	message := &SupportTicketMessage{TicketID: ticketID, SenderID: userID, SenderRole: SupportTicketSenderUser, Content: content, CreatedAt: now}
	if err := s.repo.AddMessage(ctx, ticket, message); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *SupportTicketService) ReplyAsAdmin(ctx context.Context, adminID, ticketID int64, content string) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status == SupportTicketStatusClosed {
		return nil, ErrSupportTicketClosed
	}
	content = strings.TrimSpace(content)
	if !validSupportTicketContent(content) {
		return nil, ErrSupportTicketInvalidContent
	}
	now := s.now().UTC()
	ticket.Status = SupportTicketStatusWaitingUser
	ticket.ClosedAt = nil
	ticket.LastMessageAt = now
	ticket.UserUnread = true
	ticket.AdminUnread = false
	message := &SupportTicketMessage{TicketID: ticketID, SenderID: adminID, SenderRole: SupportTicketSenderAdmin, Content: content, CreatedAt: now}
	if err := s.repo.AddMessage(ctx, ticket, message); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *SupportTicketService) CloseAsUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	if ticket.Status == SupportTicketStatusClosed {
		return ticket, nil
	}
	now := s.now().UTC()
	ticket.Status = SupportTicketStatusClosed
	ticket.ClosedAt = &now
	ticket.AdminUnread = true
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *SupportTicketService) ReopenAsUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	if ticket.Status != SupportTicketStatusClosed && ticket.Status != SupportTicketStatusResolved {
		return nil, ErrSupportTicketInvalidTransition
	}
	ticket.Status = SupportTicketStatusOpen
	ticket.ClosedAt = nil
	ticket.AdminUnread = true
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *SupportTicketService) UpdateAsAdmin(ctx context.Context, _ int64, ticketID int64, input UpdateSupportTicketInput) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if input.Priority != nil {
		priority := strings.TrimSpace(*input.Priority)
		if !isValidSupportTicketPriority(priority) {
			return nil, ErrSupportTicketInvalidPriority
		}
		ticket.Priority = priority
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if !isValidSupportTicketStatus(status) {
			return nil, ErrSupportTicketInvalidStatus
		}
		if !isAllowedAdminTicketTransition(ticket.Status, status) {
			return nil, ErrSupportTicketInvalidTransition
		}
		ticket.Status = status
		if status == SupportTicketStatusClosed {
			now := s.now().UTC()
			ticket.ClosedAt = &now
		} else {
			ticket.ClosedAt = nil
		}
		ticket.UserUnread = true
	}
	if input.Status == nil && input.Priority == nil {
		return ticket, nil
	}
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func validSupportTicketContent(content string) bool {
	length := len([]rune(content))
	return length > 0 && length <= 10000
}

func isValidSupportTicketStatus(status string) bool {
	switch status {
	case SupportTicketStatusOpen, SupportTicketStatusInProgress, SupportTicketStatusWaitingUser, SupportTicketStatusResolved, SupportTicketStatusClosed:
		return true
	default:
		return false
	}
}

func isValidSupportTicketPriority(priority string) bool {
	switch priority {
	case SupportTicketPriorityLow, SupportTicketPriorityNormal, SupportTicketPriorityHigh, SupportTicketPriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidSupportTicketCategory(category string) bool {
	switch category {
	case SupportTicketCategoryTechnical, SupportTicketCategoryBilling, SupportTicketCategoryAccount, SupportTicketCategoryOther:
		return true
	default:
		return false
	}
}

func isAllowedAdminTicketTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case SupportTicketStatusOpen:
		return to == SupportTicketStatusInProgress || to == SupportTicketStatusWaitingUser || to == SupportTicketStatusResolved || to == SupportTicketStatusClosed
	case SupportTicketStatusInProgress:
		return to == SupportTicketStatusWaitingUser || to == SupportTicketStatusResolved || to == SupportTicketStatusClosed
	case SupportTicketStatusWaitingUser:
		return to == SupportTicketStatusInProgress || to == SupportTicketStatusResolved || to == SupportTicketStatusClosed
	case SupportTicketStatusResolved:
		return to == SupportTicketStatusInProgress || to == SupportTicketStatusClosed
	case SupportTicketStatusClosed:
		return to == SupportTicketStatusInProgress
	default:
		return false
	}
}
