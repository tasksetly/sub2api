package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type supportTicketRepoStub struct {
	ticket *SupportTicket
}

func (s *supportTicketRepoStub) Create(_ context.Context, ticket *SupportTicket, message *SupportTicketMessage) error {
	ticket.ID = 1
	message.ID = 1
	message.TicketID = ticket.ID
	ticket.Messages = []SupportTicketMessage{*message}
	s.ticket = ticket
	return nil
}

func (s *supportTicketRepoStub) GetByID(_ context.Context, id int64) (*SupportTicket, error) {
	if s.ticket == nil || s.ticket.ID != id {
		return nil, ErrSupportTicketNotFound
	}
	return s.ticket, nil
}

func (s *supportTicketRepoStub) List(context.Context, pagination.PaginationParams, SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	if s.ticket == nil {
		return nil, &pagination.PaginationResult{}, nil
	}
	return []SupportTicket{*s.ticket}, &pagination.PaginationResult{Total: 1}, nil
}

func (s *supportTicketRepoStub) AddMessage(_ context.Context, ticket *SupportTicket, message *SupportTicketMessage) error {
	message.ID = int64(len(ticket.Messages) + 1)
	message.TicketID = ticket.ID
	ticket.Messages = append(ticket.Messages, *message)
	s.ticket = ticket
	return nil
}

func (s *supportTicketRepoStub) Update(_ context.Context, ticket *SupportTicket) error {
	s.ticket = ticket
	return nil
}

func (s *supportTicketRepoStub) MarkRead(_ context.Context, ticketID int64, readerRole string) error {
	if s.ticket == nil || s.ticket.ID != ticketID {
		return ErrSupportTicketNotFound
	}
	if readerRole == SupportTicketSenderAdmin {
		s.ticket.AdminUnread = false
	} else {
		s.ticket.UserUnread = false
	}
	return nil
}

func TestSupportTicketWorkflow(t *testing.T) {
	repo := &supportTicketRepoStub{}
	svc := NewSupportTicketService(repo)
	ctx := context.Background()

	ticket, err := svc.CreateForUser(ctx, 7, CreateSupportTicketInput{
		Subject:  "API requests fail",
		Category: SupportTicketCategoryTechnical,
		Priority: SupportTicketPriorityHigh,
		Content:  "Every request returns 502.",
	})
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusOpen, ticket.Status)
	require.True(t, ticket.AdminUnread)

	ticket, err = svc.ReplyAsAdmin(ctx, 99, ticket.ID, "Please provide a request ID.")
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusWaitingUser, ticket.Status)
	require.True(t, ticket.UserUnread)

	ticket, err = svc.ReplyAsUser(ctx, 7, ticket.ID, "The request ID is req_123.")
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusOpen, ticket.Status)
	require.True(t, ticket.AdminUnread)

	ticket, err = svc.CloseAsUser(ctx, 7, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusClosed, ticket.Status)
	require.NotNil(t, ticket.ClosedAt)

	_, err = svc.ReplyAsUser(ctx, 7, ticket.ID, "One more thing")
	require.ErrorIs(t, err, ErrSupportTicketClosed)
}

func TestSupportTicketUserCannotReadAnotherUsersTicket(t *testing.T) {
	repo := &supportTicketRepoStub{ticket: &SupportTicket{
		ID:        1,
		UserID:    8,
		Subject:   "private",
		Status:    SupportTicketStatusOpen,
		CreatedAt: time.Now(),
	}}
	svc := NewSupportTicketService(repo)

	_, err := svc.GetForUser(context.Background(), 7, 1)
	require.ErrorIs(t, err, ErrSupportTicketNotFound)
}

func TestSupportTicketAdminRejectsInvalidTransition(t *testing.T) {
	repo := &supportTicketRepoStub{ticket: &SupportTicket{
		ID:       1,
		UserID:   8,
		Subject:  "closed",
		Status:   SupportTicketStatusClosed,
		ClosedAt: ptrSupportTime(time.Now()),
	}}
	svc := NewSupportTicketService(repo)

	_, err := svc.UpdateAsAdmin(context.Background(), 99, 1, UpdateSupportTicketInput{
		Status: ptrString(SupportTicketStatusResolved),
	})
	require.ErrorIs(t, err, ErrSupportTicketInvalidTransition)
}

func ptrString(value string) *string            { return &value }
func ptrSupportTime(value time.Time) *time.Time { return &value }
