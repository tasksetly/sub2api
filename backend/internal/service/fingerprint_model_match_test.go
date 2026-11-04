package service

import "testing"

func fingerprintBankForMatch() []FingerprintRefModel {
	ids := []string{
		"claude-opus-5", "claude-opus-5.5", "claude-haiku-5.5", "claude-sonnet-5",
		"gpt-4o", "gpt-6-astra", "gpt-6-luna",
		"deepseek-v4-flash", "gemini-3.5-flash", "qwen3.8-max-0902",
	}
	out := make([]FingerprintRefModel, 0, len(ids))
	for _, id := range ids {
		out = append(out, FingerprintRefModel{ID: id, DisplayName: id})
	}
	return out
}

func TestResolveFingerprintExpectedModel(t *testing.T) {
	bank := fingerprintBankForMatch()

	for _, tt := range []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{"完全相等", "claude-opus-5", "claude-opus-5", true},
		{"大小写无关", "Claude-Opus-5", "claude-opus-5", true},
		{"两侧空白", "  claude-opus-5  ", "claude-opus-5", true},
		{"latest 别名", "claude-opus-5-latest", "claude-opus-5", true},
		{"八位日期", "claude-opus-5-20260101", "claude-opus-5", true},
		{"连字符日期", "gpt-4o-2024-08-06", "gpt-4o", true},
		{"小数版本取最长前缀", "claude-opus-5.5-latest", "claude-opus-5.5", true},
		{"小数版本不被整数误配", "claude-opus-5.5", "claude-opus-5.5", true},
		{"带日期的小数版本", "claude-haiku-5.5-20260301", "claude-haiku-5.5", true},
		{"中转商自编后缀", "deepseek-v4-flash-turbo", "deepseek-v4-flash", true},
		{"preview 别名", "gemini-3.5-flash-preview", "gemini-3.5-flash", true},
		{"库里没有", "some-finetuned-model", "", false},
		{"前缀不完整不该命中", "gpt-6", "", false},
		{"空字符串", "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveFingerprintExpectedModel(tt.in, bank)
			if ok != tt.found {
				t.Fatalf("found = %v, want %v (got %q)", ok, tt.found, got)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// 不能在分隔符之外的位置截断，否则 gpt-5 会命中 gpt-54 这类无关模型。
func TestResolveFingerprintExpectedModelRequiresBoundary(t *testing.T) {
	bank := []FingerprintRefModel{{ID: "gpt-5"}}
	if got, ok := ResolveFingerprintExpectedModel("gpt-54", bank); ok {
		t.Fatalf("gpt-54 不应命中 gpt-5，got %q", got)
	}
	if got, ok := ResolveFingerprintExpectedModel("gpt-5-latest", bank); !ok || got != "gpt-5" {
		t.Fatalf("gpt-5-latest 应命中 gpt-5，got %q ok=%v", got, ok)
	}
}

func TestResolveFingerprintExpectedModelEmptyBank(t *testing.T) {
	if _, ok := ResolveFingerprintExpectedModel("claude-opus-5", nil); ok {
		t.Fatal("参考库为空时不应返回匹配")
	}
}

func TestStripFingerprintModelSuffixes(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"claude-opus-5-latest", "claude-opus-5"},
		{"claude-opus-5-20260101", "claude-opus-5"},
		{"claude-opus-5-2026-01-01", "claude-opus-5"},
		{"claude-opus-5-latest-20260101", "claude-opus-5"},
		{"claude-opus-5", "claude-opus-5"},
	} {
		if got := stripFingerprintModelSuffixes(tt.in); got != tt.want {
			t.Fatalf("strip(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// 实判结果不必与预期模型逐字相同：版本差异算通过，档位/家族差异仍判不符。
func TestFingerprintModelsEquivalent(t *testing.T) {
	bank := []FingerprintRefModel{
		{ID: "claude-opus-5", Family: "claude"},
		{ID: "claude-opus-5.5", Family: "claude"},
		{ID: "claude-haiku-5.5", Family: "claude"},
		{ID: "claude-sonnet-5", Family: "claude"},
		{ID: "deepseek-v4-flash", Family: "deepseek"},
		{ID: "deepseek-v4.1-flash", Family: "deepseek"},
		{ID: "deepseek-v4-pro", Family: "deepseek"},
		{ID: "gemini-3.5-flash", Family: "gemini"},
		{ID: "gemini-3.8-flash", Family: "gemini"},
		{ID: "gemini-3.5-flash-lite", Family: "gemini"},
		{ID: "gpt-6-astra", Family: "gpt"},
		{ID: "gpt-6-luna", Family: "gpt"},
	}

	for _, tt := range []struct {
		name                string
		expected, predicted string
		want                bool
	}{
		{"完全相同", "claude-opus-5", "claude-opus-5", true},
		{"小版本差异算通过", "claude-opus-5", "claude-opus-5.5", true},
		{"小版本差异反向", "claude-opus-5.5", "claude-opus-5", true},
		{"deepseek 小版本", "deepseek-v4-flash", "deepseek-v4.1-flash", true},
		{"gemini 版本跳跃", "gemini-3.5-flash", "gemini-3.8-flash", true},

		{"opus 被换成 haiku", "claude-opus-5", "claude-haiku-5.5", false},
		{"opus 被换成 sonnet", "claude-opus-5", "claude-sonnet-5", false},
		{"flash 被换成 pro", "deepseek-v4-pro", "deepseek-v4-flash", false},
		{"flash 被换成 lite", "gemini-3.5-flash", "gemini-3.5-flash-lite", false},
		{"跨家族冒充", "claude-opus-5", "deepseek-v4-flash", false},
		{"同家族不同产品线", "gpt-6-astra", "gpt-6-luna", false},

		{"空预期", "", "claude-opus-5", false},
		{"空实判", "claude-opus-5", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FingerprintModelsEquivalent(tt.expected, tt.predicted, bank); got != tt.want {
				t.Fatalf("FingerprintModelsEquivalent(%q, %q) = %v, want %v", tt.expected, tt.predicted, got, tt.want)
			}
		})
	}
}

// 参考库为空时退化成去版本号比较，仍不放过产品线差异。
func TestFingerprintModelsEquivalentWithoutBank(t *testing.T) {
	if !FingerprintModelsEquivalent("claude-opus-5", "claude-opus-5.5", nil) {
		t.Fatal("无参考库时版本差异仍应算通过")
	}
	if FingerprintModelsEquivalent("claude-opus-5", "claude-haiku-5", nil) {
		t.Fatal("无参考库时也不该把 opus 和 haiku 当成同一个")
	}
}
