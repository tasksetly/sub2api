package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const settingKeyTicketStorageConfig = "ticket_storage_config"

const (
	defaultTicketMaxFileSizeMB    = 10
	defaultTicketMaxFiles         = 5
	defaultTicketAttachmentPrefix = "tickets/"
)

var (
	ErrTicketNotFound          = domain.ErrTicketNotFound
	ErrTicketSubjectInvalid    = infraerrors.BadRequest("TICKET_SUBJECT_INVALID", "ticket subject must be between 1 and 200 characters")
	ErrTicketMessageRequired   = infraerrors.BadRequest("TICKET_MESSAGE_REQUIRED", "ticket message or image is required")
	ErrTicketMessageTooLong    = infraerrors.BadRequest("TICKET_MESSAGE_TOO_LONG", "ticket message is too long")
	ErrTicketClosed            = infraerrors.Conflict("TICKET_CLOSED", "closed tickets cannot receive replies")
	ErrTicketStatusInvalid     = infraerrors.BadRequest("TICKET_STATUS_INVALID", "invalid ticket status")
	ErrTicketPriorityInvalid   = infraerrors.BadRequest("TICKET_PRIORITY_INVALID", "invalid ticket priority")
	ErrTicketStorageDisabled   = infraerrors.BadRequest("TICKET_STORAGE_DISABLED", "ticket image storage is not enabled")
	ErrTicketStorageIncomplete = infraerrors.BadRequest("TICKET_STORAGE_INCOMPLETE", "ticket image storage configuration is incomplete")
	ErrTicketImageInvalid      = infraerrors.BadRequest("TICKET_IMAGE_INVALID", "only PNG, JPEG, GIF, and WebP images are supported")
	ErrTicketImageTooLarge     = infraerrors.BadRequest("TICKET_IMAGE_TOO_LARGE", "ticket image exceeds the configured size limit")
	ErrTicketTooManyImages     = infraerrors.BadRequest("TICKET_TOO_MANY_IMAGES", "too many images attached to one message")
	ErrTicketImageUpload       = infraerrors.ServiceUnavailable("TICKET_IMAGE_UPLOAD_FAILED", "failed to upload ticket image")
	ErrTicketAttachmentMissing = infraerrors.NotFound("TICKET_ATTACHMENT_NOT_FOUND", "ticket attachment not found")
)

type Ticket = domain.Ticket
type TicketMessage = domain.TicketMessage
type TicketAttachment = domain.TicketAttachment

type TicketListFilters struct {
	Status   string
	Priority string
	Category string
	Search   string
	UserID   int64
}

type TicketRepository interface {
	Create(ctx context.Context, ticket *Ticket, message *TicketMessage) error
	List(ctx context.Context, params pagination.PaginationParams, filters TicketListFilters) ([]Ticket, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*Ticket, error)
	AddMessage(ctx context.Context, message *TicketMessage, nextStatus string) error
	Update(ctx context.Context, id int64, status, priority string, closedAt *time.Time) (*Ticket, error)
}

type TicketStorageConfig struct {
	Enabled              bool   `json:"enabled"`
	Endpoint             string `json:"endpoint"`
	Region               string `json:"region"`
	Bucket               string `json:"bucket"`
	AccessKeyID          string `json:"access_key_id"`
	SecretAccessKey      string `json:"secret_access_key,omitempty"` //nolint:revive // follows S3 naming
	HasSecret            bool   `json:"has_secret,omitempty"`
	Prefix               string `json:"prefix"`
	ForcePathStyle       bool   `json:"force_path_style"`
	MaxFileSizeMB        int    `json:"max_file_size_mb"`
	MaxFilesPerMessage   int    `json:"max_files_per_message"`
}

func (c *TicketStorageConfig) normalize() {
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.Region = strings.TrimSpace(c.Region)
	c.Bucket = strings.TrimSpace(c.Bucket)
	c.AccessKeyID = strings.TrimSpace(c.AccessKeyID)
	c.SecretAccessKey = strings.TrimSpace(c.SecretAccessKey)
	c.Prefix = strings.Trim(strings.TrimSpace(c.Prefix), "/")
	if c.Region == "" {
		c.Region = "auto"
	}
	if c.Prefix == "" {
		c.Prefix = strings.TrimSuffix(defaultTicketAttachmentPrefix, "/")
	}
	if c.MaxFileSizeMB <= 0 || c.MaxFileSizeMB > 50 {
		c.MaxFileSizeMB = defaultTicketMaxFileSizeMB
	}
	if c.MaxFilesPerMessage <= 0 || c.MaxFilesPerMessage > 10 {
		c.MaxFilesPerMessage = defaultTicketMaxFiles
	}
}

func (c *TicketStorageConfig) configured() bool {
	return c.Bucket != "" && c.AccessKeyID != "" && c.SecretAccessKey != ""
}

type TicketUpload struct {
	Name string
	Data []byte
}

type TicketService struct {
	repo         TicketRepository
	settingRepo  SettingRepository
	encryptor    SecretEncryptor
	storeFactory BackupObjectStoreFactory
	storeMu      sync.Mutex
	cachedConfig *TicketStorageConfig
	cachedStore  BackupObjectStore
}

func NewTicketService(repo TicketRepository, settingRepo SettingRepository, encryptor SecretEncryptor, storeFactory BackupObjectStoreFactory) *TicketService {
	return &TicketService{repo: repo, settingRepo: settingRepo, encryptor: encryptor, storeFactory: storeFactory}
}

func (s *TicketService) Create(ctx context.Context, userID int64, subject, category, content string, uploads []TicketUpload) (*Ticket, error) {
	subject = strings.TrimSpace(subject)
	content = strings.TrimSpace(content)
	category = normalizeTicketCategory(category)
	if subject == "" || len([]rune(subject)) > 200 {
		return nil, ErrTicketSubjectInvalid
	}
	if len([]rune(content)) > 10000 {
		return nil, ErrTicketMessageTooLong
	}
	if content == "" && len(uploads) == 0 {
		return nil, ErrTicketMessageRequired
	}

	attachments, store, err := s.uploadImages(ctx, uploads)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ticket := &Ticket{UserID: userID, Subject: subject, Category: category, Status: domain.TicketStatusOpen, Priority: domain.TicketPriorityNormal, LastMessageAt: now}
	message := &TicketMessage{AuthorID: userID, AuthorRole: domain.TicketAuthorUser, Content: content, Attachments: attachments, CreatedAt: now}
	if err := s.repo.Create(ctx, ticket, message); err != nil {
		s.deleteUploaded(ctx, store, attachments)
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	return s.Get(ctx, ticket.ID, userID, false)
}

func (s *TicketService) Reply(ctx context.Context, ticketID, actorID int64, isAdmin bool, content string, uploads []TicketUpload) (*Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if !isAdmin && ticket.UserID != actorID {
		return nil, ErrTicketNotFound
	}
	if ticket.Status == domain.TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	content = strings.TrimSpace(content)
	if len([]rune(content)) > 10000 {
		return nil, ErrTicketMessageTooLong
	}
	if content == "" && len(uploads) == 0 {
		return nil, ErrTicketMessageRequired
	}
	attachments, store, err := s.uploadImages(ctx, uploads)
	if err != nil {
		return nil, err
	}
	role := domain.TicketAuthorUser
	nextStatus := domain.TicketStatusOpen
	if isAdmin {
		role = domain.TicketAuthorAdmin
		nextStatus = domain.TicketStatusWaitingUser
	}
	message := &TicketMessage{TicketID: ticketID, AuthorID: actorID, AuthorRole: role, Content: content, Attachments: attachments, CreatedAt: time.Now()}
	if err := s.repo.AddMessage(ctx, message, nextStatus); err != nil {
		s.deleteUploaded(ctx, store, attachments)
		return nil, err
	}
	return s.Get(ctx, ticketID, actorID, isAdmin)
}

func (s *TicketService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters TicketListFilters) ([]Ticket, *pagination.PaginationResult, error) {
	filters.UserID = userID
	return s.repo.List(ctx, params, filters)
}

func (s *TicketService) ListAdmin(ctx context.Context, params pagination.PaginationParams, filters TicketListFilters) ([]Ticket, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, params, filters)
}

func (s *TicketService) Get(ctx context.Context, ticketID, requesterID int64, isAdmin bool) (*Ticket, error) {
	return s.getAuthorized(ctx, ticketID, requesterID, isAdmin)
}

func (s *TicketService) getAuthorized(ctx context.Context, ticketID, requesterID int64, isAdmin bool) (*Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if !isAdmin && ticket.UserID != requesterID {
		return nil, ErrTicketNotFound
	}
	return ticket, nil
}

func (s *TicketService) Close(ctx context.Context, ticketID, requesterID int64, isAdmin bool) (*Ticket, error) {
	ticket, err := s.getAuthorized(ctx, ticketID, requesterID, isAdmin)
	if err != nil {
		return nil, err
	}
	if ticket.Status == domain.TicketStatusClosed {
		return ticket, nil
	}
	now := time.Now()
	updated, err := s.repo.Update(ctx, ticketID, domain.TicketStatusClosed, ticket.Priority, &now)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TicketService) AdminUpdate(ctx context.Context, ticketID int64, status, priority string) (*Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = ticket.Status
	}
	if priority == "" {
		priority = ticket.Priority
	}
	if !validTicketStatus(status) {
		return nil, ErrTicketStatusInvalid
	}
	if !validTicketPriority(priority) {
		return nil, ErrTicketPriorityInvalid
	}
	var closedAt *time.Time
	if status == domain.TicketStatusClosed {
		now := time.Now()
		closedAt = &now
	}
	updated, err := s.repo.Update(ctx, ticketID, status, priority, closedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TicketService) GetStorageConfig(ctx context.Context) (*TicketStorageConfig, error) {
	cfg, err := s.loadStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	cfg.HasSecret = cfg.SecretAccessKey != ""
	cfg.SecretAccessKey = ""
	return cfg, nil
}

func (s *TicketService) UpdateStorageConfig(ctx context.Context, input TicketStorageConfig) (*TicketStorageConfig, error) {
	old, err := s.loadStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	input.normalize()
	if input.SecretAccessKey == "" {
		input.SecretAccessKey = old.SecretAccessKey
	}
	if input.Enabled && !input.configured() {
		return nil, ErrTicketStorageIncomplete
	}
	if input.SecretAccessKey != "" {
		encrypted, err := s.encryptor.Encrypt(input.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt ticket storage secret: %w", err)
		}
		input.SecretAccessKey = encrypted
	}
	input.HasSecret = false
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, settingKeyTicketStorageConfig, string(data)); err != nil {
		return nil, fmt.Errorf("save ticket storage config: %w", err)
	}
	s.storeMu.Lock()
	s.cachedConfig = nil
	s.cachedStore = nil
	s.storeMu.Unlock()
	return s.GetStorageConfig(ctx)
}

func (s *TicketService) TestStorage(ctx context.Context, input TicketStorageConfig) error {
	stored, err := s.loadStorageConfig(ctx)
	if err != nil {
		return err
	}
	input.normalize()
	if input.SecretAccessKey == "" {
		input.SecretAccessKey = stored.SecretAccessKey
	}
	if !input.configured() {
		return ErrTicketStorageIncomplete
	}
	store, err := s.newStore(ctx, &input)
	if err != nil {
		return err
	}
	return store.HeadBucket(ctx)
}

func (s *TicketService) DownloadAttachment(ctx context.Context, ticketID, messageID int64, index int, requesterID int64, isAdmin bool) (io.ReadCloser, *TicketAttachment, error) {
	ticket, err := s.getAuthorized(ctx, ticketID, requesterID, isAdmin)
	if err != nil {
		return nil, nil, err
	}
	for i := range ticket.Messages {
		message := &ticket.Messages[i]
		if message.ID != messageID || index < 0 || index >= len(message.Attachments) {
			continue
		}
		cfg, err := s.loadStorageConfig(ctx)
		if err != nil {
			return nil, nil, err
		}
		if !cfg.configured() {
			return nil, nil, ErrTicketStorageIncomplete
		}
		store, err := s.newStore(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		attachment := message.Attachments[index]
		reader, err := store.Download(ctx, attachment.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("download ticket attachment: %w", err)
		}
		return reader, &attachment, nil
	}
	return nil, nil, ErrTicketAttachmentMissing
}

func (s *TicketService) uploadImages(ctx context.Context, uploads []TicketUpload) ([]TicketAttachment, BackupObjectStore, error) {
	if len(uploads) == 0 {
		return []TicketAttachment{}, nil, nil
	}
	cfg, err := s.loadStorageConfig(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !cfg.Enabled {
		return nil, nil, ErrTicketStorageDisabled
	}
	if !cfg.configured() {
		return nil, nil, ErrTicketStorageIncomplete
	}
	if len(uploads) > cfg.MaxFilesPerMessage {
		return nil, nil, ErrTicketTooManyImages
	}
	store, err := s.newStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	attachments := make([]TicketAttachment, 0, len(uploads))
	maxBytes := int64(cfg.MaxFileSizeMB) << 20
	for _, upload := range uploads {
		if int64(len(upload.Data)) > maxBytes {
			s.deleteUploaded(ctx, store, attachments)
			return nil, nil, ErrTicketImageTooLarge
		}
		contentType := strings.TrimSpace(strings.Split(http.DetectContentType(upload.Data), ";")[0])
		ext, ok := ticketImageExtensions[contentType]
		if !ok {
			s.deleteUploaded(ctx, store, attachments)
			return nil, nil, ErrTicketImageInvalid
		}
		key := fmt.Sprintf("%s/%s/%s%s", cfg.Prefix, time.Now().Format("2006/01"), uuid.NewString(), ext)
		if _, err := store.Upload(ctx, key, bytes.NewReader(upload.Data), contentType); err != nil {
			s.deleteUploaded(ctx, store, attachments)
			return nil, nil, fmt.Errorf("%w: %v", ErrTicketImageUpload, err)
		}
		name := strings.TrimSpace(filepath.Base(upload.Name))
		if name == "" || name == "." {
			name = "image" + ext
		}
		if runes := []rune(name); len(runes) > 255 {
			name = string(runes[:255])
		}
		attachments = append(attachments, TicketAttachment{Key: key, Name: name, ContentType: contentType, Size: int64(len(upload.Data))})
	}
	return attachments, store, nil
}

func (s *TicketService) deleteUploaded(ctx context.Context, store BackupObjectStore, attachments []TicketAttachment) {
	if store == nil {
		return
	}
	for i := range attachments {
		_ = store.Delete(ctx, attachments[i].Key)
	}
}

func (s *TicketService) loadStorageConfig(ctx context.Context) (*TicketStorageConfig, error) {
	cfg := &TicketStorageConfig{}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyTicketStorageConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			cfg.normalize()
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, infraerrors.InternalServer("TICKET_STORAGE_CONFIG_CORRUPT", "ticket storage configuration is corrupted")
	}
	if cfg.SecretAccessKey != "" {
		secret, err := s.encryptor.Decrypt(cfg.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt ticket storage secret: %w", err)
		}
		cfg.SecretAccessKey = secret
	}
	cfg.normalize()
	return cfg, nil
}

func (s *TicketService) newStore(ctx context.Context, cfg *TicketStorageConfig) (BackupObjectStore, error) {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if s.cachedConfig != nil && s.cachedStore != nil && sameTicketStorageConfig(s.cachedConfig, cfg) {
		return s.cachedStore, nil
	}
	store, err := s.storeFactory(ctx, &BackupS3Config{Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: cfg.Bucket, AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, Prefix: cfg.Prefix, ForcePathStyle: cfg.ForcePathStyle})
	if err != nil {
		return nil, err
	}
	copy := *cfg
	s.cachedConfig = &copy
	s.cachedStore = store
	return store, nil
}

func sameTicketStorageConfig(a, b *TicketStorageConfig) bool {
	return a.Endpoint == b.Endpoint && a.Region == b.Region && a.Bucket == b.Bucket &&
		a.AccessKeyID == b.AccessKeyID && a.SecretAccessKey == b.SecretAccessKey &&
		a.ForcePathStyle == b.ForcePathStyle
}

var ticketImageExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func normalizeTicketCategory(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "billing", "technical", "account", "suggestion", "other":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "other"
	}
}

func validTicketStatus(value string) bool {
	switch value {
	case domain.TicketStatusOpen, domain.TicketStatusInProgress, domain.TicketStatusWaitingUser, domain.TicketStatusClosed:
		return true
	default:
		return false
	}
}

func validTicketPriority(value string) bool {
	switch value {
	case domain.TicketPriorityLow, domain.TicketPriorityNormal, domain.TicketPriorityHigh, domain.TicketPriorityUrgent:
		return true
	default:
		return false
	}
}
