package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// FingerprintDefaultDataSources 是参考数据的默认来源，按顺序尝试。
// 第二个是国内镜像，内容来自 npm 包 lmfpd-data。
var FingerprintDefaultDataSources = []string{
	"https://lm.ikale.io/data/",
	"https://cdn.jsdmirror.com/npm/lmfpd-data@latest/",
}

// FingerprintSettings 是指纹校对的运行时配置。
type FingerprintSettings struct {
	// DataSources 是参考数据来源。只有一项且不以 http(s) 开头时，SDK 当作本地目录读取，不联网。
	DataSources []string
	// CachePath 为空时用 SDK 默认的 <OS cache dir>/lmfpd/data.bin。
	CachePath string
	// CacheTTL 是缓存多久检查一次更新。
	CacheTTL time.Duration
	// MaxConcurrency 是集群级并发闸上限。
	MaxConcurrency int
	// DailyLimitGlobal 是全局每日检测次数上限。
	DailyLimitGlobal int
	// DailyLimitAccount 是单账号每日检测次数上限。
	DailyLimitAccount int
	// NotifyEmail 控制指纹不符是否发邮件，默认关闭以免抽查频繁导致垃圾邮件。
	NotifyEmail bool
}

// GetFingerprintSettings 一次性读出指纹校对的全部配置。
// 任何一项缺失或非法都回落到规格默认值——指纹检测要花钱，配置坏了不能变成"不限量"。
func (s *SettingService) GetFingerprintSettings(ctx context.Context) FingerprintSettings {
	return FingerprintSettings{
		DataSources:       s.getFingerprintDataSources(ctx),
		CachePath:         strings.TrimSpace(s.fingerprintRawValue(ctx, SettingKeyFingerprintCachePath)),
		CacheTTL:          time.Duration(s.fingerprintPositiveInt(ctx, SettingKeyFingerprintCacheTTLHours, FingerprintDefaultCacheTTLHours)) * time.Hour,
		MaxConcurrency:    s.fingerprintPositiveInt(ctx, SettingKeyFingerprintMaxConcurrency, FingerprintDefaultMaxConcurrency),
		DailyLimitGlobal:  s.fingerprintPositiveInt(ctx, SettingKeyFingerprintDailyLimitGlobal, FingerprintDefaultDailyLimitGlobal),
		DailyLimitAccount: s.fingerprintPositiveInt(ctx, SettingKeyFingerprintDailyLimitAccount, FingerprintDefaultDailyLimitAccount),
		NotifyEmail:       isTrueSettingValue(s.fingerprintRawValue(ctx, SettingKeyFingerprintNotifyEmail)),
	}
}

func (s *SettingService) fingerprintRawValue(ctx context.Context, key string) string {
	if s == nil || s.settingRepo == nil {
		return ""
	}
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return ""
	}
	return value
}

// fingerprintPositiveInt 读取一个正整数设置；缺失、非数字或非正数都回落到 fallback。
func (s *SettingService) fingerprintPositiveInt(ctx context.Context, key string, fallback int) int {
	raw := strings.TrimSpace(s.fingerprintRawValue(ctx, key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// getFingerprintDataSources 解析 JSON 数组形式的来源列表。
// 兼容单行字符串写法（直接当作唯一来源），方便内网部署直接填一个本地目录。
func (s *SettingService) getFingerprintDataSources(ctx context.Context) []string {
	raw := strings.TrimSpace(s.fingerprintRawValue(ctx, SettingKeyFingerprintDataSources))
	if raw == "" {
		return append([]string(nil), FingerprintDefaultDataSources...)
	}

	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		// 不是 JSON 数组时按单个来源处理。
		return []string{raw}
	}

	out := make([]string, 0, len(list))
	for _, item := range list {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), FingerprintDefaultDataSources...)
	}
	return out
}
