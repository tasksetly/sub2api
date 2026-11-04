package service

import (
	"encoding/json"
	"strings"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

// EvaluateFingerprint 把一次检测的原始结果翻译成判定四态。
//
// 判定顺序是有意为之的：
//  1. 传输层失败 → error。上游返回 HTML 错误页、401/429/5xx、超时都在这里，
//     当成 mismatch 会让一次限流就停掉好账号（反目标第一条）。
//  2. 预期模型已不在参考库 → inconclusive。参考库是封闭集合，库里没有的模型
//     也会被硬配一个"最像"的候选，不拦住就是每轮都判不符的永久误报。
//  3. 拿不到排名（unscorable / not_model）→ inconclusive。不等于造假。
//  4. 排名第一与预期模型"对得上" → match，否则 mismatch。
//     对得上不要求逐字相同：去掉版本号后同一条产品线即可（见 FingerprintModelsEquivalent），
//     但 opus 换 haiku、claude 换 deepseek 这类降级冒充仍判不符。
//
// known 为空表示参考库清单暂不可用，此时跳过第 2 步——不能因为拉不到
// 清单就把所有账号判成无法判定。
func EvaluateFingerprint(res *lmfpd.DetectResult, err error, expectedModel string, known []FingerprintRefModel) FingerprintResult {
	out := FingerprintResult{Verdict: FingerprintVerdictError}

	if err != nil {
		out.ErrorMessage = err.Error()
		return out
	}
	if res == nil {
		out.ErrorMessage = "检测没有返回结果"
		return out
	}
	if res.Error != nil && strings.TrimSpace(*res.Error) != "" {
		out.ErrorMessage = *res.Error
		out.EstimatedOutputTokens = estimateFingerprintOutputTokens(res.Samples)
		return out
	}
	if res.Analysis == nil {
		out.ErrorMessage = "检测没有产生可用的分析结果"
		out.EstimatedOutputTokens = estimateFingerprintOutputTokens(res.Samples)
		return out
	}

	analysis := res.Analysis
	out.Decision = analysis.Decision
	out.PredictedModel = analysis.Prediction
	out.PredictedModelName = analysis.PredictionName
	out.PredictedFamily = analysis.FamilyPrediction
	out.Probability = analysis.Probability
	out.EstimatedOutputTokens = estimateFingerprintOutputTokens(res.Samples)
	if encoded, encErr := json.Marshal(analysis); encErr == nil {
		out.AnalysisJSON = string(encoded)
	}

	expected := strings.TrimSpace(expectedModel)
	if expected == "" {
		out.Verdict = FingerprintVerdictInconclusive
		out.ErrorMessage = "计划未设置预期模型"
		return out
	}
	if len(known) > 0 && !fingerprintBankContains(expected, known) {
		out.Verdict = FingerprintVerdictInconclusive
		out.ErrorMessage = "预期模型 " + expected + " 已不在参考库中，请重新选择后再判定"
		return out
	}

	if analysis.Decision == "unscorable" || analysis.Decision == "not_model" || strings.TrimSpace(analysis.Prediction) == "" {
		out.Verdict = FingerprintVerdictInconclusive
		out.ErrorMessage = fingerprintInconclusiveReason(analysis.Decision)
		return out
	}

	if FingerprintModelsEquivalent(expected, analysis.Prediction, known) {
		out.Verdict = FingerprintVerdictMatch
		return out
	}
	out.Verdict = FingerprintVerdictMismatch
	return out
}

// fingerprintBankContains 报告参考库里是否有该模型 ID。
func fingerprintBankContains(id string, known []FingerprintRefModel) bool {
	for _, m := range known {
		if m.ID == id {
			return true
		}
	}
	return false
}

// fingerprintInconclusiveReason 把 SDK 的 decision 翻成可展示的原因。
func fingerprintInconclusiveReason(decision string) string {
	switch decision {
	case "unscorable":
		return "有效回答不足，无法评分"
	case "not_model":
		return "数字的变化方式不像语言模型写的，无法评分"
	default:
		return "本次检测没有得出可用排名"
	}
}

// estimateFingerprintOutputTokens 估算一次检测消耗的输出 token。
//
// 方案 A 直连上游、不经过项目网关，这笔花费不会出现在 usage_logs 里，
// 只能自己估。按约 4 字符 1 token 估算；没拿到文本时按解析到的整数个数兜底。
func estimateFingerprintOutputTokens(samples []lmfpd.Sample) int {
	total := 0
	for _, s := range samples {
		if n := len(s.RawText); n > 0 {
			total += n / 4
			continue
		}
		if s.Count > 0 {
			total += s.Count * 3
		}
	}
	return total
}
