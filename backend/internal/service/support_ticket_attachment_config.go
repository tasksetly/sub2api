package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const settingKeySupportTicketAttachmentStorage = "support_ticket_attachment_storage_config"

var (
	ErrSupportTicketStorageConfigInvalid = infraerrors.BadRequest(
		"SUPPORT_TICKET_STORAGE_CONFIG_INVALID",
		"support ticket attachment storage config is invalid",
	)
	ErrSupportTicketStorageConfigCorrupt = infraerrors.InternalServer(
		"SUPPORT_TICKET_STORAGE_CONFIG_CORRUPT",
		"support ticket attachment storage config is corrupted",
	)
)

type SupportTicketAttachmentStorageConfig struct {
	Enabled                  bool   `json:"enabled"`
	Endpoint                 string `json:"endpoint"`
	Region                   string `json:"region"`
	Bucket                   string `json:"bucket"`
	AccessKeyID              string `json:"access_key_id"`
	SecretAccessKey          string `json:"secret_access_key,omitempty"` //nolint:revive // AWS field name
	SecretConfigured         bool   `json:"secret_configured"`
	Prefix                   string `json:"prefix"`
	ForcePathStyle           bool   `json:"force_path_style"`
	MaxFileSizeMB            int    `json:"max_file_size_mb"`
	MaxAttachmentsPerMessage int    `json:"max_attachments_per_message"`
	URLExpiryMinutes         int    `json:"url_expiry_minutes"`
}

type SupportTicketAttachmentManager struct {
	settingRepo  SettingRepository
	encryptor    SecretEncryptor
	storeFactory SupportTicketAttachmentStoreFactory

	mu    sync.RWMutex
	cfg   config.SupportTicketAttachmentConfig
	store SupportTicketAttachmentObjectStore
}

func ProvideSupportTicketAttachmentManager(
	settingRepo SettingRepository,
	cfg *config.Config,
	encryptor SecretEncryptor,
	storeFactory SupportTicketAttachmentStoreFactory,
) (*SupportTicketAttachmentManager, error) {
	fallback := config.SupportTicketAttachmentConfig{}
	if cfg != nil {
		fallback = cfg.SupportTicket.Attachments
	}
	manager := &SupportTicketAttachmentManager{
		settingRepo:  settingRepo,
		encryptor:    encryptor,
		storeFactory: storeFactory,
	}
	active, err := manager.loadStoredConfig(context.Background())
	if err != nil {
		return nil, err
	}
	if active == nil {
		active = &fallback
	}
	normalized := normalizeSupportTicketAttachmentConfig(*active)
	if err := validateSupportTicketAttachmentConfig(normalized, false); err != nil {
		return nil, err
	}
	store, err := manager.createStore(context.Background(), normalized)
	if err != nil {
		return nil, err
	}
	manager.cfg = normalized
	manager.store = store
	if !normalized.Enabled {
		slog.Info("support_ticket_attachment storage_disabled")
	}
	return manager, nil
}

func (m *SupportTicketAttachmentManager) CurrentConfig() config.SupportTicketAttachmentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *SupportTicketAttachmentManager) GetConfig() SupportTicketAttachmentStorageConfig {
	cfg := m.CurrentConfig()
	result := supportTicketStorageConfigFromRuntime(cfg)
	result.SecretConfigured = strings.TrimSpace(cfg.SecretAccessKey) != ""
	result.SecretAccessKey = ""
	return result
}

func (m *SupportTicketAttachmentManager) UpdateConfig(
	ctx context.Context,
	input SupportTicketAttachmentStorageConfig,
) (SupportTicketAttachmentStorageConfig, error) {
	current := m.CurrentConfig()
	next := supportTicketStorageConfigToRuntime(input)
	if strings.TrimSpace(next.SecretAccessKey) == "" {
		next.SecretAccessKey = current.SecretAccessKey
	}
	next = normalizeSupportTicketAttachmentConfig(next)
	if err := validateSupportTicketAttachmentConfig(next, false); err != nil {
		return SupportTicketAttachmentStorageConfig{}, err
	}
	store, err := m.createStore(ctx, next)
	if err != nil {
		return SupportTicketAttachmentStorageConfig{}, fmt.Errorf("create support ticket attachment store: %w", err)
	}

	stored := supportTicketStorageConfigFromRuntime(next)
	stored.SecretConfigured = false
	if stored.SecretAccessKey != "" {
		stored.SecretAccessKey, err = m.encryptor.Encrypt(stored.SecretAccessKey)
		if err != nil {
			return SupportTicketAttachmentStorageConfig{}, fmt.Errorf("encrypt support ticket storage secret: %w", err)
		}
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return SupportTicketAttachmentStorageConfig{}, fmt.Errorf("marshal support ticket storage config: %w", err)
	}
	if err := m.settingRepo.Set(ctx, settingKeySupportTicketAttachmentStorage, string(data)); err != nil {
		return SupportTicketAttachmentStorageConfig{}, fmt.Errorf("save support ticket storage config: %w", err)
	}

	m.mu.Lock()
	m.cfg = next
	m.store = store
	m.mu.Unlock()
	return m.GetConfig(), nil
}

func (m *SupportTicketAttachmentManager) TestConnection(
	ctx context.Context,
	input SupportTicketAttachmentStorageConfig,
) error {
	current := m.CurrentConfig()
	testCfg := supportTicketStorageConfigToRuntime(input)
	if strings.TrimSpace(testCfg.SecretAccessKey) == "" {
		testCfg.SecretAccessKey = current.SecretAccessKey
	}
	testCfg.Enabled = true
	testCfg = normalizeSupportTicketAttachmentConfig(testCfg)
	if err := validateSupportTicketAttachmentConfig(testCfg, true); err != nil {
		return err
	}
	store, err := m.storeFactory(ctx, testCfg)
	if err != nil {
		return err
	}
	return store.HeadBucket(ctx)
}

func (m *SupportTicketAttachmentManager) Upload(
	ctx context.Context,
	key string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	store, err := m.activeStore()
	if err != nil {
		return err
	}
	return store.Upload(ctx, key, body, size, contentType)
}

func (m *SupportTicketAttachmentManager) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	store, err := m.activeStore()
	if err != nil {
		return nil, err
	}
	return store.Download(ctx, key)
}

func (m *SupportTicketAttachmentManager) Delete(ctx context.Context, key string) error {
	store, err := m.activeStore()
	if err != nil {
		return err
	}
	return store.Delete(ctx, key)
}

func (m *SupportTicketAttachmentManager) PresignURL(
	ctx context.Context,
	key string,
	expiry time.Duration,
) (string, error) {
	store, err := m.activeStore()
	if err != nil {
		return "", err
	}
	return store.PresignURL(ctx, key, expiry)
}

func (m *SupportTicketAttachmentManager) activeStore() (SupportTicketAttachmentObjectStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.cfg.Enabled || m.store == nil {
		return nil, ErrSupportTicketAttachmentsDisabled
	}
	return m.store, nil
}

func (m *SupportTicketAttachmentManager) createStore(
	ctx context.Context,
	cfg config.SupportTicketAttachmentConfig,
) (SupportTicketAttachmentObjectStore, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return m.storeFactory(ctx, cfg)
}

func (m *SupportTicketAttachmentManager) loadStoredConfig(
	ctx context.Context,
) (*config.SupportTicketAttachmentConfig, error) {
	raw, err := m.settingRepo.GetValue(ctx, settingKeySupportTicketAttachmentStorage)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var stored SupportTicketAttachmentStorageConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, ErrSupportTicketStorageConfigCorrupt
	}
	if stored.SecretAccessKey != "" {
		decrypted, decryptErr := m.encryptor.Decrypt(stored.SecretAccessKey)
		if decryptErr != nil {
			return nil, fmt.Errorf("%w: decrypt secret: %v", ErrSupportTicketStorageConfigCorrupt, decryptErr)
		}
		stored.SecretAccessKey = decrypted
	}
	cfg := supportTicketStorageConfigToRuntime(stored)
	return &cfg, nil
}

func normalizeSupportTicketAttachmentConfig(
	cfg config.SupportTicketAttachmentConfig,
) config.SupportTicketAttachmentConfig {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(cfg.SecretAccessKey)
	cfg.Prefix = strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "support-tickets"
	}
	if cfg.MaxFileSizeMB <= 0 {
		cfg.MaxFileSizeMB = 10
	}
	if cfg.MaxAttachmentsMessage <= 0 {
		cfg.MaxAttachmentsMessage = 4
	}
	if cfg.URLExpiryMinutes <= 0 {
		cfg.URLExpiryMinutes = 15
	}
	return cfg
}

func validateSupportTicketAttachmentConfig(
	cfg config.SupportTicketAttachmentConfig,
	requireConfigured bool,
) error {
	if !cfg.Enabled && !requireConfigured {
		return nil
	}
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return fmt.Errorf(
			"%w: bucket, access_key_id, and secret_access_key are required",
			ErrSupportTicketStorageConfigInvalid,
		)
	}
	if cfg.Endpoint != "" {
		parsed, err := url.ParseRequestURI(cfg.Endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf(
				"%w: endpoint must be a valid HTTP(S) URL",
				ErrSupportTicketStorageConfigInvalid,
			)
		}
	}
	if cfg.MaxFileSizeMB < 1 || cfg.MaxFileSizeMB > 100 {
		return fmt.Errorf(
			"%w: max_file_size_mb must be between 1 and 100",
			ErrSupportTicketStorageConfigInvalid,
		)
	}
	if cfg.MaxAttachmentsMessage < 1 || cfg.MaxAttachmentsMessage > 10 {
		return fmt.Errorf(
			"%w: max_attachments_per_message must be between 1 and 10",
			ErrSupportTicketStorageConfigInvalid,
		)
	}
	if cfg.URLExpiryMinutes < 1 || cfg.URLExpiryMinutes > 1440 {
		return fmt.Errorf(
			"%w: url_expiry_minutes must be between 1 and 1440",
			ErrSupportTicketStorageConfigInvalid,
		)
	}
	return nil
}

func supportTicketStorageConfigFromRuntime(
	cfg config.SupportTicketAttachmentConfig,
) SupportTicketAttachmentStorageConfig {
	return SupportTicketAttachmentStorageConfig{
		Enabled:                  cfg.Enabled,
		Endpoint:                 cfg.Endpoint,
		Region:                   cfg.Region,
		Bucket:                   cfg.Bucket,
		AccessKeyID:              cfg.AccessKeyID,
		SecretAccessKey:          cfg.SecretAccessKey,
		Prefix:                   cfg.Prefix,
		ForcePathStyle:           cfg.ForcePathStyle,
		MaxFileSizeMB:            cfg.MaxFileSizeMB,
		MaxAttachmentsPerMessage: cfg.MaxAttachmentsMessage,
		URLExpiryMinutes:         cfg.URLExpiryMinutes,
	}
}

func supportTicketStorageConfigToRuntime(
	cfg SupportTicketAttachmentStorageConfig,
) config.SupportTicketAttachmentConfig {
	return config.SupportTicketAttachmentConfig{
		Enabled:               cfg.Enabled,
		Endpoint:              cfg.Endpoint,
		Region:                cfg.Region,
		Bucket:                cfg.Bucket,
		AccessKeyID:           cfg.AccessKeyID,
		SecretAccessKey:       cfg.SecretAccessKey,
		Prefix:                cfg.Prefix,
		ForcePathStyle:        cfg.ForcePathStyle,
		MaxFileSizeMB:         cfg.MaxFileSizeMB,
		MaxAttachmentsMessage: cfg.MaxAttachmentsPerMessage,
		URLExpiryMinutes:      cfg.URLExpiryMinutes,
	}
}
