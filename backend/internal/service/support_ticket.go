package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
	ErrSupportTicketInvalidSubject      = infraerrors.BadRequest("SUPPORT_TICKET_SUBJECT_INVALID", "support ticket subject is invalid")
	ErrSupportTicketInvalidContent      = infraerrors.BadRequest("SUPPORT_TICKET_CONTENT_INVALID", "support ticket content is invalid")
	ErrSupportTicketInvalidStatus       = infraerrors.BadRequest("SUPPORT_TICKET_STATUS_INVALID", "support ticket status is invalid")
	ErrSupportTicketInvalidPriority     = infraerrors.BadRequest("SUPPORT_TICKET_PRIORITY_INVALID", "support ticket priority is invalid")
	ErrSupportTicketInvalidCategory     = infraerrors.BadRequest("SUPPORT_TICKET_CATEGORY_INVALID", "support ticket category is invalid")
	ErrSupportTicketAttachmentsDisabled = infraerrors.ServiceUnavailable("SUPPORT_TICKET_ATTACHMENTS_DISABLED", "support ticket image attachments are not configured")
	ErrSupportTicketTooManyAttachments  = infraerrors.BadRequest("SUPPORT_TICKET_ATTACHMENTS_TOO_MANY", "too many support ticket attachments")
	ErrSupportTicketAttachmentTooLarge  = infraerrors.BadRequest("SUPPORT_TICKET_ATTACHMENT_TOO_LARGE", "support ticket attachment is too large")
	ErrSupportTicketAttachmentType      = infraerrors.BadRequest("SUPPORT_TICKET_ATTACHMENT_TYPE_INVALID", "support ticket attachment must be a JPEG, PNG, GIF, or WebP image")
	ErrSupportTicketAttachmentUpload    = infraerrors.ServiceUnavailable("SUPPORT_TICKET_ATTACHMENT_UPLOAD_FAILED", "support ticket attachment upload failed")
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
	ID          int64                     `json:"id"`
	TicketID    int64                     `json:"ticket_id"`
	SenderID    int64                     `json:"sender_id"`
	SenderRole  string                    `json:"sender_role"`
	Content     string                    `json:"content"`
	CreatedAt   time.Time                 `json:"created_at"`
	Attachments []SupportTicketAttachment `json:"attachments,omitempty"`
}

type SupportTicketAttachment struct {
	ID          int64     `json:"id"`
	TicketID    int64     `json:"ticket_id"`
	MessageID   int64     `json:"message_id"`
	UploaderID  int64     `json:"uploader_id"`
	ObjectKey   string    `json:"-"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type SupportTicketAttachmentUpload struct {
	FileName string
	Data     []byte
}

type SupportTicketAttachmentPolicy struct {
	Enabled                  bool  `json:"enabled"`
	MaxFileSizeBytes         int64 `json:"max_file_size_bytes"`
	MaxAttachmentsPerMessage int   `json:"max_attachments_per_message"`
}

type SupportTicketListFilters struct {
	UserID   *int64
	Status   string
	Category string
	Priority string
	Search   string
}

type CreateSupportTicketInput struct {
	Subject     string
	Category    string
	Priority    string
	Content     string
	Attachments []SupportTicketAttachmentUpload
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

type SupportTicketAttachmentStore interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type SupportTicketNotifier interface {
	TicketCreated(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage)
	UserReplied(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage)
	AdminReplied(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage)
	StatusChanged(ctx context.Context, ticket *SupportTicket, oldStatus string, adminID int64)
}

type SupportTicketAttachmentDownload struct {
	Attachment SupportTicketAttachment
	Body       io.ReadCloser
}

type SupportTicketService struct {
	repo            SupportTicketRepository
	attachmentStore SupportTicketAttachmentStore
	attachmentCfg   config.SupportTicketAttachmentConfig
	now             func() time.Time
}

func NewSupportTicketService(repo SupportTicketRepository, attachmentStore SupportTicketAttachmentStore, cfg *config.Config) *SupportTicketService {
	attachmentCfg := config.SupportTicketAttachmentConfig{}
	if cfg != nil {
		attachmentCfg = cfg.SupportTicket.Attachments
	}
	return &SupportTicketService{repo: repo, attachmentStore: attachmentStore, attachmentCfg: attachmentCfg, now: time.Now}
}

func (s *SupportTicketService) AttachmentPolicy() SupportTicketAttachmentPolicy {
	attachmentCfg := s.currentAttachmentConfig()
	maxFileSizeMB := attachmentCfg.MaxFileSizeMB
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = 10
	}
	maxAttachments := attachmentCfg.MaxAttachmentsMessage
	if maxAttachments <= 0 {
		maxAttachments = 4
	}
	return SupportTicketAttachmentPolicy{
		Enabled:                  attachmentCfg.Enabled && s.attachmentStore != nil,
		MaxFileSizeBytes:         int64(maxFileSizeMB) * 1024 * 1024,
		MaxAttachmentsPerMessage: maxAttachments,
	}
}

func (s *SupportTicketService) CreateForUser(ctx context.Context, userID int64, input CreateSupportTicketInput) (*SupportTicket, error) {
	subject := strings.TrimSpace(input.Subject)
	content := strings.TrimSpace(input.Content)
	category := strings.TrimSpace(input.Category)
	priority := strings.TrimSpace(input.Priority)
	if subject == "" || len([]rune(subject)) > 200 {
		return nil, ErrSupportTicketInvalidSubject
	}
	if !validSupportTicketContent(content, len(input.Attachments) > 0) {
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
	attachments, err := s.uploadAttachments(ctx, userID, input.Attachments, now)
	if err != nil {
		return nil, err
	}
	message := &SupportTicketMessage{SenderID: userID, SenderRole: SupportTicketSenderUser, Content: content, CreatedAt: now, Attachments: attachments}
	if err := s.repo.Create(ctx, ticket, message); err != nil {
		s.deleteAttachments(ctx, attachments)
		return nil, err
	}
	s.hydrateAttachmentURLs(ctx, ticket)
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
	s.hydrateAttachmentURLs(ctx, ticket)
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
	s.hydrateAttachmentURLs(ctx, ticket)
	return ticket, nil
}

func (s *SupportTicketService) DownloadAttachmentForUser(ctx context.Context, userID, ticketID, attachmentID int64) (*SupportTicketAttachmentDownload, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	return s.downloadAttachment(ctx, ticket, attachmentID)
}

func (s *SupportTicketService) DownloadAttachmentForAdmin(ctx context.Context, ticketID, attachmentID int64) (*SupportTicketAttachmentDownload, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return s.downloadAttachment(ctx, ticket, attachmentID)
}

func (s *SupportTicketService) ReplyAsUser(ctx context.Context, userID, ticketID int64, content string) (*SupportTicket, error) {
	return s.ReplyAsUserWithAttachments(ctx, userID, ticketID, content, nil)
}

func (s *SupportTicketService) ReplyAsUserWithAttachments(ctx context.Context, userID, ticketID int64, content string, uploads []SupportTicketAttachmentUpload) (*SupportTicket, error) {
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
	if !validSupportTicketContent(content, len(uploads) > 0) {
		return nil, ErrSupportTicketInvalidContent
	}
	if ticket.Status == SupportTicketStatusWaitingUser || ticket.Status == SupportTicketStatusResolved {
		ticket.Status = SupportTicketStatusOpen
		ticket.ClosedAt = nil
	}
	now := s.now().UTC()
	attachments, err := s.uploadAttachments(ctx, userID, uploads, now)
	if err != nil {
		return nil, err
	}
	ticket.LastMessageAt = now
	ticket.AdminUnread = true
	ticket.UserUnread = false
	message := &SupportTicketMessage{TicketID: ticketID, SenderID: userID, SenderRole: SupportTicketSenderUser, Content: content, CreatedAt: now, Attachments: attachments}
	if err := s.repo.AddMessage(ctx, ticket, message); err != nil {
		s.deleteAttachments(ctx, attachments)
		return nil, err
	}
	s.hydrateAttachmentURLs(ctx, ticket)
	return ticket, nil
}

func (s *SupportTicketService) ReplyAsAdmin(ctx context.Context, adminID, ticketID int64, content string) (*SupportTicket, error) {
	return s.ReplyAsAdminWithAttachments(ctx, adminID, ticketID, content, nil)
}

func (s *SupportTicketService) ReplyAsAdminWithAttachments(ctx context.Context, adminID, ticketID int64, content string, uploads []SupportTicketAttachmentUpload) (*SupportTicket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status == SupportTicketStatusClosed {
		return nil, ErrSupportTicketClosed
	}
	content = strings.TrimSpace(content)
	if !validSupportTicketContent(content, len(uploads) > 0) {
		return nil, ErrSupportTicketInvalidContent
	}
	now := s.now().UTC()
	attachments, err := s.uploadAttachments(ctx, adminID, uploads, now)
	if err != nil {
		return nil, err
	}
	ticket.Status = SupportTicketStatusWaitingUser
	ticket.ClosedAt = nil
	ticket.LastMessageAt = now
	ticket.UserUnread = true
	ticket.AdminUnread = false
	message := &SupportTicketMessage{TicketID: ticketID, SenderID: adminID, SenderRole: SupportTicketSenderAdmin, Content: content, CreatedAt: now, Attachments: attachments}
	if err := s.repo.AddMessage(ctx, ticket, message); err != nil {
		s.deleteAttachments(ctx, attachments)
		return nil, err
	}
	s.hydrateAttachmentURLs(ctx, ticket)
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

func validSupportTicketContent(content string, hasAttachments bool) bool {
	length := len([]rune(content))
	return length <= 10000 && (length > 0 || hasAttachments)
}

func (s *SupportTicketService) uploadAttachments(ctx context.Context, uploaderID int64, uploads []SupportTicketAttachmentUpload, createdAt time.Time) ([]SupportTicketAttachment, error) {
	if len(uploads) == 0 {
		return nil, nil
	}
	slog.Info("support_ticket_attachment upload_started", "uploader_id", uploaderID, "attachment_count", len(uploads))
	policy := s.AttachmentPolicy()
	if !policy.Enabled {
		return nil, ErrSupportTicketAttachmentsDisabled
	}
	if len(uploads) > policy.MaxAttachmentsPerMessage {
		return nil, ErrSupportTicketTooManyAttachments
	}

	attachments := make([]SupportTicketAttachment, 0, len(uploads))
	for _, upload := range uploads {
		if int64(len(upload.Data)) == 0 || int64(len(upload.Data)) > policy.MaxFileSizeBytes {
			slog.Warn("support_ticket_attachment rejected_size", "uploader_id", uploaderID, "size_bytes", len(upload.Data), "max_size_bytes", policy.MaxFileSizeBytes)
			s.deleteAttachments(ctx, attachments)
			return nil, ErrSupportTicketAttachmentTooLarge
		}
		contentType := http.DetectContentType(upload.Data)
		extension, ok := supportTicketImageExtension(contentType)
		if !ok {
			slog.Warn("support_ticket_attachment rejected_content_type", "uploader_id", uploaderID, "content_type", contentType, "size_bytes", len(upload.Data))
			s.deleteAttachments(ctx, attachments)
			return nil, ErrSupportTicketAttachmentType
		}
		objectKey, err := s.newAttachmentObjectKey(uploaderID, extension, createdAt)
		if err != nil {
			slog.Error("support_ticket_attachment object_key_generation_failed", "uploader_id", uploaderID, "error", err)
			s.deleteAttachments(ctx, attachments)
			return nil, ErrSupportTicketAttachmentUpload
		}
		slog.Info("support_ticket_attachment upload_attempt", "uploader_id", uploaderID, "object_key", objectKey, "content_type", contentType, "size_bytes", len(upload.Data))
		if err := s.attachmentStore.Upload(ctx, objectKey, bytes.NewReader(upload.Data), int64(len(upload.Data)), contentType); err != nil {
			slog.Error("support_ticket_attachment upload_failed", "uploader_id", uploaderID, "object_key", objectKey, "content_type", contentType, "size_bytes", len(upload.Data), "error", err)
			s.deleteAttachments(ctx, attachments)
			return nil, fmt.Errorf("%w: %v", ErrSupportTicketAttachmentUpload, err)
		}
		slog.Info("support_ticket_attachment upload_succeeded", "uploader_id", uploaderID, "object_key", objectKey, "content_type", contentType, "size_bytes", len(upload.Data))
		attachments = append(attachments, SupportTicketAttachment{
			UploaderID: uploaderID, ObjectKey: objectKey, FileName: normalizeAttachmentFileName(upload.FileName, extension),
			ContentType: contentType, SizeBytes: int64(len(upload.Data)), CreatedAt: createdAt,
		})
	}
	slog.Info("support_ticket_attachment upload_completed", "uploader_id", uploaderID, "attachment_count", len(attachments))
	return attachments, nil
}

func (s *SupportTicketService) deleteAttachments(ctx context.Context, attachments []SupportTicketAttachment) {
	if s.attachmentStore == nil {
		return
	}
	for _, attachment := range attachments {
		if err := s.attachmentStore.Delete(ctx, attachment.ObjectKey); err != nil {
			slog.Error("support_ticket_attachment cleanup_failed", "object_key", attachment.ObjectKey, "error", err)
		}
	}
}

func (s *SupportTicketService) downloadAttachment(ctx context.Context, ticket *SupportTicket, attachmentID int64) (*SupportTicketAttachmentDownload, error) {
	if ticket == nil || s.attachmentStore == nil || !s.currentAttachmentConfig().Enabled {
		return nil, ErrSupportTicketNotFound
	}
	for _, message := range ticket.Messages {
		for _, attachment := range message.Attachments {
			if attachment.ID != attachmentID {
				continue
			}
			body, err := s.attachmentStore.Download(ctx, attachment.ObjectKey)
			if err != nil {
				return nil, fmt.Errorf("download support ticket attachment: %w", err)
			}
			return &SupportTicketAttachmentDownload{Attachment: attachment, Body: body}, nil
		}
	}
	return nil, ErrSupportTicketNotFound
}

func (s *SupportTicketService) hydrateAttachmentURLs(ctx context.Context, ticket *SupportTicket) {
	attachmentCfg := s.currentAttachmentConfig()
	if ticket == nil || s.attachmentStore == nil || !attachmentCfg.Enabled {
		return
	}
	expiryMinutes := attachmentCfg.URLExpiryMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}
	for messageIndex := range ticket.Messages {
		for attachmentIndex := range ticket.Messages[messageIndex].Attachments {
			attachment := &ticket.Messages[messageIndex].Attachments[attachmentIndex]
			url, err := s.attachmentStore.PresignURL(ctx, attachment.ObjectKey, time.Duration(expiryMinutes)*time.Minute)
			if err == nil {
				attachment.URL = url
			}
		}
	}
}

func (s *SupportTicketService) newAttachmentObjectKey(uploaderID int64, extension string, createdAt time.Time) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	prefix := strings.Trim(strings.TrimSpace(s.currentAttachmentConfig().Prefix), "/")
	fileName := hex.EncodeToString(randomBytes) + extension
	key := path.Join(createdAt.UTC().Format("2006/01/02"), fmt.Sprintf("%d", uploaderID), fileName)
	if prefix != "" {
		key = path.Join(prefix, key)
	}
	return key, nil
}

func (s *SupportTicketService) currentAttachmentConfig() config.SupportTicketAttachmentConfig {
	if provider, ok := s.attachmentStore.(supportTicketAttachmentConfigProvider); ok {
		return provider.CurrentConfig()
	}
	return s.attachmentCfg
}

func supportTicketImageExtension(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func normalizeAttachmentFileName(fileName, extension string) string {
	fileName = path.Base(strings.ReplaceAll(strings.TrimSpace(fileName), "\\", "/"))
	if fileName == "" || fileName == "." {
		fileName = "image" + extension
	}
	runes := []rune(fileName)
	if len(runes) > 255 {
		fileName = string(runes[:255])
	}
	return fileName
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
