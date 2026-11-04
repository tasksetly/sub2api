package service

import (
	"fmt"
	"net/url"
	"strings"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

// 资格判定的原因码。这些是 i18n key，前端据此展示可读文案。
const (
	FingerprintReasonAccountMissing   = "account_missing"
	FingerprintReasonUnsupportedType  = "unsupported_account_type"
	FingerprintReasonHeaderOverrides  = "header_overrides_unsupported"
	FingerprintReasonMissingBaseURL   = "missing_base_url"
	FingerprintReasonMissingAPIKey    = "missing_api_key"
	FingerprintReasonDetectorUnloaded = "reference_data_unavailable"
)

// CheckFingerprintEligibility 判断一个账号能否做指纹校对。
//
// 方案 A 用 SDK 的 detect() 直连上游，需要 base_url + api_key，因此只覆盖
// upstream / apikey 两类账号。OAuth、setup-token、bedrock、service_account
// 的凭证形态够不着，界面上要置灰并写明原因，不能让人误以为是功能故障。
func CheckFingerprintEligibility(a *Account) FingerprintEligibility {
	if a == nil {
		return FingerprintEligibility{Reason: FingerprintReasonAccountMissing}
	}

	switch a.Type {
	case AccountTypeUpstream, AccountTypeAPIKey:
		// 继续往下检查。
	default:
		return FingerprintEligibility{Reason: FingerprintReasonUnsupportedType}
	}

	// 配了自定义请求头的账号，SDK 自建的请求不会带上它们，大概率直接失败。
	if hasActiveHeaderOverrides(a) {
		return FingerprintEligibility{Reason: FingerprintReasonHeaderOverrides}
	}

	if strings.TrimSpace(a.GetCredential("base_url")) == "" {
		return FingerprintEligibility{Reason: FingerprintReasonMissingBaseURL}
	}
	if strings.TrimSpace(a.GetCredential("api_key")) == "" {
		return FingerprintEligibility{Reason: FingerprintReasonMissingAPIKey}
	}

	// 配了代理也支持检测，但 SDK 的 HTTP 客户端不可注入（sample.go 的 sampleClient
	// 是包级私有变量，只认进程级 HTTPS_PROXY/HTTP_PROXY/NO_PROXY），检测不会经过
	// 该账号的代理。出口 IP 与业务不同，上游理论上可按 IP 区别对待，所以结论要带提示。
	return FingerprintEligibility{
		Supported:     true,
		ProxyBypassed: a.ProxyID != nil && *a.ProxyID > 0,
	}
}

// hasActiveHeaderOverrides 报告账号是否启用了会实际生效的自定义请求头覆盖。
//
// 直接复用 GetHeaderOverrides：它处理了非 JSON 反序列化形态（直接注入的 map[string]string），
// 并过滤掉空值与黑名单头。自己重写一遍会在两个方向上都判错——
// map[string]string 形态漏网，只配了被禁头的账号被误伤置灰。
func hasActiveHeaderOverrides(a *Account) bool {
	return len(a.GetHeaderOverrides()) > 0
}

// BuildDetectConfig 把账号和计划翻译成 SDK 的检测参数。
//
// base_url 在这里就做一遍校验：SDK 虽然也会在发请求前拒绝非法 URL，
// 但那要等到抽查时才炸；建计划时拦住才能给管理员可读的提示。
// 任何错误信息都不得包含凭证。
func BuildDetectConfig(a *Account, plan FingerprintPlan) (lmfpd.DetectConfig, error) {
	var cfg lmfpd.DetectConfig

	eligibility := CheckFingerprintEligibility(a)
	if !eligibility.Supported {
		return cfg, fmt.Errorf("账号不支持指纹校对: %s", eligibility.Reason)
	}

	model := strings.TrimSpace(plan.TestModel)
	if model == "" {
		return cfg, fmt.Errorf("测试模型不能为空")
	}

	baseURL := strings.TrimSpace(a.GetCredential("base_url"))
	if err := validateFingerprintBaseURL(baseURL); err != nil {
		return cfg, err
	}

	count := plan.ChallengeCount
	if count < 1 || count > 3 {
		count = FingerprintDefaultChallengeCount
	}

	cfg = lmfpd.DetectConfig{
		BaseURL:  baseURL,
		APIKey:   strings.TrimSpace(a.GetCredential("api_key")),
		Model:    model,
		API:      resolveFingerprintAPI(plan.API, a.GetAPIProtocol()),
		Count:    count,
		Parallel: count,
	}
	return cfg, nil
}

// validateFingerprintBaseURL 复刻 SDK 的 base URL 规则，提前给出可读错误。
// 错误信息只描述问题，绝不回显 URL 本身——它可能内嵌凭证。
func validateFingerprintBaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("base_url 未配置")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("base_url 不是合法的 URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base_url 必须以 http:// 或 https:// 开头")
	}
	if parsed.Host == "" {
		return fmt.Errorf("base_url 缺少主机名")
	}
	if parsed.User != nil {
		return fmt.Errorf("base_url 不能内嵌用户名或密码")
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("base_url 不能带查询参数")
	}
	if parsed.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("base_url 不能带 # 片段")
	}
	return nil
}

// resolveFingerprintAPI 决定用哪种上游协议发检测请求。
// 计划里显式指定的优先；否则按账号自身的协议推导，未知协议回落到 chat_completions。
func resolveFingerprintAPI(planAPI string, accountProtocol string) lmfpd.API {
	if IsValidFingerprintAPI(planAPI) {
		return lmfpd.API(planAPI)
	}
	switch accountProtocol {
	case APIProtocolAnthropic:
		return lmfpd.APIMessages
	case APIProtocolResponses:
		return lmfpd.APIResponses
	default:
		return lmfpd.APIChatCompletions
	}
}
