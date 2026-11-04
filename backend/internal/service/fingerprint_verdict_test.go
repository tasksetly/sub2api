package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

func fingerprintKnownModels(ids ...string) []FingerprintRefModel {
	out := make([]FingerprintRefModel, 0, len(ids))
	for _, id := range ids {
		family := strings.Split(id, "-")[0]
		out = append(out, FingerprintRefModel{ID: id, DisplayName: id, Family: family, FamilyName: family})
	}
	return out
}

func fingerprintDetectResult(decision, prediction string) *lmfpd.DetectResult {
	prob := 0.87
	return &lmfpd.DetectResult{
		Analysis: &lmfpd.Analysis{
			Decision:             decision,
			Prediction:           prediction,
			PredictionName:       prediction,
			FamilyPrediction:     strings.Split(prediction, "-")[0],
			FamilyPredictionName: strings.Split(prediction, "-")[0],
			Probability:          &prob,
			ProbabilityStatus:    "reference_calibrated",
			Results: []lmfpd.AnalysisResult{
				{Model: prediction, DisplayName: prediction, Score: 1.2, Probability: &prob},
			},
		},
		Samples: []lmfpd.Sample{
			{State: "complete", RawText: strings.Repeat("123, ", 300), Count: 300, ExpectedCount: 300},
		},
	}
}

func TestEvaluateFingerprintVerdicts(t *testing.T) {
	known := fingerprintKnownModels("claude-opus-5", "claude-haiku-5.5", "deepseek-v4-flash")

	for _, tt := range []struct {
		name       string
		decision   string
		prediction string
		want       string
	}{
		{"相符", "scored", "claude-opus-5", FingerprintVerdictMatch},
		{"跨家族不符", "scored", "deepseek-v4-flash", FingerprintVerdictMismatch},
		{"同家族降级也算不符", "scored", "claude-haiku-5.5", FingerprintVerdictMismatch},
		{"partial 有排名仍判定", "partial", "deepseek-v4-flash", FingerprintVerdictMismatch},
		{"partial 相符", "partial", "claude-opus-5", FingerprintVerdictMatch},
		{"unscorable 无法判定", "unscorable", "", FingerprintVerdictInconclusive},
		{"not_model 无法判定", "not_model", "", FingerprintVerdictInconclusive},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateFingerprint(fingerprintDetectResult(tt.decision, tt.prediction), nil, "claude-opus-5", known)
			if got.Verdict != tt.want {
				t.Fatalf("Verdict = %q, want %q", got.Verdict, tt.want)
			}
			if got.Decision != tt.decision {
				t.Fatalf("Decision = %q, want %q", got.Decision, tt.decision)
			}
		})
	}
}

// Review Focus #1：上游返回 HTML 错误页、非法 JSON、401/429/5xx、超时，
// 都必须落 error 态。当成 mismatch 会让一次限流就停掉好账号。
func TestEvaluateFingerprintTreatsTransportFailureAsError(t *testing.T) {
	known := fingerprintKnownModels("claude-opus-5")

	t.Run("detect 直接返回错误", func(t *testing.T) {
		got := EvaluateFingerprint(nil, errors.New("429 Too Many Requests"), "claude-opus-5", known)
		if got.Verdict != FingerprintVerdictError {
			t.Fatalf("Verdict = %q, want %q", got.Verdict, FingerprintVerdictError)
		}
		if got.ErrorMessage == "" {
			t.Fatal("必须保留错误原因，否则界面上是沉默失败")
		}
	})

	t.Run("非法响应体", func(t *testing.T) {
		msg := "The server did not return valid JSON or SSE."
		res := &lmfpd.DetectResult{Error: &msg}
		got := EvaluateFingerprint(res, nil, "claude-opus-5", known)
		if got.Verdict != FingerprintVerdictError {
			t.Fatalf("Verdict = %q, want %q", got.Verdict, FingerprintVerdictError)
		}
		if !strings.Contains(got.ErrorMessage, "valid JSON") {
			t.Fatalf("ErrorMessage = %q, 应保留 SDK 的原因", got.ErrorMessage)
		}
	})

	t.Run("结果为空", func(t *testing.T) {
		got := EvaluateFingerprint(nil, nil, "claude-opus-5", known)
		if got.Verdict != FingerprintVerdictError {
			t.Fatalf("Verdict = %q, want %q", got.Verdict, FingerprintVerdictError)
		}
	})
}

// Review Focus #3：参考库重训后预期模型可能消失。
// 封闭集合会给它硬配一个"最像"的候选，放任不管就是每轮都判不符的永久误报。
func TestEvaluateFingerprintInconclusiveWhenExpectedModelLeftTheBank(t *testing.T) {
	known := fingerprintKnownModels("claude-opus-5.5", "deepseek-v4-flash")

	got := EvaluateFingerprint(fingerprintDetectResult("scored", "deepseek-v4-flash"), nil, "claude-opus-5", known)
	if got.Verdict != FingerprintVerdictInconclusive {
		t.Fatalf("Verdict = %q, want %q（参考库里已无该预期模型）", got.Verdict, FingerprintVerdictInconclusive)
	}
	if got.ErrorMessage == "" {
		t.Fatal("应提示管理员重选预期模型")
	}
}

func TestEvaluateFingerprintSkipsBankCheckWhenCatalogUnavailable(t *testing.T) {
	// 清单拉取失败时传 nil：不能因为拿不到清单就把所有账号判成无法判定。
	got := EvaluateFingerprint(fingerprintDetectResult("scored", "claude-opus-5"), nil, "claude-opus-5", nil)
	if got.Verdict != FingerprintVerdictMatch {
		t.Fatalf("Verdict = %q, want %q", got.Verdict, FingerprintVerdictMatch)
	}
}

func TestEvaluateFingerprintCapturesFullAnalysis(t *testing.T) {
	known := fingerprintKnownModels("claude-opus-5", "deepseek-v4-flash")
	got := EvaluateFingerprint(fingerprintDetectResult("scored", "deepseek-v4-flash"), nil, "claude-opus-5", known)

	if got.PredictedModel != "deepseek-v4-flash" {
		t.Fatalf("PredictedModel = %q", got.PredictedModel)
	}
	if got.PredictedFamily != "deepseek" {
		t.Fatalf("PredictedFamily = %q", got.PredictedFamily)
	}
	if got.Probability == nil || *got.Probability != 0.87 {
		t.Fatalf("Probability = %v, want 0.87", got.Probability)
	}
	if got.AnalysisJSON == "" {
		t.Fatal("必须保留完整 analysis 供人工复核候选排名")
	}

	var decoded lmfpd.Analysis
	if err := json.Unmarshal([]byte(got.AnalysisJSON), &decoded); err != nil {
		t.Fatalf("AnalysisJSON 不是合法 JSON: %v", err)
	}
	if len(decoded.Results) != 1 || decoded.ProbabilityStatus != "reference_calibrated" {
		t.Fatalf("analysis 丢失了候选排名或校准状态: %+v", decoded)
	}
}

func TestEvaluateFingerprintLeavesProbabilityNilWithoutCalibration(t *testing.T) {
	res := fingerprintDetectResult("partial", "deepseek-v4-flash")
	res.Analysis.Probability = nil
	res.Analysis.ProbabilityStatus = "unavailable"

	got := EvaluateFingerprint(res, nil, "claude-opus-5", fingerprintKnownModels("claude-opus-5", "deepseek-v4-flash"))
	if got.Probability != nil {
		t.Fatalf("Probability = %v, want nil", got.Probability)
	}
	if got.Verdict != FingerprintVerdictMismatch {
		t.Fatalf("Verdict = %q, 没有置信度也应按排名判定", got.Verdict)
	}
}

func TestEstimateFingerprintOutputTokens(t *testing.T) {
	samples := []lmfpd.Sample{
		{RawText: strings.Repeat("a", 400), Count: 100},
		{RawText: "", Count: 50},
	}
	got := estimateFingerprintOutputTokens(samples)
	if got <= 0 {
		t.Fatalf("estimate = %d, want > 0", got)
	}
	// 400 字符 ≈ 100 token，加上无文本样本按整数个数兜底的 50×3。
	if got != 100+150 {
		t.Fatalf("estimate = %d, want %d", got, 250)
	}
}

func TestEvaluateFingerprintErrorMessageNeverEmptyOnFailure(t *testing.T) {
	got := EvaluateFingerprint(&lmfpd.DetectResult{}, nil, "claude-opus-5", nil)
	if got.Verdict != FingerprintVerdictError {
		t.Fatalf("Verdict = %q, want %q", got.Verdict, FingerprintVerdictError)
	}
	if strings.TrimSpace(got.ErrorMessage) == "" {
		t.Fatal("error 态必须带原因")
	}
}
