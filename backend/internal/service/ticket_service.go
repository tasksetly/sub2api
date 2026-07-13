package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type TicketService struct{ repo TicketRepository }

func NewTicketService(repo TicketRepository) *TicketService { return &TicketService{repo: repo} }

func ticketText(v string, max int) (string, bool) {
	v = strings.TrimSpace(v)
	return v, v != "" && len(v) <= max
}
func (s *TicketService) Create(ctx context.Context, userID int64, in CreateTicketInput) (*TicketDetail, error) {
	subject, ok := ticketText(in.Subject, 200)
	if !ok || !domain.IsTicketCategory(in.Category) {
		return nil, ErrTicketInvalidInput
	}
	content, ok := ticketText(in.Content, 10000)
	if !ok || userID <= 0 {
		return nil, ErrTicketInvalidInput
	}
	now := time.Now()
	ticket := &Ticket{UserID: userID, Subject: subject, Category: in.Category, Priority: domain.TicketPriorityNormal, Status: domain.TicketStatusPending, LastActivityAt: now}
	message := &TicketMessage{SenderUserID: userID, SenderRole: "user", Content: content, CreatedAt: now}
	if err := s.repo.Create(ctx, ticket, message); err != nil {
		return nil, err
	}
	return &TicketDetail{Ticket: *ticket, Messages: []TicketMessage{*message}}, nil
}
func (s *TicketService) GetForUser(ctx context.Context, userID, id int64) (*TicketDetail, error) {
	d, err := s.repo.GetDetail(ctx, id)
	if err != nil || d.UserID != userID {
		return nil, ErrTicketNotFound
	}
	return d, nil
}
func (s *TicketService) GetForAdmin(ctx context.Context, id int64) (*TicketDetail, error) {
	return s.repo.GetDetail(ctx, id)
}
func (s *TicketService) ListForUser(ctx context.Context, id int64, p pagination.PaginationParams) ([]Ticket, *pagination.PaginationResult, error) {
	return s.repo.ListByUser(ctx, id, p)
}
func (s *TicketService) ListForAdmin(ctx context.Context, p pagination.PaginationParams, f TicketListFilters) ([]Ticket, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, p, f)
}
func (s *TicketService) AddUserMessage(ctx context.Context, userID, id int64, content string) (*TicketDetail, error) {
	d, err := s.GetForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.addMessage(ctx, d, userID, "user", content)
}
func (s *TicketService) AddAdminMessage(ctx context.Context, adminID, id int64, content string) (*TicketDetail, error) {
	d, err := s.repo.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.addMessage(ctx, d, adminID, "admin", content)
}
func (s *TicketService) addMessage(ctx context.Context, d *TicketDetail, senderID int64, role, content string) (*TicketDetail, error) {
	content, ok := ticketText(content, 10000)
	if !ok {
		return nil, ErrTicketInvalidInput
	}
	if d.Status == domain.TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	now := time.Now()
	status := d.Status
	if role == "user" && status == domain.TicketStatusResolved {
		status = domain.TicketStatusInProgress
	}
	m := &TicketMessage{TicketID: d.ID, SenderUserID: senderID, SenderRole: role, Content: content, CreatedAt: now}
	if err := s.repo.AddMessage(ctx, m, status, now); err != nil {
		return nil, err
	}
	d.Status = status
	d.LastActivityAt = now
	d.Messages = append(d.Messages, *m)
	return d, nil
}
func (s *TicketService) UpdateByAdmin(ctx context.Context, id int64, in UpdateTicketInput) (*Ticket, error) {
	t, err := s.repo.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Status == domain.TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	changed := false
	if in.Status != nil {
		if !domain.IsTicketStatus(*in.Status) || *in.Status == domain.TicketStatusPending {
			return nil, ErrTicketInvalidInput
		}
		t.Status = *in.Status
		changed = true
	}
	if in.Priority != nil {
		if !domain.IsTicketPriority(*in.Priority) {
			return nil, ErrTicketInvalidInput
		}
		t.Priority = *in.Priority
		changed = true
	}
	if !changed {
		return nil, ErrTicketInvalidInput
	}
	t.UpdatedAt = time.Now()
	t.LastActivityAt = t.UpdatedAt
	if err := s.repo.Update(ctx, &t.Ticket); err != nil {
		return nil, err
	}
	return &t.Ticket, nil
}
