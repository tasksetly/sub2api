package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const SettingKeyCCTest = "cctest"

type CCTestSettings struct {
	Enabled                 bool   `json:"enabled"`
	AllowUsers              bool   `json:"allow_users"`
	PublicURL               string `json:"public_url"`
	HasAPIKey               bool   `json:"has_api_key"`
	EncryptionKeyConfigured bool   `json:"encryption_key_configured"`
	Version                 string `json:"version"`
}

type CCTestSettingsInput struct {
	Enabled    bool   `json:"enabled"`
	AllowUsers bool   `json:"allow_users"`
	PublicURL  string `json:"public_url"`
	APIKey     string `json:"api_key"`
	Version    string `json:"version"`
}

type cctestStoredSettings struct {
	Enabled    bool   `json:"enabled"`
	AllowUsers bool   `json:"allow_users"`
	PublicURL  string `json:"public_url"`
	Secret     string `json:"secret"`
	Version    string `json:"version"`
}

type CCTestRun struct {
	ID           string          `json:"id"`
	OwnerID      int64           `json:"-"`
	AccountID    *int64          `json:"account_id,omitempty"`
	APIKeyID     *int64          `json:"api_key_id,omitempty"`
	TargetName   string          `json:"target_name"`
	Model        string          `json:"model"`
	TokenAudit   bool            `json:"token_audit"`
	Status       string          `json:"status"`
	TaskID       string          `json:"-"`
	RelayHash    string          `json:"-"`
	Credentials  string          `json:"-"`
	RelayURL     string          `json:"-"`
	RequestCount int             `json:"request_count"`
	Result       json.RawMessage `json:"result"`
	Error        string          `json:"error"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	LeaseID      string          `json:"-"`
}

type CCTestInput struct {
	AccountIDs []int64 `json:"account_ids"`
	APIKeyID   int64   `json:"api_key_id"`
	Model      string  `json:"model"`
	TokenAudit bool    `json:"token_audit"`
}

type CCTestRepository interface {
	SaveSettings(context.Context, string, string) (bool, error)
	Create(context.Context, []*CCTestRun, bool) error
	List(context.Context, int64, bool, string) ([]*CCTestRun, error)
	Cancel(context.Context, string, int64, bool) error
	Claim(context.Context) (*CCTestRun, error)
	Update(context.Context, *CCTestRun) error
	ConsumeRelay(context.Context, string, string) (*CCTestRun, error)
}

type CCTestService struct {
	repo                    CCTestRepository
	settings                SettingRepository
	encryptor               SecretEncryptor
	accounts                AccountRepository
	keys                    APIKeyRepository
	client                  *http.Client
	encryptionKeyConfigured bool
	ctx                     context.Context
	cancel                  context.CancelFunc
	wg                      sync.WaitGroup
	startOnce               sync.Once
}

func NewCCTestService(repo CCTestRepository, settings SettingRepository, encryptor SecretEncryptor, accounts AccountRepository, keys APIKeyRepository, cfg *config.Config) *CCTestService {
	ctx, cancel := context.WithCancel(context.Background())
	return &CCTestService{repo: repo, settings: settings, encryptor: encryptor, accounts: accounts, keys: keys,
		ctx: ctx, cancel: cancel, encryptionKeyConfigured: cfg != nil && cfg.Totp.EncryptionKeyConfigured,
		client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (s *CCTestService) readSettings(ctx context.Context) (cctestStoredSettings, error) {
	var cfg cctestStoredSettings
	raw, err := s.settings.GetValue(ctx, SettingKeyCCTest)
	if errors.Is(err, ErrSettingNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal([]byte(raw), &cfg)
	return cfg, err
}

func (s *CCTestService) GetSettings(ctx context.Context) (*CCTestSettings, error) {
	cfg, err := s.readSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &CCTestSettings{Enabled: cfg.Enabled, AllowUsers: cfg.AllowUsers, PublicURL: cfg.PublicURL,
		HasAPIKey: cfg.Secret != "", Version: cfg.Version, EncryptionKeyConfigured: s.encryptionKeyConfigured}, nil
}

func (s *CCTestService) SaveSettings(ctx context.Context, input CCTestSettingsInput) (*CCTestSettings, error) {
	cfg, err := s.readSettings(ctx)
	if err != nil {
		return nil, err
	}
	if input.Version != cfg.Version {
		return nil, infraerrors.Conflict("CCTEST_CONFIG_CHANGED", "CCTest settings changed; refresh before saving")
	}
	if !s.encryptionKeyConfigured {
		return nil, infraerrors.BadRequest("CCTEST_ENCRYPTION_REQUIRED", "configure a fixed TOTP_ENCRYPTION_KEY before saving CCTest settings")
	}
	publicURL := strings.TrimRight(strings.TrimSpace(input.PublicURL), "/")
	if publicURL != "" || input.Enabled {
		if err := validateNotificationWebhookURL(publicURL); err != nil {
			return nil, infraerrors.BadRequest("CCTEST_URL_INVALID", "CCTest requires this site's public HTTPS base URL")
		}
		u, err := url.Parse(publicURL)
		if err != nil || u.RawQuery != "" || u.Path != "" && u.Path != "/" {
			return nil, infraerrors.BadRequest("CCTEST_URL_INVALID", "public URL must be a site-root URL without a path or query")
		}
	}
	key := strings.TrimSpace(input.APIKey)
	if key != "" {
		if !strings.HasPrefix(key, "cct-") || len(key) > 512 || strings.ContainsAny(key, "\r\n\t ") {
			return nil, infraerrors.BadRequest("CCTEST_KEY_INVALID", "a CCTest API key starting with cct- is required")
		}
		cfg.Secret, err = s.encryptor.Encrypt(key)
		if err != nil {
			return nil, errors.New("cannot encrypt CCTest API key")
		}
	}
	if input.Enabled && cfg.Secret == "" {
		return nil, infraerrors.BadRequest("CCTEST_KEY_REQUIRED", "configure a CCTest API key first")
	}
	cfg.Enabled, cfg.AllowUsers, cfg.PublicURL, cfg.Version = input.Enabled, input.AllowUsers, publicURL, uuid.NewString()
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.SaveSettings(ctx, input.Version, string(raw))
	if err != nil {
		return nil, err
	}
	if !saved {
		return nil, infraerrors.Conflict("CCTEST_CONFIG_CHANGED", "CCTest settings changed; refresh before saving")
	}
	return s.GetSettings(ctx)
}

func (s *CCTestService) userKey(ctx context.Context, id, ownerID int64) (*APIKey, error) {
	key, err := s.keys.GetByID(ctx, id)
	if err != nil {
		return nil, infraerrors.BadRequest("CCTEST_KEY_INVALID", "API key is unavailable")
	}
	if key == nil || key.UserID != ownerID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.GroupID == nil || key.Group == nil || !key.Group.IsActive() {
		return nil, infraerrors.BadRequest("CCTEST_KEY_INVALID", "select your own active API key bound to an active group")
	}
	return key, nil
}

func (s *CCTestService) Create(ctx context.Context, ownerID int64, admin bool, input CCTestInput) ([]*CCTestRun, error) {
	cfg, err := s.readSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled || cfg.Secret == "" || (!admin && !cfg.AllowUsers) {
		return nil, infraerrors.Forbidden("CCTEST_DISABLED", "CCTest is not enabled")
	}
	input.Model = strings.TrimSpace(input.Model)
	if len(input.Model) > 200 || !strings.Contains(strings.ToLower(input.Model), "claude") {
		return nil, infraerrors.BadRequest("CCTEST_MODEL_INVALID", "enter a Claude model ID (up to 200 characters)")
	}
	if (!admin && (len(input.AccountIDs) != 0 || input.APIKeyID <= 0)) || (admin && (len(input.AccountIDs) < 1 || len(input.AccountIDs) > 20 || input.APIKeyID != 0)) {
		return nil, infraerrors.BadRequest("CCTEST_TARGET_INVALID", "select up to 20 Claude accounts, or your own API key")
	}
	serviceKey, err := s.encryptor.Decrypt(cfg.Secret)
	if err != nil {
		return nil, errors.New("cannot decrypt CCTest API key")
	}
	runs := make([]*CCTestRun, 0, len(input.AccountIDs)+1)
	seen := make(map[int64]bool)
	for _, id := range input.AccountIDs {
		if id <= 0 || seen[id] {
			return nil, infraerrors.BadRequest("CCTEST_TARGET_INVALID", "account IDs must be positive and unique")
		}
		seen[id] = true
		account, err := s.accounts.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if account == nil || account.Platform != PlatformAnthropic {
			return nil, infraerrors.BadRequest("CCTEST_TARGET_INVALID", fmt.Sprintf("account %d is not a Claude account", id))
		}
		if !account.IsModelSupported(input.Model) {
			return nil, infraerrors.BadRequest("CCTEST_MODEL_UNSUPPORTED", fmt.Sprintf("account %d does not support the selected model", id))
		}
		accountID := id
		runs = append(runs, &CCTestRun{AccountID: &accountID, TargetName: account.Name})
	}
	if !admin {
		key, err := s.userKey(ctx, input.APIKeyID, ownerID)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &CCTestRun{APIKeyID: &key.ID, TargetName: key.Name})
	}
	for _, run := range runs {
		random := make([]byte, 32)
		if _, err := rand.Read(random); err != nil {
			return nil, err
		}
		token := "cctrelay-" + hex.EncodeToString(random)
		digest := sha256.Sum256([]byte(token))
		secret, err := json.Marshal(cctestCredentials{APIKey: serviceKey, RelayToken: token})
		if err != nil {
			return nil, err
		}
		run.Credentials, err = s.encryptor.Encrypt(string(secret))
		if err != nil {
			return nil, errors.New("cannot encrypt CCTest task credentials")
		}
		run.ID, run.OwnerID, run.Model, run.TokenAudit = uuid.NewString(), ownerID, input.Model, input.TokenAudit
		run.Status, run.RelayHash, run.RelayURL = "queued", hex.EncodeToString(digest[:]), cfg.PublicURL+"/cctest/relay"
		run.Result = json.RawMessage(`{}`)
	}
	if err := s.repo.Create(ctx, runs, admin); err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *CCTestService) List(ctx context.Context, ownerID int64, admin bool, before string) ([]*CCTestRun, error) {
	if before != "" {
		if _, err := uuid.Parse(before); err != nil {
			return nil, infraerrors.BadRequest("CCTEST_CURSOR_INVALID", "invalid history cursor")
		}
	}
	return s.repo.List(ctx, ownerID, admin, before)
}

func (s *CCTestService) Cancel(ctx context.Context, id string, ownerID int64, admin bool) error {
	if _, err := uuid.Parse(id); err != nil {
		return infraerrors.BadRequest("CCTEST_ID_INVALID", "invalid test ID")
	}
	return s.repo.Cancel(ctx, id, ownerID, admin)
}

// ResolveRelay consumes a bounded, model-scoped capability. Real keys never leave this server.
func (s *CCTestService) ResolveRelay(ctx context.Context, token, model string) (*CCTestRun, *Account, *APIKey, error) {
	if !strings.HasPrefix(token, "cctrelay-") || len(token) != 73 {
		return nil, nil, nil, infraerrors.Unauthorized("CCTEST_RELAY_INVALID", "invalid test credential")
	}
	cfg, err := s.readSettings(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if !cfg.Enabled {
		return nil, nil, nil, infraerrors.Forbidden("CCTEST_DISABLED", "CCTest is disabled")
	}
	digest := sha256.Sum256([]byte(token))
	run, err := s.repo.ConsumeRelay(ctx, hex.EncodeToString(digest[:]), model)
	if err != nil {
		return nil, nil, nil, err
	}
	if run.AccountID != nil {
		account, err := s.accounts.GetByID(ctx, *run.AccountID)
		if err != nil {
			return nil, nil, nil, err
		}
		if account == nil || account.Platform != PlatformAnthropic {
			return nil, nil, nil, infraerrors.Forbidden("CCTEST_TARGET_INVALID", "test account is no longer a Claude account")
		}
		return run, account, nil, nil
	}
	if !cfg.AllowUsers {
		return nil, nil, nil, infraerrors.Forbidden("CCTEST_DISABLED", "user tests are disabled")
	}
	key, err := s.userKey(ctx, *run.APIKeyID, run.OwnerID)
	return run, nil, key, err
}

type cctestCredentials struct {
	APIKey     string `json:"api_key"`
	RelayToken string `json:"relay_token"`
}

func (s *CCTestService) Start() {
	s.startOnce.Do(func() {
		for i := 0; i < 2; i++ {
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-s.ctx.Done():
						return
					case <-ticker.C:
					}
					ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
					run, err := s.repo.Claim(ctx)
					if err != nil {
						slog.Error("cctest_claim_failed", "error", err)
					} else if run != nil {
						s.advance(ctx, run)
					}
					cancel()
				}
			}()
		}
	})
}

func (s *CCTestService) Stop() { s.cancel(); s.wg.Wait(); s.client.CloseIdleConnections() }

func (s *CCTestService) api(ctx context.Context, method, path, key string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://cctest.ai/api/v1/check"+path, bytes.NewReader(payload))
	if err != nil {
		return errors.New("cannot create CCTest request")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return errors.New("CCTest request failed or timed out")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CCTest HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return errors.New("CCTest response is incomplete or too large")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("invalid CCTest response")
	}
	return nil
}

var cctestTaskIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

func (s *CCTestService) advance(ctx context.Context, run *CCTestRun) {
	cfg, err := s.readSettings(ctx)
	if err != nil {
		return
	}
	if !cfg.Enabled || (run.APIKeyID != nil && !cfg.AllowUsers) {
		run.Status, run.Error = "cancelled", "CCTest disabled"
	} else {
		plain, err := s.encryptor.Decrypt(run.Credentials)
		var credentials cctestCredentials
		if err != nil || json.Unmarshal([]byte(plain), &credentials) != nil {
			run.Status, run.Error = "error", "cannot decrypt test credentials"
		} else if run.Status == "starting" {
			var reply struct {
				TaskID string `json:"taskId"`
			}
			err := s.api(ctx, http.MethodPost, "", credentials.APIKey, map[string]any{
				"url": run.RelayURL, "apiKey": credentials.RelayToken, "model": run.Model, "provider": "claude", "checkTokenUsage": run.TokenAudit, "concurrency": 1,
			}, &reply)
			if err != nil || !cctestTaskIDPattern.MatchString(reply.TaskID) {
				run.Status, run.Error = "error", "CCTest submission failed; it may have consumed a credit. No automatic resubmission."
				if err != nil {
					run.Error += " " + err.Error()
				}
			} else {
				run.Status, run.TaskID = "running", reply.TaskID
			}
		} else {
			var result map[string]json.RawMessage
			err := s.api(ctx, http.MethodGet, "/"+url.PathEscape(run.TaskID), credentials.APIKey, nil, &result)
			if err == nil {
				var status string
				_ = json.Unmarshal(result["status"], &status)
				if status == "done" || status == "error" {
					run.Status = status
				}
				// Decode into a closed report schema: nested fields must not retain
				// arbitrary request bodies or credentials returned by the provider.
				raw, marshalErr := json.Marshal(result)
				var report cctestReport
				if marshalErr != nil || json.Unmarshal(raw, &report) != nil {
					run.Status, run.Error = "error", "CCTest returned an invalid report"
				} else {
					if status == "done" && (report.Total == nil || *report.Total < 0 || *report.Total > 100) {
						run.Status, run.Error = "error", "CCTest returned an invalid total score"
					}
					if status == "error" {
						run.Error = "CCTest could not complete the check"
					}
					run.Result, _ = json.Marshal(report)
					for _, secret := range []string{credentials.APIKey, credentials.RelayToken} {
						if secret != "" {
							run.Result = bytes.ReplaceAll(run.Result, []byte(secret), []byte("[redacted]"))
						}
					}
				}
			}
		}
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.Update(writeCtx, run); err != nil {
		slog.Error("cctest_save_failed", "run_id", run.ID, "error", err)
	}
}

// Keep only documented, typed report data; never persist raw provider payloads.
type cctestReport struct {
	Status           string             `json:"status"`
	Progress         *float64           `json:"progress,omitempty"`
	Step             *int               `json:"step,omitempty"`
	StepName         string             `json:"stepName,omitempty"`
	ExpectedModel    string             `json:"expectedModel,omitempty"`
	ResponseModel    string             `json:"responseModel,omitempty"`
	Scores           map[string]float64 `json:"scores,omitempty"`
	VerdictKey       string             `json:"verdictKey,omitempty"`
	Total            *float64           `json:"total,omitempty"`
	StreamChannel    *string            `json:"streamChannel,omitempty"`
	NonStreamChannel *string            `json:"nonStreamChannel,omitempty"`
	HasVertex        bool               `json:"hasVertex,omitempty"`
	Metrics          *struct {
		LatencyMs    *float64 `json:"latencyMs,omitempty"`
		TtfbMs       *float64 `json:"ttfbMs,omitempty"`
		TokensPerSec *float64 `json:"tokensPerSec,omitempty"`
		InputTokens  *int64   `json:"inputTokens,omitempty"`
		OutputTokens *int64   `json:"outputTokens,omitempty"`
	} `json:"metrics,omitempty"`
	TokenAudit *struct {
		TotalCost         *float64 `json:"totalCost,omitempty"`
		BaselineTotalCost *float64 `json:"baselineTotalCost,omitempty"`
		OverallRatio      *float64 `json:"overallRatio,omitempty"`
		CacheHitRate      *float64 `json:"cacheHitRate,omitempty"`
	} `json:"tokenAudit,omitempty"`
}
