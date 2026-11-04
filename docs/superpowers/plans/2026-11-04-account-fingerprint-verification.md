# 账号指纹校对 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给账号管理加一套指纹校对能力，用 LM Fingerpoint Detector 核对上游账号实际回的是不是它声称的模型。

**Architecture:** 新建一组独立的 `fingerprint_plans` / `fingerprint_results` 表与配套 repo/service/runner，**不**扩展既有 `scheduled_test_plans`（prompt/expected_answer/error_codes 等字段对指纹类型无意义，混在一起会让两种语义都难读）。runner 照搬既有定时测试 runner 的形状——每分钟 cron tick、`ListDue`、信号量、`tryAcquireSingletonLeaderLock` 计划锁、`StartPlanNow` 手动触发——但不共用实例。检测走 SDK 的 `Detect()` 直连上游。

**Tech Stack:** Go 1.27.3 + Gin + 原生 SQL 迁移；`github.com/Ikaleio/lm-detector/sdk/go`；Vue 3 + TypeScript + TailwindCSS；Redis（集群级并发闸与每日配额）

**Spec:** `docs/superpowers/specs/2026-11-04-account-fingerprint-verification-design.md`

## Global Constraints

- 新依赖只允许一个：`github.com/Ikaleio/lm-detector/sdk/go`。`github.com/klauspost/compress` 从 `v1.18.2` 上浮到 `v1.18.4`。**前端不得新增任何第三方依赖。**
- **不运行编译、构建或测试命令**（`AGENTS.md`）。本计划的「运行测试」步骤默认**只写不跑**，除非用户明确授权执行；跳过时要在交付说明里讲明跳过了哪些检查。用 `git diff --check` 和静态检查代替。
- 每个任务结束提交一次本地 commit，**中文 commit message**，不 push。只提交本任务改动的文件——仓库里已存在的未跟踪文件（`frontend/src/components/layout/SidePanel.vue`、`TopNav.vue`、`navConfig.ts`、`__tests__/SidePanel.spec.ts`、`backend/internal/payment/provider/easypay_create_test.go`）**一律不碰、不提交**。
- 迁移号从 **260** 开始（259 已占用）。
- 判定四态：`match` / `mismatch` / `inconclusive` / `error`。**只有 `mismatch` 计入连续计数，也只有它能触发动账号的动作。**
- 动账号的动作门槛：**连续 2 次 `mismatch`**。告警与账号列表标记是**单次立即出**。
- 默认值：cron `0 */6 * * *`、`Count` 3、`MaxResults` 50、`MismatchActionThreshold` 2、全局并发闸 3、失败动作全关、邮件告警关、接入时校验不勾选。
- 凭证绝不进日志、错误信息或 API 响应。
- 中英文 i18n 同步补齐。

## Review Focus

1. **上游返回 200 但 body 是 HTML 错误页或非法 JSON** —— SDK 归为 `errInvalidResponse`，必须落 `error` 态，绝不能当成 `mismatch` 去停账号。（Task 5）
2. **`base_url` 带 query / fragment / 内嵌凭证** —— SDK 在发请求前就拒绝（`Base URL must use HTTP or HTTPS without credentials, a query, or a fragment`）。要在**建计划时**校验并给出可读提示，而不是等到抽查时才炸。（Task 3）
3. **参考库重训后 `expected_model` 从库里消失** —— 不能每轮都判 `mismatch`（永久误报），要落 `inconclusive` 并提示管理员重选预期模型。（Task 5）
4. **错误信息与日志泄露 API Key** —— SDK 自己会 `[REDACTED]`，但我们拼接的错误信息、`analysis_json`、ops 事件的 `dimensions` 都不能带出 credentials。（Task 7、Task 8）
5. **cron 被改成 `* * * * *`** —— 每日次数上限必须真的拦得住，且跳过要留痕。（Task 4）

---

### Task 1: 迁移、领域模型与仓储

**Files:**
- Create: `backend/migrations/260_fingerprint_plans.sql`
- Create: `backend/internal/service/fingerprint_port.go`
- Create: `backend/internal/repository/fingerprint_repo.go`
- Test: `backend/internal/service/fingerprint_port_test.go`

**Interfaces:**
- Consumes: 既有 `accounts(id)`、`groups(id)` 表
- Produces: `FingerprintPlan`、`FingerprintResult`、`FingerprintPlanRepository`、`FingerprintResultRepository`、`DefaultFingerprintPlan()`、四态常量 `FingerprintVerdictMatch/Mismatch/Inconclusive/Error`

- [ ] **Step 1: 写迁移 `260_fingerprint_plans.sql`**

两张表，照 `066_add_scheduled_test_tables.sql` 的写法（`CREATE TABLE IF NOT EXISTS`、`BIGSERIAL`、`TIMESTAMPTZ`、`ON DELETE CASCADE`）。

`fingerprint_plans`：`id`、`account_id`（REFERENCES accounts ON DELETE CASCADE）、`test_model VARCHAR(200) NOT NULL`、`expected_model VARCHAR(100) NOT NULL`、`api VARCHAR(20) NOT NULL DEFAULT 'chat_completions'`、`challenge_count INT NOT NULL DEFAULT 3`、`cron_expression VARCHAR(100) NOT NULL DEFAULT '0 */6 * * *'`、`enabled BOOLEAN NOT NULL DEFAULT true`、`max_results INT NOT NULL DEFAULT 50`、`consecutive_mismatches INT NOT NULL DEFAULT 0`、`mismatch_action_threshold INT NOT NULL DEFAULT 2`、`failure_disable BOOLEAN NOT NULL DEFAULT false`、`failure_temp_unschedulable_minutes INT NOT NULL DEFAULT 0`、`failure_remove_group_ids BIGINT[] NOT NULL DEFAULT '{}'`、`success_add_group_ids BIGINT[] NOT NULL DEFAULT '{}'`、`last_run_at`、`next_run_at`、`created_at`、`updated_at`。

索引：`(account_id)`；`(enabled, next_run_at) WHERE enabled = true`。

`fingerprint_results`：`id`、`plan_id`（CASCADE）、`verdict VARCHAR(20) NOT NULL`、`decision VARCHAR(20) NOT NULL DEFAULT ''`、`predicted_model VARCHAR(100) NOT NULL DEFAULT ''`、`predicted_model_name VARCHAR(200) NOT NULL DEFAULT ''`、`predicted_family VARCHAR(100) NOT NULL DEFAULT ''`、`probability DOUBLE PRECISION`、`analysis_json JSONB`、`error_message TEXT NOT NULL DEFAULT ''`、`skip_reason VARCHAR(100) NOT NULL DEFAULT ''`、`estimated_output_tokens INT NOT NULL DEFAULT 0`、`latency_ms BIGINT NOT NULL DEFAULT 0`、`started_at`、`finished_at`、`created_at`。索引 `(plan_id, created_at DESC)`。

- [ ] **Step 2: 写失败的测试 `fingerprint_port_test.go`**

```go
func TestDefaultFingerprintPlanUsesSpecDefaults(t *testing.T) {
	p := DefaultFingerprintPlan()
	if p.CronExpression != "0 */6 * * *" { t.Fatalf("cron = %q", p.CronExpression) }
	if p.ChallengeCount != 3 { t.Fatalf("challenge count = %d", p.ChallengeCount) }
	if p.MaxResults != 50 { t.Fatalf("max results = %d", p.MaxResults) }
	if p.MismatchActionThreshold != 2 { t.Fatalf("threshold = %d", p.MismatchActionThreshold) }
	if !p.Enabled { t.Fatal("plan should default to enabled") }
	if p.FailureDisable || p.FailureTempUnschedulableMinutes != 0 || len(p.FailureRemoveGroupIDs) != 0 {
		t.Fatal("failure actions must default to off")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestDefaultFingerprintPlan -v`（working dir `backend/`）
Expected: FAIL — `undefined: DefaultFingerprintPlan`
*（按 Global Constraints，未获授权时跳过执行并记录。）*

- [ ] **Step 4: 写 `fingerprint_port.go`**

四态常量、`FingerprintPlan` / `FingerprintResult` 结构体（字段与迁移一一对应，JSON tag 用 snake_case）、`DefaultFingerprintPlan() FingerprintPlan`，以及两个仓储接口——方法签名照 `scheduled_test_port.go` 的 `ScheduledTestPlanRepository` / `ScheduledTestResultRepository`，额外加 `UpdateConsecutiveMismatches(ctx, planID int64, n int) error`。

- [ ] **Step 5: 写 `fingerprint_repo.go`**

照 `backend/internal/repository/scheduled_test_repo.go` 的写法逐方法实现。`BIGINT[]` 用该文件处理 `failure_remove_group_ids` 的同一套方式。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestDefaultFingerprintPlan -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add backend/migrations/260_fingerprint_plans.sql backend/internal/service/fingerprint_port.go backend/internal/service/fingerprint_port_test.go backend/internal/repository/fingerprint_repo.go
git commit -m "feat: 新增指纹校对计划与结果的表结构、领域模型和仓储"
```

---

### Task 2: 依赖接入、系统设置与 Detector 懒加载

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`
- Modify: `backend/internal/service/domain_constants.go`（追加 SettingKey 常量）
- Create: `backend/internal/service/setting_fingerprint.go`
- Create: `backend/internal/service/fingerprint_detector.go`
- Test: `backend/internal/service/fingerprint_detector_test.go`

**Interfaces:**
- Produces: `FingerprintSettings`、`(*SettingService).GetFingerprintSettings(ctx) FingerprintSettings`、`FingerprintDetectorProvider`、`(*FingerprintDetectorProvider).Get(ctx) (*lmfpd.Detector, error)`、`(*FingerprintDetectorProvider).ListReferenceModels(ctx) ([]FingerprintRefModel, error)`

- [ ] **Step 1: 加依赖**

`backend/go.mod` 增 `github.com/Ikaleio/lm-detector/sdk/go`，`github.com/klauspost/compress` 提到 `v1.18.4`。**不要运行 `go mod tidy`**（Global Constraints），手写条目并在交付说明里注明需要用户自行跑一次 tidy/download。

- [ ] **Step 2: 加 SettingKey 常量**

在 `domain_constants.go` 照既有风格（`SettingKeyXxx = "xxx" // 中文注释`）追加：`fingerprint_data_sources`、`fingerprint_cache_path`、`fingerprint_cache_ttl_hours`、`fingerprint_max_concurrency`、`fingerprint_daily_limit_global`、`fingerprint_daily_limit_per_account`、`fingerprint_notify_email`。

- [ ] **Step 3: 写失败的测试**

```go
func TestFingerprintSettingsFallBackToSpecDefaults(t *testing.T) {
	// settingRepo 全部返回 not found
	s := newFingerprintSettingsForTest(t, map[string]string{})
	if got := s.DataSources; !reflect.DeepEqual(got, []string{
		"https://lm.ikale.io/data/",
		"https://cdn.jsdmirror.com/npm/lmfpd-data@latest/",
	}) { t.Fatalf("sources = %v", got) }
	if s.MaxConcurrency != 3 { t.Fatalf("concurrency = %d", s.MaxConcurrency) }
	if s.NotifyEmail { t.Fatal("email must default to off") }
}

func TestFingerprintLocalDirectorySourceSkipsNetwork(t *testing.T) {
	// 单个非 http(s) 来源 → LoadOptions.Sources 原样传入，离线可用
	opts := buildLoadOptions(FingerprintSettings{DataSources: []string{"/srv/lmfpd-data"}})
	if len(opts.Sources) != 1 || opts.Sources[0] != "/srv/lmfpd-data" { t.Fatalf("sources = %v", opts.Sources) }
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestFingerprint -v`
Expected: FAIL — 未定义

- [ ] **Step 5: 写 `setting_fingerprint.go`**

照 `setting_features.go` 的写法：每个设置一个读取方法，`settingRepo.GetValue` 失败就回落到规格默认值。再加一个聚合方法 `GetFingerprintSettings(ctx) FingerprintSettings` 一次性返回全部。

- [ ] **Step 6: 写 `fingerprint_detector.go`**

`FingerprintDetectorProvider` 持有 `*SettingService`，用 `sync.Mutex` + 缓存字段做**懒加载单例**：首次 `Get` 调 `lmfpd.Load(ctx, lmfpd.LoadOptions{Sources, Cache, TTL})`，成功后常驻复用（加载 31ms、评分 2.3ms，绝不能每次检测重新加载）。加载失败**不缓存失败**，下次重试，并把原始错误原样返回给调用方——上层要能在界面上显示"参考数据加载失败"的具体原因。

`buildLoadOptions(FingerprintSettings) lmfpd.LoadOptions` 单独成函数以便测试。

`ListReferenceModels` 从配置的第一个来源取 `bundle.json` 解析 `models[]`（字段 `id` / `display_name` / `family` / `family_name`），按 `family_name` 再按 `id` 排序返回；带一个短 TTL 的内存缓存。本地目录来源时读目录下的 `unified_bank.json`。

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestFingerprint -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add backend/go.mod backend/go.sum backend/internal/service/domain_constants.go backend/internal/service/setting_fingerprint.go backend/internal/service/fingerprint_detector.go backend/internal/service/fingerprint_detector_test.go
git commit -m "feat: 接入指纹检测 SDK，新增数据源设置与 detector 懒加载"
```

---

### Task 3: 账号资格判定与 DetectConfig 映射

**Files:**
- Create: `backend/internal/service/fingerprint_eligibility.go`
- Test: `backend/internal/service/fingerprint_eligibility_test.go`

**Interfaces:**
- Consumes: `Account`（`domain.AccountType*`、`domain.Platform*`、`GetCredential`、`GetBaseURL` / `GetOpenAIBaseURL` / `GetAnthropicProtocolBaseURL` / `GetCNProtocolBaseURL`、`GetTypeSafeAPIKey` / `GetCNAPIKey` / `GetOpenAIApiKey` / `GetOpenAIProtocolAPIKey`）
- Produces: `FingerprintEligibility{Supported bool; Reason string}`、`CheckFingerprintEligibility(a *Account) FingerprintEligibility`、`BuildDetectConfig(a *Account, plan FingerprintPlan) (lmfpd.DetectConfig, error)`

- [ ] **Step 1: 写失败的测试**

```go
func TestFingerprintEligibilityByAccountType(t *testing.T) {
	for _, tt := range []struct{ name, typ string; want bool }{
		{"upstream 支持", domain.AccountTypeUpstream, true},
		{"apikey 支持", domain.AccountTypeAPIKey, true},
		{"oauth 不支持", domain.AccountTypeOAuth, false},
		{"setup-token 不支持", domain.AccountTypeSetupToken, false},
		{"bedrock 不支持", domain.AccountTypeBedrock, false},
		{"service_account 不支持", domain.AccountTypeServiceAccount, false},
	} { /* 断言 Supported，且不支持时 Reason 非空 */ }
}

func TestFingerprintEligibilityRejectsHeaderOverrides(t *testing.T) {
	// 配了 header_overrides 的 upstream 账号 → Supported=false，Reason 说明自定义请求头
}

// Review Focus #2
func TestBuildDetectConfigRejectsUnusableBaseURL(t *testing.T) {
	for _, raw := range []string{
		"https://api.example.com/v1?key=abc",
		"https://user:pw@api.example.com/v1",
		"https://api.example.com/v1#frag",
		"ftp://api.example.com/v1",
	} { /* 断言返回 error，且 error 文本不含凭证 */ }
}

func TestBuildDetectConfigMapsPlatformToAPI(t *testing.T) {
	// anthropic 协议 → lmfpd.APIMessages；chat_completions 协议 → lmfpd.APIChatCompletions
	// Count 取 plan.ChallengeCount；Parallel 不超过 Count
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestFingerprintEligibility -v`
Expected: FAIL

- [ ] **Step 3: 实现 `CheckFingerprintEligibility` 与 `BuildDetectConfig`**

资格规则：仅 `AccountTypeUpstream` / `AccountTypeAPIKey` 支持；配了 `header_overrides` 或非标准鉴权头的归入不支持。`Reason` 用可直接展示给管理员的中文 key（i18n 在 Task 11 落）。

`BuildDetectConfig` 在本地先做一遍 base URL 校验（scheme 必须 http/https、无 userinfo、无 query、无 fragment），不合格直接返回 error——SDK 虽然也会拒，但我们要在**建计划时**就拦住。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestFingerprint -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_eligibility.go backend/internal/service/fingerprint_eligibility_test.go
git commit -m "feat: 新增指纹校对的账号资格判定与检测参数映射"
```

---

### Task 4: 集群级并发闸与每日配额

**Files:**
- Create: `backend/internal/service/fingerprint_quota.go`
- Test: `backend/internal/service/fingerprint_quota_test.go`

**Interfaces:**
- Consumes: `LeaderLockCache`（`TryAcquireLeaderLock` / `ReleaseLeaderLock`）、`FingerprintSettings`
- Produces: `FingerprintQuota`、`(*FingerprintQuota).Acquire(ctx, accountID int64) (release func(), skipReason string)`、跳过原因常量 `FingerprintSkipConcurrency` / `FingerprintSkipDailyGlobal` / `FingerprintSkipDailyAccount`

- [ ] **Step 1: 写失败的测试**

```go
func TestQuotaConcurrencyGateIsClusterWide(t *testing.T) {
	// 并发闸上限 2：第 3 次 Acquire 返回 skipReason=FingerprintSkipConcurrency
	// 闸必须经由 LeaderLockCache（Redis），不能是进程内计数——用 fake cache 断言 key 被访问
}

func TestQuotaReleaseFreesSlot(t *testing.T) { /* release 后能再次 Acquire */ }

// Review Focus #5
func TestQuotaDailyLimitBlocksRunawayCron(t *testing.T) {
	// 全局上限 5：第 6 次 Acquire 返回 FingerprintSkipDailyGlobal
	// 单账号上限 2：同一 accountID 第 3 次返回 FingerprintSkipDailyAccount
	// 即使 cron 是 "* * * * *" 也必须拦住
}

func TestQuotaDailyCounterKeyRollsOverByDate(t *testing.T) { /* key 含 UTC 日期 */ }
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestQuota -v`
Expected: FAIL

- [ ] **Step 3: 实现 `fingerprint_quota.go`**

并发闸用 `LeaderLockCache` 的槽位 key（`fingerprint:slot:<i>`，`i` 从 0 到 `MaxConcurrency-1`，逐个 `TryAcquireLeaderLock`，抢到哪个用哪个，TTL 略大于单次检测超时）。每日计数用 Redis 计数器 key（含 UTC 日期，便于自然过期）。

`Acquire` 返回的 `skipReason` 非空时 `release` 为 no-op，调用方据此写一条 `verdict=error` + `skip_reason` 的结果记录——**不能静默跳过**。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestQuota -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_quota.go backend/internal/service/fingerprint_quota_test.go
git commit -m "feat: 新增指纹校对的集群级并发闸与每日配额"
```

---

### Task 5: 判定四态状态机

**Files:**
- Create: `backend/internal/service/fingerprint_verdict.go`
- Test: `backend/internal/service/fingerprint_verdict_test.go`

**Interfaces:**
- Consumes: `*lmfpd.DetectResult`、`FingerprintPlan.ExpectedModel`
- Produces: `EvaluateFingerprint(res *lmfpd.DetectResult, err error, expectedModel string, knownModels map[string]bool) FingerprintResult`

- [ ] **Step 1: 写失败的测试**

```go
func TestEvaluateFingerprintVerdicts(t *testing.T) {
	for _, tt := range []struct{ name string; decision, prediction string; wantVerdict string }{
		{"相符", "scored", "claude-opus-5", FingerprintVerdictMatch},
		{"不符", "scored", "deepseek-v4-flash", FingerprintVerdictMismatch},
		{"同家族降级也算不符", "scored", "claude-haiku-5.5", FingerprintVerdictMismatch},
		{"partial 有排名仍判定", "partial", "deepseek-v4-flash", FingerprintVerdictMismatch},
		{"unscorable 无法判定", "unscorable", "", FingerprintVerdictInconclusive},
		{"not_model 无法判定", "not_model", "", FingerprintVerdictInconclusive},
	} { /* expectedModel 固定 "claude-opus-5" */ }
}

// Review Focus #1
func TestEvaluateFingerprintTreatsTransportFailureAsError(t *testing.T) {
	// DetectResult.Error 非 nil（含 "did not return valid JSON or SSE"）→ verdict=error，绝不是 mismatch
	// Detect 直接返回 err（401/429/超时）→ verdict=error
}

// Review Focus #3
func TestEvaluateFingerprintInconclusiveWhenExpectedModelLeftTheBank(t *testing.T) {
	// knownModels 里没有 expectedModel → verdict=inconclusive，ErrorMessage 提示重选预期模型
	// 绝不能因为参考库重训就把账号判成造假
}

func TestEvaluateFingerprintCapturesFullAnalysis(t *testing.T) {
	// AnalysisJSON 含全部候选排名与 probability_status；Probability 在 partial 下为 nil
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestEvaluateFingerprint -v`
Expected: FAIL

- [ ] **Step 3: 实现 `EvaluateFingerprint`**

判定顺序：传输层 err / `res.Error` 非 nil → `error`；`expectedModel` 不在 `knownModels` → `inconclusive`；`decision` 为 `unscorable` / `not_model` → `inconclusive`；`Prediction == expectedModel` → `match`；否则 `mismatch`。

`AnalysisJSON` 序列化整个 `*lmfpd.Analysis`。`EstimatedOutputTokens` 按样本里解析到的整数个数估算。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestEvaluateFingerprint -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_verdict.go backend/internal/service/fingerprint_verdict_test.go
git commit -m "feat: 新增指纹判定四态状态机"
```

---

### Task 6: Service CRUD、结果留存与连续计数

**Files:**
- Create: `backend/internal/service/fingerprint_service.go`
- Test: `backend/internal/service/fingerprint_service_test.go`

**Interfaces:**
- Consumes: Task 1 的两个仓储接口
- Produces: `FingerprintService` 及 `CreatePlan` / `GetPlan` / `ListPlansByAccount` / `UpdatePlan` / `DeletePlan` / `ListResults` / `SaveResult(ctx, plan *FingerprintPlan, result *FingerprintResult) error`

- [ ] **Step 1: 写失败的测试**

```go
func TestSaveResultPrunesToMaxResults(t *testing.T) { /* 超出 MaxResults 调用 PruneOldResults */ }

func TestSaveResultMaintainsConsecutiveMismatchCounter(t *testing.T) {
	for _, tt := range []struct{ name string; start int; verdict string; want int }{
		{"不符累加", 1, FingerprintVerdictMismatch, 2},
		{"相符清零", 2, FingerprintVerdictMatch, 0},
		{"无法判定不动", 1, FingerprintVerdictInconclusive, 1},
		{"检测失败不动", 1, FingerprintVerdictError, 1},
	} { /* 断言 UpdateConsecutiveMismatches 的入参 */ }
}

func TestCreatePlanRejectsInvalidCron(t *testing.T) { /* 非法 cron 返回 error */ }
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestSaveResult -v`
Expected: FAIL

- [ ] **Step 3: 实现 `fingerprint_service.go`**

照 `scheduled_test_service.go` 的结构。`CreatePlan` / `UpdatePlan` 要校验 cron（复用 `scheduledTestCronParser`）、`ChallengeCount` 在 1–3、`ExpectedModel` 必须在参考库里（用 Task 2 的 `ListReferenceModels`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run "TestSaveResult|TestCreatePlan" -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_service.go backend/internal/service/fingerprint_service_test.go
git commit -m "feat: 新增指纹校对计划的 CRUD 与结果留存逻辑"
```

---

### Task 7: Runner（调度、执行、动账号动作）

**Files:**
- Create: `backend/internal/service/fingerprint_runner_service.go`
- Test: `backend/internal/service/fingerprint_runner_service_test.go`

**Interfaces:**
- Consumes: Task 2 `FingerprintDetectorProvider`、Task 3 `CheckFingerprintEligibility` / `BuildDetectConfig`、Task 4 `FingerprintQuota`、Task 5 `EvaluateFingerprint`、Task 6 `FingerprintService`、既有 `AccountRepository` / `AccountGroupMutationRepository` / `computeNextRun` / `tryAcquireSingletonLeaderLock`
- Produces: `FingerprintRunnerService`、`NewFingerprintRunnerService(...)`、`Start()` / `Stop()` / `StartPlanNow(ctx, planID int64) error`

- [ ] **Step 1: 写失败的测试**

```go
func TestRunnerAppliesAccountActionsOnlyAfterTwoMismatches(t *testing.T) {
	// 第 1 次 mismatch：不调用 updateAccountStatus / SetTempUnschedulable / RemoveFromGroup
	// 第 2 次 mismatch：全部调用
}

func TestRunnerNeverTouchesAccountOnInconclusiveOrError(t *testing.T) {
	// verdict=inconclusive / error，即便 consecutive 已达 2，也不得动账号
	// 覆盖 429/5xx/超时/参考数据加载失败/账号类型不支持
}

func TestRunnerSkipsInactiveAccountsOnSchedule(t *testing.T) {
	// 停用 / 已过期 / 临时不可调度 的账号：定期抽查直接跳过，不发任何请求
	// StartPlanNow 不受此限
}

func TestRunnerSuccessActionsDoNotEnableAccount(t *testing.T) {
	// match 时只做 AddToGroup，绝不调用 updateAccountStatus(StatusActive) 或 ClearError/SetSchedulable
}

func TestRunnerRecordsSkipReasonWhenQuotaExhausted(t *testing.T) {
	// 配额用尽：写一条 verdict=error + skip_reason 的结果，不静默跳过
}

// Review Focus #4
func TestRunnerErrorMessageNeverLeaksCredentials(t *testing.T) {
	// 构造带 api_key 的账号让 detect 失败，断言落库的 ErrorMessage 不含该 key
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestRunner -v`
Expected: FAIL

- [ ] **Step 3: 实现 `fingerprint_runner_service.go`**

结构照 `scheduled_test_runner_service.go`：`Start()` 用 `cron.New(cron.WithParser(scheduledTestCronParser), ...)` 挂 `"* * * * *"` tick；`runScheduled()` → `ListDue` → 信号量并发 → `runOnePlan()` → `acquirePlanLock`（key 前缀 `fingerprint:plan:`）→ `executePlan()` → `computeNextRun` + `UpdateAfterRun`。`StartPlanNow` 照搬 `manualRuns` 去重 + 后台 goroutine 的写法。

`executePlan` 顺序：查账号 → 资格判定（不支持 → 写 `error` 结果并返回）→ 定期抽查时检查账号活跃状态（非活跃 → 跳过）→ `quota.Acquire`（被拦 → 写带 `skip_reason` 的结果）→ `BuildDetectConfig` → `detector.Detect` → `EvaluateFingerprint` → `SaveResult` → 按 verdict 决定动作。

动账号的动作只在 `verdict == mismatch` **且** `plan.ConsecutiveMismatches >= plan.MismatchActionThreshold` 时执行。成功动作只含 `AddToGroup`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestRunner -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_runner_service.go backend/internal/service/fingerprint_runner_service_test.go
git commit -m "feat: 新增指纹校对调度执行器，动账号动作需连续两次不符"
```

---

### Task 8: ops 告警内置规则与事件

**Files:**
- Create: `backend/internal/service/fingerprint_alert.go`
- Modify: `backend/internal/service/fingerprint_runner_service.go`（判定后触发告警）
- Test: `backend/internal/service/fingerprint_alert_test.go`

**Interfaces:**
- Consumes: `OpsService`（`CreateAlertEvent` / `IsAlertSilenced` / `GetLatestAlertEvent`）、`OpsAlertRule`、`OpsAlertEvent`
- Produces: `FingerprintAlerter`、`(*FingerprintAlerter).NotifyMismatch(ctx, plan *FingerprintPlan, account *Account, result *FingerprintResult) error`、内置规则常量 `FingerprintAlertMetricType = "fingerprint_mismatch"`

- [ ] **Step 1: 写失败的测试**

```go
func TestNotifyMismatchFiresOnFirstMismatch(t *testing.T) {
	// 单次 mismatch 即发事件（告警不等连续 2 次），Dimensions 含 account_id / expected_model / predicted_model
}

func TestNotifyMismatchRespectsSilence(t *testing.T) { /* IsAlertSilenced 为真 → 不发事件 */ }

func TestNotifyMismatchSurvivesMonitoringDisabled(t *testing.T) {
	// ops 监控总开关关闭：CreateAlertEvent 返回 monitoring-disabled 错误
	// NotifyMismatch 必须吞掉它并返回 nil —— 账号列表标记（Task 6 已落库）不受影响，不能因此让整个 executePlan 失败
}

// Review Focus #4
func TestNotifyMismatchDimensionsNeverIncludeCredentials(t *testing.T) { /* dimensions 不含 api_key / base_url 凭证 */ }

func TestNotifyMismatchOnlyForMismatch(t *testing.T) { /* match / inconclusive / error 不发事件 */ }
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestNotifyMismatch -v`
Expected: FAIL

- [ ] **Step 3: 实现 `fingerprint_alert.go`**

启动时确保存在一条内置 `OpsAlertRule`（`MetricType = "fingerprint_mismatch"`，`Operator` / `Threshold` 填事件驱动的占位值，`Severity` 默认 `warning`，`NotifyEmail` 取 `fingerprint_notify_email` 设置，默认 `false`），这样静默、冷却、邮件、严重级别全部复用既有机制。

`NotifyMismatch` 在 `verdict == mismatch` 时发 `OpsAlertEvent`，`Title` / `Description` 写明账号、预期模型、实判模型、概率。监控关闭或发送失败只记日志、返回 nil。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestNotifyMismatch -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/fingerprint_alert.go backend/internal/service/fingerprint_runner_service.go backend/internal/service/fingerprint_alert_test.go
git commit -m "feat: 指纹不符接入 ops 告警，支持静默与邮件开关"
```

---

### Task 9: Handler、路由与依赖装配

**Files:**
- Create: `backend/internal/handler/admin/fingerprint_handler.go`
- Modify: `backend/internal/server/routes/admin.go`（新增 `registerFingerprintRoutes`，在 `registerScheduledTestRoutes` 之后调用）
- Modify: `backend/internal/handler/wire.go`, `backend/internal/handler/handler.go`, `backend/internal/service/wire.go`, `backend/internal/repository/wire.go`
- Test: `backend/internal/handler/admin/fingerprint_handler_test.go`

**Interfaces:**
- Produces: REST 端点
  - `GET /admin/fingerprint/reference-models` → `[]FingerprintRefModel`
  - `GET /admin/accounts/:id/fingerprint-plans` → `[]FingerprintPlan`
  - `GET /admin/accounts/:id/fingerprint-eligibility` → `FingerprintEligibility`
  - `POST /admin/fingerprint-plans`、`PUT /admin/fingerprint-plans/:id`、`DELETE /admin/fingerprint-plans/:id`
  - `GET /admin/fingerprint-plans/:id/results`、`POST /admin/fingerprint-plans/:id/run`

- [ ] **Step 1: 写失败的测试**

```go
func TestCreateFingerprintPlanRejectsUnknownExpectedModel(t *testing.T) { /* 400 + 可读消息 */ }
func TestCreateFingerprintPlanRejectsIneligibleAccount(t *testing.T) { /* OAuth 账号 → 400 + Reason */ }
func TestRunNowReturnsConflictWhenAlreadyRunning(t *testing.T) { /* 409 */ }
func TestListResultsNeverReturnsCredentials(t *testing.T) { /* 响应体不含 api_key */ }
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/admin/ -run TestFingerprint -v`
Expected: FAIL

- [ ] **Step 3: 写 handler 并注册路由与依赖**

handler 方法照 `scheduled_test_handler.go` 的签名与错误处理风格。路由照 `registerScheduledTestRoutes` 的分组写法。wire 文件里按既有顺序补齐 repo → service → runner → handler 的装配，并在服务启动处调用 `FingerprintRunner.Start()` / `Stop()`（与 `ScheduledTestRunner` 同处）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/admin/ -run TestFingerprint -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/handler/admin/fingerprint_handler.go backend/internal/handler/admin/fingerprint_handler_test.go backend/internal/server/routes/admin.go backend/internal/handler/wire.go backend/internal/handler/handler.go backend/internal/service/wire.go backend/internal/repository/wire.go
git commit -m "feat: 新增指纹校对的管理端接口与路由装配"
```

---

### Task 10: 前端 API、类型与指纹面板

**Files:**
- Create: `frontend/src/api/admin/fingerprint.ts`
- Modify: `frontend/src/api/admin/index.ts`（挂 `fingerprint`）
- Modify: `frontend/src/types/`（新增 `FingerprintPlan` / `FingerprintResult` / `FingerprintRefModel` / `FingerprintEligibility` 及请求类型，位置照 `ScheduledTestPlan` 所在文件）
- Create: `frontend/src/components/admin/account/FingerprintPanel.vue`
- Test: `frontend/src/components/admin/account/__tests__/FingerprintPanel.spec.ts`

**Interfaces:**
- Consumes: Task 9 的全部端点
- Produces: `fingerprintAPI`（`listByAccount` / `eligibility` / `referenceModels` / `create` / `update` / `delete` / `listResults` / `runNow`）、`FingerprintPanel.vue`（props `accountId` / `accountPlatform` / `accountGroupIds` / `groups`）

> 新建独立面板，**不改 `ScheduledTestsPanel.vue`**（已 928 行，再塞一种校验类型会让两种语义都难读）。

- [ ] **Step 1: 写失败的测试**

照 `ScheduledTestsPanel.groups.spec.ts` 的 `vi.hoisted` + `vi.mock('@/api/admin', ...)` + `shallowMount` 写法。

```ts
it('账号不支持时禁用新建并显示原因', async () => { /* eligibility.supported=false → 表单禁用 + Reason 文案 */ })
it('预期模型下拉只来自参考库', async () => { /* 选项数量与 referenceModels 一致，且不可自由输入 */ })
it('配了代理的账号显示结论仅供参考的提示', async () => { /* 规格 5.3.1 要求该提示出现在管理员看结果的地方 */ })
it('四态结果各自显示不同样式与原因', async () => { /* match/mismatch/inconclusive/error；后两者显示 error_message 或 skip_reason */ })
it('展示完整候选排名与置信度', async () => { /* 从 analysis_json 渲染 results[] */ })
it('显示每次检测的消耗与累计消耗', async () => { /* 规格 6.9：单条显示 estimated_output_tokens，面板顶部显示本月累计 */ })
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm vitest run src/components/admin/account/__tests__/FingerprintPanel.spec.ts`（working dir `frontend/`）
Expected: FAIL

- [ ] **Step 3: 写 `fingerprint.ts` 与类型**

照 `scheduledTests.ts` 的结构（具名导出 + 聚合对象 + default export）。

- [ ] **Step 4: 写 `FingerprintPanel.vue`**

复用 `@/components/common/` 下既有的 `Input` / `Select` / 按钮等组件，**不新增任何第三方依赖**。计划列表、新建/编辑表单（预期模型下拉、测试模型输入并注明"填映射后的最终模型名"、cron、挑战数 1–3、失败动作开关）、结果列表（四态样式 + 展开看完整排名 + 单条消耗）、面板顶部的本月累计消耗、手动触发按钮。账号不支持时整体禁用并显示 `eligibility.reason`。

- [ ] **Step 5: 运行测试确认通过**

Run: `pnpm vitest run src/components/admin/account/__tests__/FingerprintPanel.spec.ts`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add frontend/src/api/admin/fingerprint.ts frontend/src/api/admin/index.ts frontend/src/types frontend/src/components/admin/account/FingerprintPanel.vue frontend/src/components/admin/account/__tests__/FingerprintPanel.spec.ts
git commit -m "feat: 新增指纹校对前端接口与管理面板"
```

---

### Task 11: 账号列表标记、入口、创建勾选与 i18n

**Files:**
- Modify: `frontend/src/components/admin/account/AccountActionMenu.vue`（加「指纹校对」入口）
- Modify: `frontend/src/views/admin/AccountsView.vue`（指纹标记列，挂载 `FingerprintPanel`）
- Modify: `frontend/src/components/account/CreateAccountModal.vue`（勾选项 + 预期模型下拉）
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`, `frontend/src/i18n/locales/en/admin/accounts.ts`
- Modify: `CLAUDE_DEV.md`（功能清单加一行）
- Test: `frontend/src/views/admin/__tests__/AccountsView.fingerprintBadge.spec.ts`

**Interfaces:**
- Consumes: Task 10 的 `fingerprintAPI` 与 `FingerprintPanel.vue`

- [ ] **Step 1: 写失败的测试**

```ts
it('不符时账号行显示指纹存疑标记', async () => { /* 悬停含实判模型与时间 */ })
it('无法判定与检测失败显示各自原因而非存疑', async () => { /* 规格反目标 4：不得沉默，也不得误报为造假 */ })
it('标记随最近一次结果自动更新', async () => { /* 由 match 转为无标记，无需手动确认 */ })
it('批量创建不提供接入时校验勾选', async () => { /* 规格 6.2 */ })
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm vitest run src/views/admin/__tests__/AccountsView.fingerprintBadge.spec.ts`
Expected: FAIL

- [ ] **Step 3: 改三个前端文件**

`AccountActionMenu.vue` 照既有「定时测试」项的写法加「指纹校对」。`AccountsView.vue` 加标记列（四态各自样式）并挂载面板。`CreateAccountModal.vue` 加「创建后立即做指纹校对」勾选（默认不勾），勾选后展开预期模型下拉；创建成功后异步建计划并触发一次，结果用 toast + 列表标记呈现，**弹窗不等检测**。批量创建/导入流程不加此勾选。

- [ ] **Step 4: 补 i18n**

中英文同步，覆盖四态文案、资格不支持原因、代理提示、字段标签与帮助文字。

- [ ] **Step 5: 运行测试确认通过**

Run: `pnpm vitest run src/views/admin/__tests__/AccountsView.fingerprintBadge.spec.ts`
Expected: PASS

- [ ] **Step 6: 在 `CLAUDE_DEV.md` 功能清单加一行**

日期 2026-11-04、功能「账号指纹校对」、说明、主要文件、Commit。

- [ ] **Step 7: 提交**

```bash
git add frontend/src/components/admin/account/AccountActionMenu.vue frontend/src/views/admin/AccountsView.vue frontend/src/components/account/CreateAccountModal.vue frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts frontend/src/views/admin/__tests__/AccountsView.fingerprintBadge.spec.ts CLAUDE_DEV.md
git commit -m "feat: 账号列表接入指纹校对入口、状态标记与创建时校验"
```
