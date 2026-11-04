package service

import (
	"context"
	"time"
)

// 指纹校对的判定四态。
//
// 只有 FingerprintVerdictMismatch 计入连续不符计数，也只有它能触发动账号的动作。
// Inconclusive 与 Error 都表示"这次没能得出结论"，绝不等于造假——把它们当作不符
// 会让一次 429 或一次参考库重训就停掉正常账号。
const (
	// FingerprintVerdictMatch 实判模型与预期模型一致。
	FingerprintVerdictMatch = "match"
	// FingerprintVerdictMismatch 实判模型与预期模型不一致，疑似上游拿别的模型冒充。
	FingerprintVerdictMismatch = "mismatch"
	// FingerprintVerdictInconclusive 拿不到可用排名：decision 为 unscorable/not_model，
	// 或预期模型已不在参考库中。
	FingerprintVerdictInconclusive = "inconclusive"
	// FingerprintVerdictError 检测本身失败：请求出错、超时、参考数据加载失败、
	// 账号类型不支持，或被并发闸/每日配额拦下。
	FingerprintVerdictError = "error"
)

// FingerprintAPI 是 SDK 支持的三种上游协议。
type FingerprintAPI string

const (
	FingerprintAPIResponses       FingerprintAPI = "responses"
	FingerprintAPIChatCompletions FingerprintAPI = "chat_completions"
	FingerprintAPIMessages        FingerprintAPI = "messages"
)

// IsValidFingerprintAPI 报告 api 是否为 SDK 接受的取值。
func IsValidFingerprintAPI(api string) bool {
	switch FingerprintAPI(api) {
	case FingerprintAPIResponses, FingerprintAPIChatCompletions, FingerprintAPIMessages:
		return true
	default:
		return false
	}
}

// 跳过原因：被并发闸或每日配额拦下时写入结果，避免静默跳过。
const (
	FingerprintSkipConcurrency  = "concurrency_gate"
	FingerprintSkipDailyGlobal  = "daily_limit_global"
	FingerprintSkipDailyAccount = "daily_limit_account"
)

// 规格 6.4 的默认值。
const (
	FingerprintDefaultCron              = "0 */6 * * *"
	FingerprintDefaultChallengeCount    = 3
	FingerprintDefaultMaxResults        = 50
	FingerprintDefaultMismatchThreshold = 2
	FingerprintDefaultMaxConcurrency    = 3
	FingerprintDefaultCacheTTLHours     = 24
	FingerprintDefaultDailyLimitGlobal  = 200
	FingerprintDefaultDailyLimitAccount = 8
)

// FingerprintPlan 是一个账号上的一条指纹校对计划。
//
// 一条计划只测一个模型：账号下即使挂了多个模型，也只针对 TestModel 跑，不遍历。
// 要测多个模型就建多条计划。
type FingerprintPlan struct {
	ID        int64 `json:"id"`
	AccountID int64 `json:"account_id"`

	// TestModel 是发给上游的最终模型名（经本地 model_mapping 映射之后的值）。
	// detect() 直连上游、绕过本地映射，填错就测不到真实链路。
	TestModel string `json:"test_model"`
	// ExpectedModel 是参考库里的规范化模型 ID，用于和 analysis.prediction 比对。
	ExpectedModel string `json:"expected_model"`

	API            string `json:"api"`
	ChallengeCount int    `json:"challenge_count"`
	CronExpression string `json:"cron_expression"`
	Enabled        bool   `json:"enabled"`
	MaxResults     int    `json:"max_results"`

	ConsecutiveMismatches   int `json:"consecutive_mismatches"`
	MismatchActionThreshold int `json:"mismatch_action_threshold"`

	FailureDisable                  bool    `json:"failure_disable"`
	FailureTempUnschedulableMinutes int     `json:"failure_temp_unschedulable_minutes"`
	FailureRemoveGroupIDs           []int64 `json:"failure_remove_group_ids,omitempty"`
	SuccessAddGroupIDs              []int64 `json:"success_add_group_ids,omitempty"`

	LastRunAt *time.Time `json:"last_run_at"`
	NextRunAt *time.Time `json:"next_run_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// DefaultFingerprintPlan 返回带规格默认值的空计划。
// 失败动作一律默认关闭：默认只标记告警，停不停由管理员决定。
func DefaultFingerprintPlan() FingerprintPlan {
	return FingerprintPlan{
		API:                     string(FingerprintAPIChatCompletions),
		ChallengeCount:          FingerprintDefaultChallengeCount,
		CronExpression:          FingerprintDefaultCron,
		Enabled:                 true,
		MaxResults:              FingerprintDefaultMaxResults,
		MismatchActionThreshold: FingerprintDefaultMismatchThreshold,
		FailureRemoveGroupIDs:   []int64{},
		SuccessAddGroupIDs:      []int64{},
	}
}

// FingerprintResult 是一次指纹检测的结果。
type FingerprintResult struct {
	ID     int64 `json:"id"`
	PlanID int64 `json:"plan_id"`

	Verdict  string `json:"verdict"`
	Decision string `json:"decision"`

	PredictedModel     string   `json:"predicted_model"`
	PredictedModelName string   `json:"predicted_model_name"`
	PredictedFamily    string   `json:"predicted_family"`
	Probability        *float64 `json:"probability"`

	// AnalysisJSON 是完整的 lmfpd.Analysis 序列化结果，供人工复核全部候选排名。
	AnalysisJSON string `json:"analysis_json,omitempty"`

	ErrorMessage string `json:"error_message"`
	SkipReason   string `json:"skip_reason"`

	EstimatedOutputTokens int   `json:"estimated_output_tokens"`
	LatencyMs             int64 `json:"latency_ms"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// FingerprintRefModel 是参考库 bundle.json 里的一个模型条目。
type FingerprintRefModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Family      string `json:"family"`
	FamilyName  string `json:"family_name"`
}

// FingerprintEligibility 说明一个账号能否做指纹校对。
// 不支持时 Reason 是可直接展示给管理员的 i18n key。
type FingerprintEligibility struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
	// ProxyBypassed 为真时，该账号配了代理但检测不会经过它（SDK 的 HTTP 客户端不可注入）。
	// 界面上必须据此提示"检测未经该账号代理，结论仅供参考"。
	ProxyBypassed bool `json:"proxy_bypassed"`
}

// FingerprintPlanRepository 是指纹计划的数据访问接口。
type FingerprintPlanRepository interface {
	Create(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error)
	GetByID(ctx context.Context, id int64) (*FingerprintPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*FingerprintPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*FingerprintPlan, error)
	Update(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
	UpdateConsecutiveMismatches(ctx context.Context, planID int64, n int) error
}

// FingerprintResultRepository 是指纹检测结果的数据访问接口。
type FingerprintResultRepository interface {
	Create(ctx context.Context, result *FingerprintResult) (*FingerprintResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int) ([]*FingerprintResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
	// SumEstimatedTokensSince 汇总某账号自 since 起的估算输出 token，用于面板上的累计消耗。
	SumEstimatedTokensSince(ctx context.Context, accountID int64, since time.Time) (int64, error)
	// CountSince 统计某账号自 since 起实际发生的检测次数（不含被配额跳过的）。
	CountSince(ctx context.Context, accountID int64, since time.Time) (int64, error)
	// CountAllSince 统计全局自 since 起实际发生的检测次数，用于每日全局配额。
	CountAllSince(ctx context.Context, since time.Time) (int64, error)
}
