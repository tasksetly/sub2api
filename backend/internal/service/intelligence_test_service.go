package service

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/robfig/cron/v3"
)

var (
	ErrIntelligenceTestConfigNotFound = infraerrors.NotFound("INTELLIGENCE_TEST_CONFIG_NOT_FOUND", "intelligence test config not found")
	ErrIntelligenceTestConfigInvalid  = infraerrors.BadRequest("INTELLIGENCE_TEST_CONFIG_INVALID", "name, model, prompt, and API key are required")
	ErrIntelligenceTestAlreadyRunning = infraerrors.BadRequest("INTELLIGENCE_TEST_ALREADY_RUNNING", "intelligence test is already running")
)

const intelligenceTestSVGSystemPrompt = "只输出完整的 SVG 代码，以 <svg> 开始、以 </svg> 结束。不要 Markdown 代码块、解释文字或其他内容。"

type IntelligenceTestConfig struct {
	AnswerMode string `json:"answer_mode"`
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	ModelID         string     `json:"model_id"`
	ModelName       string     `json:"model_name"`
	ReasoningEffort string     `json:"reasoning_effort"`
	PromptText      string     `json:"prompt_text"`
	APIKeyID        int64      `json:"api_key_id"`
	APIKeyName      string     `json:"api_key_name"`
	GroupID         *int64     `json:"group_id"`
	GroupName       string     `json:"group_name"`
	CronExpression  string     `json:"cron_expression"`
	KeepCount       int        `json:"keep_count"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type IntelligenceTestResult struct {
	ResponseText string `json:"response_text,omitempty"`
	AnswerMode string `json:"answer_mode"`
	AccountID    *int64     `json:"account_id,omitempty"`
	AccountName  string     `json:"account_name,omitempty"`
	PromptText   string     `json:"prompt_text,omitempty"`
	ID           int64      `json:"id"`
	ConfigID     int64      `json:"config_id"`
	APIKeyID     int64      `json:"api_key_id"`
	GroupID      *int64     `json:"group_id,omitempty"`
	GroupName    string     `json:"group_name"`
	RateMultiplier float64  `json:"rate_multiplier"`
	ModelID      string     `json:"model_id"`
	ModelName    string     `json:"model_name"`
	ReasoningEffort string  `json:"effort,omitempty"`
	Status       string     `json:"status"`
	Check        bool       `json:"check"`
	SVGContent   string     `json:"svg_content,omitempty"`
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
	UpdateRun(context.Context, int64, time.Time, time.Time, *time.Time) error
	ListAPIKeyOptions(context.Context) ([]*IntelligenceTestAPIKeyOption, error)
	InsertResult(context.Context, *IntelligenceTestResult) error
	UpdateAccountResult(context.Context, *IntelligenceTestResult) error
	FailInterruptedAccountResults(context.Context, int64) error
	ListAccountResults(context.Context, int64, int64, int) ([]*IntelligenceTestResult, error)
	PruneResults(context.Context, int64) error
	ListResults(context.Context, int64, int) ([]*IntelligenceTestResult, error)
	ListSVGResults(context.Context, int64, int) ([]*IntelligenceTestResult, error)
}

type IntelligenceTestConfigInput struct {
	AnswerMode string `json:"answer_mode"`
	Name           string `json:"name"`
	ModelID        string `json:"model_id"`
	ModelName      string `json:"model_name"`
	ReasoningEffort string `json:"reasoning_effort"`
	PromptText     string `json:"prompt_text"`
	APIKeyID       int64  `json:"api_key_id"`
	CronExpression string `json:"cron_expression"`
	KeepCount      int    `json:"keep_count"`
	Enabled        *bool  `json:"enabled"`
}

type IntelligenceTestPublicConfig struct {
	AnswerMode string `json:"answer_mode"`
	Title      string     `json:"title"`
	Description string    `json:"description"`
	PromptName string     `json:"prompt_name"`
	PromptText string     `json:"prompt_text"`
	ModelID    string     `json:"model_id"`
	ModelName  string     `json:"model_name"`
	ReasoningEffort string `json:"reasoning_effort"`
	KeepCount  int        `json:"keep_count"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type IntelligenceTestPublicResult struct {
	ResponseText string `json:"response_text,omitempty"`
	AnswerMode string `json:"answer_mode"`
	ID         int64     `json:"id"`
	GroupID    *int64    `json:"group_id,omitempty"`
	GroupName  string    `json:"group_name"`
	RateMultiplier float64 `json:"rate_multiplier"`
	ModelID    string    `json:"model_id"`
	ModelName  string    `json:"model_name"`
	ReasoningEffort string `json:"effort,omitempty"`
	Status     string    `json:"status"`
	Check      bool      `json:"check"`
	SVGContent string    `json:"svg_content,omitempty"`
	LatencyMs  int64     `json:"latency_ms"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type IntelligenceTestPublicGroup struct {
	ID             *int64 `json:"id,omitempty"`
	Name           string `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type IntelligenceTestPublicResponse struct {
	Config    *IntelligenceTestPublicConfig `json:"config,omitempty"`
	KeepCount int                           `json:"keep_count"`
	Items     []*IntelligenceTestPublicResult `json:"items"`
	Groups    []IntelligenceTestPublicGroup  `json:"groups"`
	Updated   *time.Time                    `json:"updated,omitempty"`
}

type IntelligenceTestRunStatus struct {
	Running    bool       `json:"running"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ResultID   int64      `json:"result_id,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type IntelligenceTestService struct {
	repo          IntelligenceTestRepository
	apiKeyRepo    APIKeyRepository
	accountRepo   AccountRepository
	accountTest   *AccountTestService
	gateway       *GatewayService
	db            *sql.DB
	cron          *cron.Cron
	startOnce     sync.Once
	stopOnce      sync.Once
	runMu         sync.Mutex
	runs          map[int64]IntelligenceTestRunStatus
	runSlots      chan struct{}
}

func NewIntelligenceTestService(repo IntelligenceTestRepository, apiKeyRepo APIKeyRepository, accountRepo AccountRepository, accountTest *AccountTestService, gateway *GatewayService, db *sql.DB) *IntelligenceTestService {
	return &IntelligenceTestService{repo: repo, apiKeyRepo: apiKeyRepo, accountRepo: accountRepo, accountTest: accountTest, gateway: gateway, db: db, runs: make(map[int64]IntelligenceTestRunStatus), runSlots: make(chan struct{}, scheduledTestDefaultMaxWorkers)}
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

// GetModelListAPIKey supplies the selected key's group to the existing gateway model listing.
func (s *IntelligenceTestService) GetModelListAPIKey(ctx context.Context, id int64) (*APIKey, error) {
	key, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if key == nil || !key.IsActive() || key.IsExpired() || key.GroupID == nil || key.Group == nil {
		return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_KEY_INVALID", "an active API key bound to a group is required")
	}
	return key, nil
}

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
	if c.AnswerMode != next.AnswerMode {
		return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_MODE_IMMUTABLE", "create a new config to change the test type")
	}
	c.Name, c.ModelID, c.ModelName, c.PromptText, c.APIKeyID = next.Name, next.ModelID, next.ModelName, next.PromptText, next.APIKeyID
	c.ReasoningEffort = next.ReasoningEffort
	if c.CronExpression != next.CronExpression || (!c.Enabled && next.Enabled) || c.NextRunAt == nil || c.NextRunAt.After(*next.NextRunAt) {
		c.NextRunAt = next.NextRunAt
	}
	c.CronExpression, c.KeepCount, c.Enabled = next.CronExpression, next.KeepCount, next.Enabled
	return s.repo.UpdateConfig(ctx, c)
}

func (s *IntelligenceTestService) DeleteConfig(ctx context.Context, id int64) error { return s.repo.DeleteConfig(ctx, id) }

func (s *IntelligenceTestService) normalizeConfig(input IntelligenceTestConfigInput) (*IntelligenceTestConfig, error) {
	mode := strings.TrimSpace(input.AnswerMode)
	if mode == "" { mode = "svg" }
	if mode != "svg" && mode != "candy" { return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_MODE_INVALID", "test type must be svg or candy") }
	name, model, prompt := strings.TrimSpace(input.Name), strings.TrimSpace(input.ModelID), strings.TrimSpace(input.PromptText)
	if mode == "candy" { prompt = intelligenceTestCandyPrompt }
	if name == "" || model == "" || prompt == "" || input.APIKeyID <= 0 { return nil, ErrIntelligenceTestConfigInvalid }
	effort := strings.ToLower(strings.TrimSpace(input.ReasoningEffort))
	switch effort {
	case "", "low", "medium", "high", "xhigh", "max":
	default:
		return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_EFFORT_INVALID", "invalid reasoning effort")
	}
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
	modelName := strings.TrimSpace(input.ModelName)
	if modelName == "" { modelName = model }
	return &IntelligenceTestConfig{AnswerMode: mode, Name: name, ModelID: model, ModelName: modelName, ReasoningEffort: effort, PromptText: prompt, APIKeyID: input.APIKeyID, CronExpression: cronExpression, KeepCount: keepCount, Enabled: enabled, NextRunAt: &next}, nil
}

func (s *IntelligenceTestService) RunNow(ctx context.Context, id int64) (*IntelligenceTestResult, error) {
	return s.run(ctx, id, nil)
}

// Use the same per-config lock for scheduled and manual runs. Reloading the
// config under the lock prevents a stale due-list from running a completed job.
func (s *IntelligenceTestService) acquireRun(ctx context.Context, id int64, dueAt *time.Time) (*IntelligenceTestConfig, IntelligenceTestRunStatus, func(), error) {
	release, acquired, err := s.tryLockRun(ctx, id)
	if err != nil {
		return nil, IntelligenceTestRunStatus{}, nil, err
	}
	if !acquired {
		return nil, IntelligenceTestRunStatus{}, nil, ErrIntelligenceTestAlreadyRunning
	}
	c, err := s.repo.GetConfig(ctx, id)
	if err != nil {
		release()
		return nil, IntelligenceTestRunStatus{}, nil, err
	}
	if dueAt != nil && (!c.Enabled || (c.NextRunAt != nil && c.NextRunAt.After(*dueAt))) {
		release()
		return nil, IntelligenceTestRunStatus{}, nil, nil
	}
	state, started := s.beginRun(id)
	if !started {
		release()
		return nil, state, nil, ErrIntelligenceTestAlreadyRunning
	}
	return c, state, release, nil
}

func (s *IntelligenceTestService) tryLockRun(ctx context.Context, id int64) (func(), bool, error) {
	// Never silently fall back to a process-local lock in a multi-instance job.
	if s.db == nil {
		return nil, false, errors.New("intelligence test database lock is unavailable")
	}
	return tryAcquireDBAdvisoryLockWithError(ctx, s.db, hashAdvisoryLockID("intelligence_test:config:"+strconv.FormatInt(id, 10)))
}

func (s *IntelligenceTestService) run(ctx context.Context, id int64, dueAt *time.Time) (*IntelligenceTestResult, error) {
	c, _, release, err := s.acquireRun(ctx, id, dueAt)
	if err != nil || c == nil {
		return nil, err
	}
	defer release()
	result, err := s.executeRun(ctx, c)
	s.finishRun(id, result, err)
	return result, err
}

// QueueRun detaches a manually started benchmark from the browser request.
// A high-effort SVG can then finish even when the HTTP client times out or closes.
func (s *IntelligenceTestService) QueueRun(ctx context.Context, id int64) (*IntelligenceTestRunStatus, error) {
	c, state, release, err := s.acquireRun(ctx, id, nil)
	if errors.Is(err, ErrIntelligenceTestAlreadyRunning) {
		state, err := s.RunStatus(ctx, id)
		return &state, err
	}
	if err != nil {
		return nil, err
	}
	go func() {
		defer release()
		jobCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		result, runErr := s.executeRun(jobCtx, c)
		s.finishRun(id, result, runErr)
	}()
	return &state, nil
}

func (s *IntelligenceTestService) RunStatus(ctx context.Context, id int64) (IntelligenceTestRunStatus, error) {
	release, acquired, err := s.tryLockRun(ctx, id)
	if err != nil {
		return IntelligenceTestRunStatus{}, err
	}
	if !acquired {
		s.runMu.Lock()
		state := s.runs[id]
		s.runMu.Unlock()
		if !state.Running {
			state = IntelligenceTestRunStatus{}
		}
		state.Running, state.Status = true, "running"
		return state, nil
	}
	defer release()
	// Polling can land on any instance. Persisted results are authoritative once
	// the lock is released, even when this process never ran the benchmark.
	items, err := s.repo.ListResults(ctx, id, 1)
	if err != nil {
		return IntelligenceTestRunStatus{}, err
	}
	s.runMu.Lock()
	state, known := s.runs[id]
	s.runMu.Unlock()
	// Keep a local persistence failure visible instead of reporting an older
	// successful result when the failed run could not be saved.
	if known && !state.Running && (len(items) == 0 || state.StartedAt.After(items[0].StartedAt)) {
		return state, nil
	}
	if len(items) > 0 {
		item := items[0]
		return IntelligenceTestRunStatus{Status: item.Status, StartedAt: item.StartedAt, FinishedAt: &item.FinishedAt, ResultID: item.ID, Error: item.ErrorMessage}, nil
	}
	return IntelligenceTestRunStatus{Status: "idle"}, nil
}

func (s *IntelligenceTestService) beginRun(id int64) (IntelligenceTestRunStatus, bool) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if state, ok := s.runs[id]; ok && state.Running { return state, false }
	state := IntelligenceTestRunStatus{Running: true, Status: "running", StartedAt: time.Now()}
	s.runs[id] = state
	return state, true
}

func (s *IntelligenceTestService) finishRun(id int64, result *IntelligenceTestResult, err error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	state := s.runs[id]
	state.Running = false
	finished := time.Now()
	state.FinishedAt = &finished
	if result != nil {
		state.ResultID = result.ID
		state.Status = result.Status
		state.Error = result.ErrorMessage
	}
	if err != nil {
		state.Status = "failed"
		state.Error = err.Error()
	}
	s.runs[id] = state
}

func (s *IntelligenceTestService) executeRun(ctx context.Context, c *IntelligenceTestConfig) (*IntelligenceTestResult, error) {
	result, err := s.runConfig(ctx, c)
	now := time.Now()
	next := now
	if schedule, parseErr := scheduledTestCronParser.Parse(c.CronExpression); parseErr == nil { next = schedule.Next(now) }
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if updateErr := s.repo.UpdateRun(writeCtx, c.ID, now, next, c.NextRunAt); updateErr != nil && err == nil { err = updateErr }
	return result, err
}

func (s *IntelligenceTestService) runDue() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	now := time.Now()
	configs, err := s.repo.ListDueConfigs(ctx, now)
	cancel()
	if err != nil {
		slog.Error("intelligence_test_list_due_failed", "error", err)
		return
	}
	var wg sync.WaitGroup
dispatch:
	for _, c := range configs {
		select {
		case s.runSlots <- struct{}{}:
		default:
			break dispatch
		}
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			defer func() { <-s.runSlots }()
			// Give each inference its own timeout so a slow SVG cannot exhaust another test's budget.
			jobCtx, jobCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer jobCancel()
			if _, err := s.run(jobCtx, id, &now); err != nil && !errors.Is(err, ErrIntelligenceTestAlreadyRunning) {
				slog.Error("intelligence_test_run_failed", "config_id", id, "error", err)
			}
		}(c.ID)
	}
	wg.Wait()
}

func (s *IntelligenceTestService) runConfig(ctx context.Context, c *IntelligenceTestConfig) (*IntelligenceTestResult, error) {
	started := time.Now()
	result := &IntelligenceTestResult{AnswerMode: c.AnswerMode, PromptText: c.PromptText, ConfigID: c.ID, APIKeyID: c.APIKeyID, ModelID: c.ModelID, ModelName: c.ModelName, ReasoningEffort: c.ReasoningEffort, Status: "failed", StartedAt: started, CreatedAt: started}
	key, err := s.apiKeyRepo.GetByID(ctx, c.APIKeyID)
	if err == nil && key.GroupID != nil {
		result.GroupID = key.GroupID
		if key.Group != nil { result.GroupName = key.Group.Name }
		if s.gateway == nil {
			result.ErrorMessage = "gateway service unavailable"
		} else {
			run, runErr := s.runThroughGateway(ctx, key, c)
			applyIntelligenceTestRun(ctx, result, run, runErr)
			if runErr != nil {
				slog.Error("intelligence_test_upstream_failed", "config_id", c.ID, "error", runErr)
			}
		}
	} else if err != nil { result.ErrorMessage = "API key lookup failed" } else { result.ErrorMessage = "API key is not bound to a group" }
	if result.FinishedAt.IsZero() { result.FinishedAt = time.Now(); result.LatencyMs = result.FinishedAt.Sub(started).Milliseconds() }
	// Persist the completed state with a fresh, short context even when the
	// upstream inference hit its deadline. Only successful candy replies are retained.
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.InsertResult(writeCtx, result); err != nil { return result, err }
	if err := s.repo.PruneResults(writeCtx, result.ConfigID); err != nil { return result, err }
	return result, nil
}

// Keep group load-aware selection, then probe the account's native protocol.
func (s *IntelligenceTestService) runThroughGateway(ctx context.Context, key *APIKey, c *IntelligenceTestConfig) (*ScheduledTestResult, error) {
	if s.accountTest == nil { return nil, fmt.Errorf("account test service unavailable") }
	selection, err := s.gateway.SelectAccountWithLoadAwareness(ctx, key.GroupID, "", c.ModelID, nil, "", key.UserID)
	if err != nil { return nil, err }
	if selection == nil || selection.Account == nil { return nil, fmt.Errorf("no schedulable account in the API key group") }
	if selection.ReleaseFunc != nil { defer selection.ReleaseFunc() }

	systemPrompt, prompt := intelligenceTestSVGSystemPrompt, c.PromptText
	if c.AnswerMode == "candy" {
		systemPrompt, prompt = "", intelligenceTestCandyPrompt
	}
	return s.accountTest.RunTestBackgroundWithOptions(ctx, selection.Account.ID, c.ModelID, prompt, AccountTestOptions{
		ReasoningEffort: c.ReasoningEffort,
		SystemPrompt:    systemPrompt,
	})
}

func extractSVG(value string) string {
	start := regexp.MustCompile(`(?is)<svg\b`).FindStringIndex(value)
	if start == nil { return "" }
	fragment := value[start[0]:]
	// Try strict XML parse first; it handles depth correctly for well-formed SVG.
	if svg := extractSVGStrict(fragment); svg != "" {
		return svg
	}
	// shortcut: string-search fallback for non-strict XML (unescaped & or HTML entities
	// that models commonly produce); doesn't handle nested <svg>, upgrade if needed.
	end := strings.LastIndex(strings.ToLower(fragment), "</svg>")
	if end < 0 { return "" }
	return strings.TrimSpace(fragment[:end+len("</svg>")])
}

// extractSVGStrict uses a streaming XML decoder to find the closing </svg> tag at
// the correct nesting depth. Returns "" on any parse error.
func extractSVGStrict(fragment string) string {
	decoder := xml.NewDecoder(strings.NewReader(fragment))
	rootSeen, depth := false, 0
	for {
		token, err := decoder.Token()
		if err != nil { return "" }
		switch element := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if rootSeen || !strings.EqualFold(element.Name.Local, "svg") { return "" }
				rootSeen = true
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 { return "" }
			if rootSeen && depth == 0 { return strings.TrimSpace(fragment[:decoder.InputOffset()]) }
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(element)) != "" { return "" }
		case xml.Directive, xml.ProcInst:
			return ""
		}
	}
}

func (s *IntelligenceTestService) Public(ctx context.Context, configID int64, answerMode string) (*IntelligenceTestPublicResponse, error) {
	if answerMode == "" { answerMode = "svg" }
	if answerMode != "svg" && answerMode != "candy" { return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_MODE_INVALID", "test type must be svg or candy") }
	response := &IntelligenceTestPublicResponse{Items: make([]*IntelligenceTestPublicResult, 0), Groups: make([]IntelligenceTestPublicGroup, 0)}
	if answerMode == "candy" {
		response.Config = &IntelligenceTestPublicConfig{AnswerMode: "candy", PromptText: intelligenceTestCandyPrompt}
	}
	configs := make([]*IntelligenceTestConfig, 0)
	if configID > 0 {
		c, err := s.repo.GetConfig(ctx, configID)
		if err != nil {
			if errors.Is(err, ErrIntelligenceTestConfigNotFound) { return response, nil }
			return nil, err
		}
		if c.Enabled && c.AnswerMode == answerMode { configs = append(configs, c) }
	} else {
		all, err := s.repo.ListConfigs(ctx)
		if err != nil { return nil, err }
		for _, c := range all {
			if c.Enabled && c.AnswerMode == answerMode { configs = append(configs, c) }
		}
	}
	if len(configs) == 0 { return response, nil }

	first := configs[0]
	keepCount := first.KeepCount
	response.Config = &IntelligenceTestPublicConfig{AnswerMode: answerMode, Title: first.Name, Description: "API Key 分组 SVG 生成能力测试", PromptName: first.Name, PromptText: first.PromptText, ModelID: first.ModelID, ModelName: first.ModelName, ReasoningEffort: first.ReasoningEffort, KeepCount: keepCount, UpdatedAt: &first.UpdatedAt}
	if answerMode == "candy" {
		response.Config.Title, response.Config.Description = "", ""
		response.Config.PromptText = intelligenceTestCandyPrompt
	}
	seenGroups := make(map[int64]bool)
	for _, c := range configs {
		if answerMode == "candy" && c.GroupID != nil && !seenGroups[*c.GroupID] {
			seenGroups[*c.GroupID] = true
			response.Groups = append(response.Groups, IntelligenceTestPublicGroup{ID: c.GroupID, Name: c.GroupName})
		}
		if c.KeepCount > keepCount { keepCount = c.KeepCount }
		var items []*IntelligenceTestResult
		var err error
		if answerMode == "candy" { items, err = s.repo.ListResults(ctx, c.ID, c.KeepCount) } else { items, err = s.repo.ListSVGResults(ctx, c.ID, c.KeepCount) }
		if err != nil { return nil, err }
		for _, item := range items {
			if item.AnswerMode != answerMode { continue }
			rateMultiplier := item.RateMultiplier
			if rateMultiplier <= 0 { rateMultiplier = 1 }
			response.Items = append(response.Items, &IntelligenceTestPublicResult{
				ID: item.ID, AnswerMode: item.AnswerMode, ResponseText: item.ResponseText,
				GroupID: item.GroupID,
				GroupName: item.GroupName, ModelID: item.ModelID, ModelName: item.ModelName, ReasoningEffort: item.ReasoningEffort, Status: item.Status,
				RateMultiplier: rateMultiplier,
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
		if item.GroupName != "" && answerMode != "candy" {
			if _, ok := seen[item.GroupName]; !ok {
				seen[item.GroupName] = struct{}{}
				response.Groups = append(response.Groups, IntelligenceTestPublicGroup{
					ID: item.GroupID, Name: item.GroupName, RateMultiplier: item.RateMultiplier,
				})
			}
		}
		if response.Updated == nil || item.CreatedAt.After(*response.Updated) {
			t := item.CreatedAt
			response.Updated = &t
		}
	}
	return response, nil
}
