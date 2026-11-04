package service

import (
	"context"
	"testing"
	"time"
)

type fingerprintSettingRepoStub struct {
	values map[string]string
}

func (s *fingerprintSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *fingerprintSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *fingerprintSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *fingerprintSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *fingerprintSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *fingerprintSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *fingerprintSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func newFingerprintSettingsForTest(t *testing.T, values map[string]string) FingerprintSettings {
	t.Helper()
	svc := &SettingService{settingRepo: &fingerprintSettingRepoStub{values: values}}
	return svc.GetFingerprintSettings(context.Background())
}

func TestFingerprintSettingsFallBackToSpecDefaults(t *testing.T) {
	s := newFingerprintSettingsForTest(t, map[string]string{})

	want := []string{
		"https://lm.ikale.io/data/",
		"https://cdn.jsdmirror.com/npm/lmfpd-data@latest/",
	}
	if len(s.DataSources) != len(want) {
		t.Fatalf("sources = %v, want %v", s.DataSources, want)
	}
	for i := range want {
		if s.DataSources[i] != want[i] {
			t.Fatalf("sources[%d] = %q, want %q", i, s.DataSources[i], want[i])
		}
	}
	if s.MaxConcurrency != FingerprintDefaultMaxConcurrency {
		t.Fatalf("concurrency = %d, want %d", s.MaxConcurrency, FingerprintDefaultMaxConcurrency)
	}
	if s.DailyLimitGlobal != FingerprintDefaultDailyLimitGlobal {
		t.Fatalf("daily global = %d, want %d", s.DailyLimitGlobal, FingerprintDefaultDailyLimitGlobal)
	}
	if s.DailyLimitAccount != FingerprintDefaultDailyLimitAccount {
		t.Fatalf("daily account = %d, want %d", s.DailyLimitAccount, FingerprintDefaultDailyLimitAccount)
	}
	if s.CacheTTL != FingerprintDefaultCacheTTLHours*time.Hour {
		t.Fatalf("ttl = %v, want %v", s.CacheTTL, FingerprintDefaultCacheTTLHours*time.Hour)
	}
	if s.NotifyEmail {
		t.Fatal("email must default to off")
	}
}

func TestFingerprintSettingsParseJSONDataSources(t *testing.T) {
	s := newFingerprintSettingsForTest(t, map[string]string{
		SettingKeyFingerprintDataSources: `["https://mirror.internal/lmfpd/"]`,
	})
	if len(s.DataSources) != 1 || s.DataSources[0] != "https://mirror.internal/lmfpd/" {
		t.Fatalf("sources = %v", s.DataSources)
	}
}

func TestFingerprintSettingsRejectNonPositiveLimits(t *testing.T) {
	s := newFingerprintSettingsForTest(t, map[string]string{
		SettingKeyFingerprintMaxConcurrency:    "0",
		SettingKeyFingerprintDailyLimitGlobal:  "-5",
		SettingKeyFingerprintDailyLimitAccount: "abc",
	})
	if s.MaxConcurrency != FingerprintDefaultMaxConcurrency {
		t.Fatalf("concurrency = %d, want fallback %d", s.MaxConcurrency, FingerprintDefaultMaxConcurrency)
	}
	if s.DailyLimitGlobal != FingerprintDefaultDailyLimitGlobal {
		t.Fatalf("daily global = %d, want fallback %d", s.DailyLimitGlobal, FingerprintDefaultDailyLimitGlobal)
	}
	if s.DailyLimitAccount != FingerprintDefaultDailyLimitAccount {
		t.Fatalf("daily account = %d, want fallback %d", s.DailyLimitAccount, FingerprintDefaultDailyLimitAccount)
	}
}

func TestFingerprintLocalDirectorySourceSkipsNetwork(t *testing.T) {
	opts := buildFingerprintLoadOptions(FingerprintSettings{
		DataSources: []string{"/srv/lmfpd-data"},
		CacheTTL:    time.Hour,
	})
	if len(opts.Sources) != 1 || opts.Sources[0] != "/srv/lmfpd-data" {
		t.Fatalf("sources = %v, want the local directory passed through untouched", opts.Sources)
	}
	if opts.Cache != "" {
		t.Fatalf("cache = %q, want empty so the SDK skips cache reads for a local directory", opts.Cache)
	}
}

func TestIsFingerprintLocalDirectorySource(t *testing.T) {
	for _, tt := range []struct {
		sources []string
		want    bool
	}{
		{[]string{"/srv/lmfpd-data"}, true},
		{[]string{"./data"}, true},
		{[]string{"https://lm.ikale.io/data/"}, false},
		{[]string{"http://mirror/lmfpd/"}, false},
		{[]string{"/srv/a", "/srv/b"}, false}, // 多个来源时不按本地目录处理
		{nil, false},
	} {
		if got := isFingerprintLocalDirectorySource(tt.sources); got != tt.want {
			t.Fatalf("isFingerprintLocalDirectorySource(%v) = %v, want %v", tt.sources, got, tt.want)
		}
	}
}

func TestParseFingerprintBundleModels(t *testing.T) {
	raw := []byte(`{
		"schema": "lmfpd-data-v1",
		"built_at": "2026-11-03T12:06:39Z",
		"models": [
			{"id": "claude-opus-5", "display_name": "claude-opus-5", "family": "claude", "family_name": "Claude"},
			{"id": "deepseek-v4-flash", "display_name": "deepseek-v4-flash", "family": "deepseek", "family_name": "DeepSeek"}
		]
	}`)
	models, err := parseFingerprintBundleModels(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %d, want 2", len(models))
	}
	// 按 family_name 再按 id 排序：Claude 在 DeepSeek 之前
	if models[0].ID != "claude-opus-5" || models[0].FamilyName != "Claude" {
		t.Fatalf("models[0] = %+v", models[0])
	}
	if models[1].ID != "deepseek-v4-flash" {
		t.Fatalf("models[1] = %+v", models[1])
	}
}

func TestParseFingerprintBundleModelsRejectsGarbage(t *testing.T) {
	if _, err := parseFingerprintBundleModels([]byte("not json")); err == nil {
		t.Fatal("want error for non-JSON bundle")
	}
}
