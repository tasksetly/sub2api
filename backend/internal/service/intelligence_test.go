package service

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/robfig/cron/v3"
)

var (
	ErrIntelligenceTestConfigNotFound = infraerrors.NotFound("INTELLIGENCE_TEST_CONFIG_NOT_FOUND", "intelligence test config not found")
	ErrIntelligenceTestConfigInvalid  = infraerrors.BadRequest("INTELLIGENCE_TEST_CONFIG_INVALID", "name, model, prompt, and API key are required")
)

type IntelligenceTestConfig struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	ModelID        string     `json:"model_id"`
	PromptText     string     `json:"prompt_text"`
	APIKeyID       int64      `json:"api_key_id"`
	APIKeyName     string     `json:"api_key_name"`
	GroupID        *int64     `json:"group_id"`
	GroupName      string     `json:"group_name"`
	CronExpression string     `json:"cron_expression"`
	KeepCount      int        `json:"keep_count"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	NextRunAt      *time.Time `json:"next_run_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type IntelligenceTestResult struct {
	ID           int64      `json:"id"`
	ConfigID     int64      `json:"config_id"`
	APIKeyID     int64      `json:"api_key_id"`
	GroupID      *int64     `json:"group_id,omitempty"`
	GroupName    string     `json:"group_name"`
	ModelID      string     `json:"model_id"`
	Status       string     `json:"status"`
	Check        bool       `json:"check"`
	SVGContent   string     `json:"svg_content,omitempty"`
	ResponseText string     `json:"response_text,omitempty"`
	ErrorMessage string     `json:"error,omitempty"`
	LatencyMs    int64      `json:"latency_ms"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   time.Time  `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type IntelligenceTestAPIKeyOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"`
	Status    string `json:"status"`
	GroupID   *int64 `json:"group_id"`
	GroupName string `json:"group_name"`
}

type IntelligenceTestRepository interface {
	ListConfigs(context.Context) ([]*IntelligenceTestConfig, error)
	GetConfig(context.Context, int64) (*IntelligenceTestConfig, error)
	CreateConfig(context.Context, *IntelligenceTestConfig) (*IntelligenceTestConfig, error)
	UpdateConfig(context.Context, *IntelligenceTestConfig) (*IntelligenceTestConfig, error)
	DeleteConfig(context.Context, int64) error
	ListDueConfigs(context.Context, time.Time) ([]*IntelligenceTestConfig, error)
	UpdateRun(context.Context, int64, time.Time, time.Time) error
	ListAPIKeyOptions(context.Context) ([]*IntelligenceTestAPIKeyOption, error)
	InsertResult(context.Context, *IntelligenceTestResult) error
	PruneResults(context.Context, int64, int) error
	ListResults(context.Context, int64, int) ([]*IntelligenceTestResult, error)
}

type IntelligenceTestConfigInput struct {
	Name           string `json:"name"`
	ModelID        string `json:"model_id"`
	PromptText     string `json:"prompt_text"`
	APIKeyID       int64  `json:"api_key_id"`
	CronExpression string `json:"cron_expression"`
	KeepCount      int    `json:"keep_count"`
	Enabled        *bool  `json:"enabled"`
}

type IntelligenceTestPublicConfig struct {
	Title      string     `json:"title"`
	Description string    `json:"description"`
	PromptName string     `json:"prompt_name"`
	PromptText string     `json:"prompt_text"`
	ModelID    string     `json:"model_id"`
	KeepCount  int        `json:"keep_count"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type IntelligenceTestPublicResult struct {
	ID         int64     `json:"id"`
	GroupName  string    `json:"group_name"`
	ModelID    string    `json:"model_id"`
	Status     string    `json:"status"`
	Check      bool      `json:"check"`
	SVGContent string    `json:"svg_content,omitempty"`
	LatencyMs  int64     `json:"latency_ms"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type IntelligenceTestPublicResponse struct {
	Config    *IntelligenceTestPublicConfig `json:"config,omitempty"`
	KeepCount int                           `json:"keep_count"`
	Items     []*IntelligenceTestPublicResult `json:"items"`
	Groups    []string                      `json:"groups"`
	Updated   *time.Time                    `json:"updated,omitempty"`
}

type IntelligenceTestService struct {
	repo          IntelligenceTestRepository
	apiKeyRepo    APIKeyRepository
	accountRepo   AccountRepository
	accountTest   *AccountTestService
	cron          *cron.Cron
	startOnce     sync.Once
	stopOnce      sync.Once
}

func NewIntelligenceTestService(repo IntelligenceTestRepository, apiKeyRepo APIKeyRepository, accountRepo AccountRepository, accountTest *AccountTestService) *IntelligenceTestService {
	return &IntelligenceTestService{repo: repo, apiKeyRepo: apiKeyRepo, accountRepo: accountRepo, accountTest: accountTest}
}

func (s *IntelligenceTestService) Start() {
	if s == nil { return }
	s.startOnce.Do(func() {
		s.cron = cron.New(cron.WithParser(scheduledTestCronParser))
		_, _ = s.cron.AddFunc("* * * * *", func() { s.runDue() })
		s.cron.Start()
	})
}

func (s *IntelligenceTestService) Stop() {
	if s == nil { return }
	s.stopOnce.Do(func() {
		if s.cron != nil { <-s.cron.Stop().Done() }
	})
}

func (s *IntelligenceTestService) ListConfigs(ctx context.Context) ([]*IntelligenceTestConfig, error) { return s.repo.ListConfigs(ctx) }
func (s *IntelligenceTestService) GetConfig(ctx context.Context, id int64) (*IntelligenceTestConfig, error) { return s.repo.GetConfig(ctx, id) }
func (s *IntelligenceTestService) ListAPIKeyOptions(ctx context.Context) ([]*IntelligenceTestAPIKeyOption, error) { return s.repo.ListAPIKeyOptions(ctx) }
func (s *IntelligenceTestService) ListResults(ctx context.Context, id int64, limit int) ([]*IntelligenceTestResult, error) { return s.repo.ListResults(ctx, id, limit) }

func (s *IntelligenceTestService) CreateConfig(ctx context.Context, input IntelligenceTestConfigInput) (*IntelligenceTestConfig, error) {
	c, err := s.normalizeConfig(input)
	if err != nil { return nil, err }
	return s.repo.CreateConfig(ctx, c)
}

func (s *IntelligenceTestService) UpdateConfig(ctx context.Context, id int64, input IntelligenceTestConfigInput) (*IntelligenceTestConfig, error) {
	c, err := s.repo.GetConfig(ctx, id)
	if err != nil { return nil, err }
	next, err := s.normalizeConfig(input)
	if err != nil { return nil, err }
	c.Name, c.ModelID, c.PromptText, c.APIKeyID = next.Name, next.ModelID, next.PromptText, next.APIKeyID
	c.CronExpression, c.KeepCount, c.Enabled = next.CronExpression, next.KeepCount, next.Enabled
	return s.repo.UpdateConfig(ctx, c)
}

func (s *IntelligenceTestService) DeleteConfig(ctx context.Context, id int64) error { return s.repo.DeleteConfig(ctx, id) }

func (s *IntelligenceTestService) normalizeConfig(input IntelligenceTestConfigInput) (*IntelligenceTestConfig, error) {
	name, model, prompt := strings.TrimSpace(input.Name), strings.TrimSpace(input.ModelID), strings.TrimSpace(input.PromptText)
	if name == "" || model == "" || prompt == "" || input.APIKeyID <= 0 { return nil, ErrIntelligenceTestConfigInvalid }
	cronExpression := strings.TrimSpace(input.CronExpression)
	if cronExpression == "" { cronExpression = "0 * * * *" }
	if _, err := scheduledTestCronParser.Parse(cronExpression); err != nil { return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_CRON_INVALID", "invalid cron expression") }
	keepCount := input.KeepCount
	if keepCount <= 0 { keepCount = 20 }
	if keepCount > 100 { keepCount = 100 }
	enabled := true
	if input.Enabled != nil { enabled = *input.Enabled }
	next := time.Now()
	if schedule, err := scheduledTestCronParser.Parse(cronExpression); err == nil { next = schedule.Next(next) }
	return &IntelligenceTestConfig{Name: name, ModelID: model, PromptText: prompt, APIKeyID: input.APIKeyID, CronExpression: cronExpression, KeepCount: keepCount, Enabled: enabled, NextRunAt: &next}, nil
}

func (s *IntelligenceTestService) RunNow(ctx context.Context, id int64) (*IntelligenceTestResult, error) {
	c, err := s.repo.GetConfig(ctx, id)
	if err != nil { return nil, err }
	result := s.runConfig(ctx, c)
	now := time.Now()
	next := now
	if schedule, parseErr := scheduledTestCronParser.Parse(c.CronExpression); parseErr == nil { next = schedule.Next(now) }
	if updateErr := s.repo.UpdateRun(ctx, c.ID, now, next); updateErr != nil && err == nil { err = updateErr }
	if err != nil { return result, err }
	return result, nil
}

func (s *IntelligenceTestService) runDue() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	configs, err := s.repo.ListDueConfigs(ctx, time.Now())
	if err != nil { return }
	for _, c := range configs { _, _ = s.RunNow(ctx, c.ID) }
}

func (s *IntelligenceTestService) runConfig(ctx context.Context, c *IntelligenceTestConfig) *IntelligenceTestResult {
	started := time.Now()
	result := &IntelligenceTestResult{ConfigID: c.ID, APIKeyID: c.APIKeyID, ModelID: c.ModelID, Status: "failed", StartedAt: started, CreatedAt: started}
	key, err := s.apiKeyRepo.GetByID(ctx, c.APIKeyID)
	if err == nil && key.GroupID != nil {
		result.GroupID = key.GroupID
		if key.Group != nil { result.GroupName = key.Group.Name }
		accounts, accountErr := s.accountRepo.ListSchedulableByGroupID(ctx, *key.GroupID)
		if accountErr == nil && len(accounts) > 0 {
			run, runErr := s.accountTest.RunTestBackgroundWithPrompt(ctx, accounts[0].ID, c.ModelID, c.PromptText)
			if runErr == nil && run != nil {
				result.Status, result.ResponseText, result.ErrorMessage, result.LatencyMs = run.Status, run.ResponseText, run.ErrorMessage, run.LatencyMs
				result.StartedAt, result.FinishedAt = run.StartedAt, run.FinishedAt
				result.SVGContent = extractSVG(run.ResponseText)
				result.Check = result.Status == "success" && result.SVGContent != ""
				if result.Check { result.Status = "validated" }
			} else if runErr != nil { result.ErrorMessage = runErr.Error() }
		} else if accountErr != nil { result.ErrorMessage = accountErr.Error() } else { result.ErrorMessage = "no schedulable account in the API key group" }
	} else if err != nil { result.ErrorMessage = err.Error() } else { result.ErrorMessage = "API key is not bound to a group" }
	if result.FinishedAt.IsZero() { result.FinishedAt = time.Now(); result.LatencyMs = result.FinishedAt.Sub(started).Milliseconds() }
	if err := s.repo.InsertResult(ctx, result); err == nil {
		// Keep the public benchmark bounded so a long-running schedule does not
		// grow the SVG response table without limit.
		_ = s.repo.PruneResults(ctx, result.ConfigID, c.KeepCount)
	}
	return result
}

func extractSVG(value string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(value, "```svg", ""))
	clean = strings.ReplaceAll(clean, "```", "")
	start := regexp.MustCompile(`(?is)<svg\b`).FindStringIndex(clean)
	if start == nil { return "" }
	end := strings.Index(strings.ToLower(clean[start[0]:]), "</svg>")
	if end < 0 { return "" }
	return strings.TrimSpace(clean[start[0] : start[0]+end+len("</svg>")])
}

func (s *IntelligenceTestService) Public(ctx context.Context, configID int64) (*IntelligenceTestPublicResponse, error) {
	response := &IntelligenceTestPublicResponse{Items: make([]*IntelligenceTestPublicResult, 0), Groups: make([]string, 0)}
	configs := make([]*IntelligenceTestConfig, 0)
	if configID > 0 {
		c, err := s.repo.GetConfig(ctx, configID)
		if err != nil {
			if errors.Is(err, ErrIntelligenceTestConfigNotFound) { return response, nil }
			return nil, err
		}
		if c.Enabled { configs = append(configs, c) }
	} else {
		all, err := s.repo.ListConfigs(ctx)
		if err != nil { return nil, err }
		for _, c := range all {
			if c.Enabled { configs = append(configs, c) }
		}
	}
	if len(configs) == 0 { return response, nil }

	first := configs[0]
	keepCount := first.KeepCount
	response.Config = &IntelligenceTestPublicConfig{Title: first.Name, Description: "API Key 分组 SVG 生成能力测试", PromptName: first.Name, PromptText: first.PromptText, ModelID: first.ModelID, KeepCount: keepCount, UpdatedAt: &first.UpdatedAt}
	for _, c := range configs {
		if c.KeepCount > keepCount { keepCount = c.KeepCount }
		items, err := s.repo.ListResults(ctx, c.ID, c.KeepCount)
		if err != nil { return nil, err }
		for _, item := range items {
			response.Items = append(response.Items, &IntelligenceTestPublicResult{
				ID: item.ID,
				GroupName: item.GroupName, ModelID: item.ModelID, Status: item.Status,
				Check: item.Check, SVGContent: item.SVGContent, LatencyMs: item.LatencyMs,
				StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, CreatedAt: item.CreatedAt,
			})
		}
	}
	response.KeepCount = keepCount
	response.Config.KeepCount = keepCount
	sort.Slice(response.Items, func(i, j int) bool { return response.Items[i].CreatedAt.After(response.Items[j].CreatedAt) })
	seen := make(map[string]struct{})
	for _, item := range response.Items {
		if item.GroupName != "" {
			if _, ok := seen[item.GroupName]; !ok {
				seen[item.GroupName] = struct{}{}
				response.Groups = append(response.Groups, item.GroupName)
			}
		}
		if response.Updated == nil || item.CreatedAt.After(*response.Updated) {
			t := item.CreatedAt
			response.Updated = &t
		}
	}
	return response, nil
}
