package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/google/uuid"
)

const (
	SettingKeyNotificationWebhook = "notification_webhook"
	WebhookEventRechargeCompleted = "payment.recharge.completed"
	WebhookMaxAttempts            = 5
)

type NotificationWebhookSettings struct {
	Enabled                 bool   `json:"enabled"`
	URL                     string `json:"webhook_url"`
	HasSecret               bool   `json:"has_secret"`
	Version                 string `json:"version"`
	EncryptionKeyConfigured bool   `json:"encryption_key_configured"`
}

type UpdateNotificationWebhookSettings struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"webhook_url"`
	Secret      string `json:"secret"`
	ClearSecret bool   `json:"clear_secret"`
	Version     string `json:"version"`
}

type storedNotificationWebhook struct {
	Enabled     bool   `json:"enabled"`
	Version     string `json:"version"`
	Destination string `json:"destination"` // Encrypted URL and signing secret.
}

type notificationWebhookDestination struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type NotificationWebhookDelivery struct {
	ID             int64           `json:"id"`
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	OrderID        int64           `json:"order_id"`
	ConfigVersion  string          `json:"-"`
	Payload        json.RawMessage `json:"-"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	LastStatusCode int             `json:"last_status_code"`
	LastError      string          `json:"last_error"`
	CreatedAt      time.Time       `json:"created_at"`
	FinishedAt     *time.Time      `json:"finished_at"`
}

type NotificationWebhookRepository interface {
	SaveConfig(context.Context, string, string) (bool, error)
	Claim(context.Context, string) (*NotificationWebhookDelivery, error)
	Finish(context.Context, int64, string, string, int, string, time.Time) error
	ListRecent(context.Context) ([]NotificationWebhookDelivery, error)
	Cleanup(context.Context) error
}

type NotificationWebhookService struct {
	settings                SettingRepository
	repo                    NotificationWebhookRepository
	encryptor               SecretEncryptor
	client                  *http.Client
	ctx                     context.Context
	cancel                  context.CancelFunc
	startOnce               sync.Once
	stopOnce                sync.Once
	wg                      sync.WaitGroup
	encryptionKeyConfigured bool
}

func NewNotificationWebhookService(settings SettingRepository, repo NotificationWebhookRepository, encryptor SecretEncryptor, cfg *config.Config) *NotificationWebhookService {
	ctx, cancel := context.WithCancel(context.Background())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext
	transport.MaxConnsPerHost = 4
	transport.MaxIdleConns = 8
	transport.MaxIdleConnsPerHost = 2
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.TLSHandshakeTimeout = 5 * time.Second
	return &NotificationWebhookService{
		settings: settings, repo: repo, encryptor: encryptor, ctx: ctx, cancel: cancel,
		encryptionKeyConfigured: cfg != nil && cfg.Totp.EncryptionKeyConfigured,
		client: &http.Client{
			Transport: transport, Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *NotificationWebhookService) readConfig(ctx context.Context) (storedNotificationWebhook, notificationWebhookDestination, error) {
	var config storedNotificationWebhook
	var destination notificationWebhookDestination
	if s == nil || s.settings == nil || s.encryptor == nil {
		return config, destination, ErrServiceUnavailable
	}
	raw, err := s.settings.GetValue(ctx, SettingKeyNotificationWebhook)
	if errors.Is(err, ErrSettingNotFound) {
		return config, destination, nil
	}
	if err != nil {
		return config, destination, err
	}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return config, destination, errors.New("invalid webhook settings")
	}
	if config.Destination != "" {
		plain, err := s.encryptor.Decrypt(config.Destination)
		if err != nil {
			return config, destination, errors.New("cannot decrypt webhook settings")
		}
		if err := json.Unmarshal([]byte(plain), &destination); err != nil {
			return config, destination, errors.New("invalid webhook destination")
		}
	}
	return config, destination, nil
}

func (s *NotificationWebhookService) settingsResponse(config storedNotificationWebhook, destination notificationWebhookDestination) *NotificationWebhookSettings {
	return &NotificationWebhookSettings{
		Enabled: config.Enabled, URL: destination.URL, HasSecret: destination.Secret != "", Version: config.Version,
		EncryptionKeyConfigured: s.encryptionKeyConfigured,
	}
}

func (s *NotificationWebhookService) GetSettings(ctx context.Context) (*NotificationWebhookSettings, error) {
	config, destination, err := s.readConfig(ctx)
	if err != nil {
		return nil, err
	}
	return s.settingsResponse(config, destination), nil
}

func validateNotificationWebhookURL(raw string) error {
	// Reuse the URL policy without its path normalization: signed webhook URLs
	// may depend on an encoded path, query string, or trailing slash.
	if len(raw) > 4096 {
		return infraerrors.BadRequest("WEBHOOK_URL_INVALID", "webhook URL is too long")
	}
	if _, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{}); err != nil {
		return infraerrors.BadRequest("WEBHOOK_URL_INVALID", "webhook requires a public HTTPS URL")
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || isBlockedHostname(u.Hostname()) {
		return infraerrors.BadRequest("WEBHOOK_URL_INVALID", "webhook URL must not contain credentials, fragments, or a private host")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (isPrivateIP(ip) || !ip.IsGlobalUnicast()) {
		return infraerrors.BadRequest("WEBHOOK_URL_INVALID", "webhook requires a public HTTPS URL")
	}
	return nil
}

func (s *NotificationWebhookService) UpdateSettings(ctx context.Context, input UpdateNotificationWebhookSettings) (*NotificationWebhookSettings, error) {
	config, previous, err := s.readConfig(ctx)
	if err != nil {
		return nil, err
	}
	if input.Version != config.Version {
		return nil, infraerrors.Conflict("WEBHOOK_VERSION_CONFLICT", "webhook settings changed; refresh before saving")
	}
	if !s.encryptionKeyConfigured {
		return nil, infraerrors.BadRequest("SECRET_ENCRYPTION_KEY_NOT_CONFIGURED", "configure a fixed TOTP_ENCRYPTION_KEY before saving webhook settings")
	}
	destination := notificationWebhookDestination{URL: strings.TrimSpace(input.URL), Secret: previous.Secret}
	if input.ClearSecret && input.Secret != "" {
		return nil, infraerrors.BadRequest("WEBHOOK_SECRET_INVALID", "cannot replace and clear the signing secret together")
	}
	if len(input.Secret) > 512 {
		return nil, infraerrors.BadRequest("WEBHOOK_SECRET_INVALID", "signing secret is too long")
	}
	if input.ClearSecret {
		destination.Secret = ""
	} else if input.Secret != "" {
		destination.Secret = input.Secret
	}
	if input.Enabled || destination.URL != "" {
		if err := validateNotificationWebhookURL(destination.URL); err != nil {
			return nil, err
		}
	}
	if config.Version != "" && config.Enabled == input.Enabled && previous == destination {
		return s.settingsResponse(config, previous), nil
	}
	plain, err := json.Marshal(destination)
	if err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(string(plain))
	if err != nil {
		return nil, errors.New("cannot encrypt webhook settings")
	}
	config = storedNotificationWebhook{Enabled: input.Enabled, Version: uuid.NewString(), Destination: encrypted}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	saved, err := s.repo.SaveConfig(ctx, input.Version, string(raw))
	if err != nil {
		return nil, err
	}
	if !saved {
		return nil, infraerrors.Conflict("WEBHOOK_VERSION_CONFLICT", "webhook settings changed; refresh before saving")
	}
	return s.settingsResponse(config, destination), nil
}

func (s *NotificationWebhookService) ListRecent(ctx context.Context) ([]NotificationWebhookDelivery, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListRecent(ctx)
}
