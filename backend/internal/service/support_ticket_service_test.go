package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
	for i := range message.Attachments {
		message.Attachments[i].ID = int64(i + 1)
		message.Attachments[i].TicketID = ticket.ID
		message.Attachments[i].MessageID = message.ID
	}
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
	for i := range message.Attachments {
		message.Attachments[i].ID = int64(i + 1)
		message.Attachments[i].TicketID = ticket.ID
		message.Attachments[i].MessageID = message.ID
	}
	ticket.Messages = append(ticket.Messages, *message)
	s.ticket = ticket
	return nil
}

type supportTicketAttachmentStoreStub struct {
	uploads map[string][]byte
	deleted []string
}

func (s *supportTicketAttachmentStoreStub) Upload(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.uploads == nil {
		s.uploads = make(map[string][]byte)
	}
	s.uploads[key] = data
	return nil
}

func (s *supportTicketAttachmentStoreStub) Download(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.uploads[key]
	if !ok {
		return nil, ErrSupportTicketNotFound
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (s *supportTicketAttachmentStoreStub) Delete(_ context.Context, key string) error {
	delete(s.uploads, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *supportTicketAttachmentStoreStub) PresignURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://r2.example.test/" + key, nil
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
	svc := NewSupportTicketService(repo, nil, nil)
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
	svc := NewSupportTicketService(repo, nil, nil)

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
	svc := NewSupportTicketService(repo, nil, nil)

	_, err := svc.UpdateAsAdmin(context.Background(), 99, 1, UpdateSupportTicketInput{
		Status: ptrString(SupportTicketStatusResolved),
	})
	require.ErrorIs(t, err, ErrSupportTicketInvalidTransition)
}

func TestSupportTicketAllowsImageOnlyMessageWhenStorageConfigured(t *testing.T) {
	repo := &supportTicketRepoStub{}
	store := &supportTicketAttachmentStoreStub{}
	cfg := &config.Config{SupportTicket: config.SupportTicketConfig{Attachments: config.SupportTicketAttachmentConfig{
		Enabled: true, Prefix: "support-tickets", MaxFileSizeMB: 1, MaxAttachmentsMessage: 2, URLExpiryMinutes: 10,
	}}}
	svc := NewSupportTicketService(repo, store, cfg)

	ticket, err := svc.CreateForUser(context.Background(), 7, CreateSupportTicketInput{
		Subject: "Screenshot", Category: SupportTicketCategoryTechnical, Priority: SupportTicketPriorityNormal,
		Attachments: []SupportTicketAttachmentUpload{{FileName: "error.png", Data: []byte("\x89PNG\r\n\x1a\nimage")}},
	})
	require.NoError(t, err)
	require.Len(t, ticket.Messages, 1)
	require.Empty(t, ticket.Messages[0].Content)
	require.Len(t, ticket.Messages[0].Attachments, 1)
	attachment := ticket.Messages[0].Attachments[0]
	require.Equal(t, "image/png", attachment.ContentType)
	require.Equal(t, "error.png", attachment.FileName)
	require.True(t, strings.HasPrefix(attachment.ObjectKey, "support-tickets/"))
	require.True(t, strings.HasPrefix(attachment.URL, "https://r2.example.test/"))
	require.Len(t, store.uploads, 1)

	download, err := svc.DownloadAttachmentForUser(context.Background(), 7, ticket.ID, attachment.ID)
	require.NoError(t, err)
	defer download.Body.Close()
	body, err := io.ReadAll(download.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("\x89PNG\r\n\x1a\nimage"), body)
	require.Equal(t, attachment.FileName, download.Attachment.FileName)
}

func TestSupportTicketRejectsAttachmentWhenStorageDisabled(t *testing.T) {
	svc := NewSupportTicketService(&supportTicketRepoStub{}, nil, nil)
	_, err := svc.CreateForUser(context.Background(), 7, CreateSupportTicketInput{
		Subject: "Screenshot", Category: SupportTicketCategoryTechnical, Priority: SupportTicketPriorityNormal,
		Attachments: []SupportTicketAttachmentUpload{{FileName: "error.png", Data: []byte("\x89PNG\r\n\x1a\nimage")}},
	})
	require.ErrorIs(t, err, ErrSupportTicketAttachmentsDisabled)
}

func TestSupportTicketRejectsNonImageAttachment(t *testing.T) {
	store := &supportTicketAttachmentStoreStub{}
	cfg := &config.Config{SupportTicket: config.SupportTicketConfig{Attachments: config.SupportTicketAttachmentConfig{
		Enabled: true, MaxFileSizeMB: 1, MaxAttachmentsMessage: 2,
	}}}
	svc := NewSupportTicketService(&supportTicketRepoStub{}, store, cfg)
	_, err := svc.CreateForUser(context.Background(), 7, CreateSupportTicketInput{
		Subject: "Log file", Category: SupportTicketCategoryTechnical, Priority: SupportTicketPriorityNormal,
		Attachments: []SupportTicketAttachmentUpload{{FileName: "error.txt", Data: []byte("not an image")}},
	})
	require.ErrorIs(t, err, ErrSupportTicketAttachmentType)
	require.Empty(t, store.uploads)
}

func ptrString(value string) *string            { return &value }
func ptrSupportTime(value time.Time) *time.Time { return &value }
