package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyUsageRetention = "usage_retention"
const usageRetentionLockKey = "usage:retention:cleanup"

type UsageRetentionSettings struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
}

func (p UsageRetentionSettings) Validate() error {
	if p.RetentionDays < 1 || p.RetentionDays > 3650 {
		return infraerrors.BadRequest("INVALID_USAGE_RETENTION", "retention_days must be between 1 and 3650")
	}
	return nil
}

// Existing installations keep their deployment/runtime policy until explicitly changed.
func readUsageRetention(ctx context.Context, repo SettingRepository, cfg config.DashboardAggregationConfig) (UsageRetentionSettings, error) {
	days := cfg.Retention.UsageLogsDays
	if !cfg.Enabled {
		days = 0
	}
	if repo != nil {
		raw, err := repo.GetValue(ctx, SettingKeyUsageRetention)
		if err == nil {
			var policy UsageRetentionSettings
			if err := json.Unmarshal([]byte(raw), &policy); err != nil {
				return policy, err
			}
			return policy, policy.Validate()
		}
		if !errors.Is(err, ErrSettingNotFound) {
			return UsageRetentionSettings{}, err
		}
		raw, err = repo.GetValue(ctx, SettingKeyOpsRuntimeLogConfig)
		if err == nil {
			var legacy struct {
				Days *int `json:"request_retention_days"`
			}
			if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
				return UsageRetentionSettings{}, err
			}
			if legacy.Days != nil {
				days = *legacy.Days
			}
		} else if !errors.Is(err, ErrSettingNotFound) {
			return UsageRetentionSettings{}, err
		}
	}
	if days < 0 || days > 3650 {
		return UsageRetentionSettings{}, fmt.Errorf("invalid request_retention_days: %d", days)
	}
	policy := UsageRetentionSettings{Enabled: days > 0, RetentionDays: days}
	if days == 0 {
		policy.RetentionDays = 90
	}
	return policy, nil
}

func (s *SettingService) GetUsageRetentionSettings(ctx context.Context) (UsageRetentionSettings, error) {
	var cfg config.DashboardAggregationConfig
	if s.cfg != nil {
		cfg = s.cfg.DashboardAgg
	}
	return readUsageRetention(ctx, s.settingRepo, cfg)
}

type usageRetentionWriter interface {
	SetUsageRetentionSettings(context.Context, UsageRetentionSettings, UsageRetentionSettings, int64) error
}

func (s *SettingService) SetUsageRetentionSettings(ctx context.Context, policy UsageRetentionSettings, actorID int64) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if actorID <= 0 {
		return infraerrors.BadRequest("INVALID_OPERATOR", "invalid operator id")
	}
	old, err := s.GetUsageRetentionSettings(ctx)
	if err != nil {
		return err
	}
	writer, ok := s.settingRepo.(usageRetentionWriter)
	if !ok {
		return fmt.Errorf("usage retention writer not initialized")
	}
	if err := writer.SetUsageRetentionSettings(ctx, policy, old, actorID); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
