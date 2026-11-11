package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const defaultUpstreamSyncCron = "*/30 * * * *"

type UpstreamProviderAutomationConfig struct {
	SyncCron              string  `json:"sync_cron"`
	BalanceAlertEnabled   bool    `json:"balance_alert_enabled"`
	BalanceAlertThreshold float64 `json:"balance_alert_threshold"`
	BalanceAlertEmail     string  `json:"balance_alert_email"`
}

type UpstreamProviderAutomation struct {
	UpstreamProviderAutomationConfig
	NextSyncAt            *time.Time `json:"next_sync_at"`
	BalanceAlertSentAt    *time.Time `json:"balance_alert_sent_at"`
	BalanceAlertLastError string     `json:"balance_alert_last_error,omitempty"`
}

func DefaultUpstreamProviderAutomation() UpstreamProviderAutomation {
	return UpstreamProviderAutomation{UpstreamProviderAutomationConfig: UpstreamProviderAutomationConfig{
		SyncCron: defaultUpstreamSyncCron,
	}}
}

func nextUpstreamSync(cronExpr string, from time.Time) (time.Time, error) {
	// UTC is the stable default across replicas; CRON_TZ can override it.
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 && !(len(fields) == 6 && strings.HasPrefix(fields[0], "CRON_TZ=")) {
		return time.Time{}, fmt.Errorf("use five cron fields, optionally prefixed by CRON_TZ=Area/City")
	}
	next, err := computeNextRun(strings.Join(fields, " "), from.UTC())
	if err != nil {
		return time.Time{}, err
	}
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression has no upcoming execution")
	}
	return next, nil
}

func applyUpstreamAutomation(provider *UpstreamProvider, input *UpstreamProviderAutomationConfig, now time.Time) error {
	if input == nil {
		return nil
	}
	cfg := *input
	cfg.SyncCron = strings.Join(strings.Fields(cfg.SyncCron), " ")
	if cfg.SyncCron == "" {
		cfg.SyncCron = defaultUpstreamSyncCron
	}
	if len(cfg.SyncCron) > 128 {
		return infraerrors.BadRequest("UPSTREAM_SYNC_CRON_INVALID", "sync cron must be at most 128 characters")
	}
	next, err := nextUpstreamSync(cfg.SyncCron, now)
	if err != nil {
		return infraerrors.BadRequest("UPSTREAM_SYNC_CRON_INVALID", "invalid sync cron: "+err.Error())
	}
	if math.IsNaN(cfg.BalanceAlertThreshold) || math.IsInf(cfg.BalanceAlertThreshold, 0) || cfg.BalanceAlertThreshold < 0 || cfg.BalanceAlertThreshold >= 1e12 {
		return infraerrors.BadRequest("UPSTREAM_BALANCE_THRESHOLD_INVALID", "balance threshold must be finite, non-negative and less than 1000000000000")
	}
	cfg.BalanceAlertEmail = strings.TrimSpace(cfg.BalanceAlertEmail)
	if cfg.BalanceAlertEmail != "" {
		address, err := parseSMTPAddress(cfg.BalanceAlertEmail, "recipient")
		if err != nil || len(cfg.BalanceAlertEmail) > 254 {
			return infraerrors.BadRequest("UPSTREAM_BALANCE_EMAIL_INVALID", "a valid recipient email is required")
		}
		cfg.BalanceAlertEmail = address.Address
	}
	if cfg.BalanceAlertEnabled && cfg.BalanceAlertEmail == "" {
		return infraerrors.BadRequest("UPSTREAM_BALANCE_EMAIL_REQUIRED", "recipient email is required when balance alerts are enabled")
	}
	if provider.Automation.SyncCron != cfg.SyncCron || provider.Automation.NextSyncAt == nil {
		provider.Automation.NextSyncAt = &next
	}
	provider.Automation.UpstreamProviderAutomationConfig = cfg
	return nil
}

type upstreamBalanceEmailSender interface {
	SendEmail(context.Context, string, string, string) error
}

func (s *UpstreamProviderService) notifyUpstreamBalance(ctx context.Context, provider *UpstreamProvider, snapshot UpstreamSyncSnapshot) {
	if !provider.Automation.BalanceAlertEnabled || snapshot.Balance == nil || math.IsNaN(*snapshot.Balance) || math.IsInf(*snapshot.Balance, 0) || s.emailService == nil {
		return
	}
	claimedAt := time.Now().UTC().Truncate(time.Microsecond)
	claimed, err := s.repo.PrepareBalanceAlert(ctx, provider, snapshot, claimedAt)
	if err != nil {
		slog.Warn("upstream_balance_alert_prepare_failed", "provider_id", provider.ID, "error", err)
		return
	}
	if !claimed {
		return
	}
	subject, body := upstreamBalanceAlertMessage(provider, *snapshot.Balance)
	sendErr := s.emailService.SendEmail(ctx, provider.Automation.BalanceAlertEmail, subject, body)
	lastError := ""
	if sendErr != nil {
		lastError = sendErr.Error()
		slog.Warn("upstream_balance_alert_send_failed", "provider_id", provider.ID, "error", sendErr)
	}
	// Persist delivery even when a manual sync request was disconnected meanwhile.
	finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.FinishBalanceAlert(finishCtx, provider.ID, claimedAt, lastError); err != nil {
		slog.Warn("upstream_balance_alert_finish_failed", "provider_id", provider.ID, "error", err)
	}
}

func upstreamBalanceAlertMessage(provider *UpstreamProvider, balance float64) (string, string) {
	return "上游余额不足提醒 / Low upstream balance: " + provider.Name,
		fmt.Sprintf(`<p>上游余额已低于提醒阈值 / Upstream balance is below the alert threshold.</p><p>上游 / Provider: %s</p><p>地址 / URL: %s</p><p>余额 / Balance: %.8f</p><p>阈值 / Threshold: %.8f</p><p>金额为上游同步余额，未乘倍率修正系数。余额恢复后再次低于阈值才会重新提醒。<br>Amounts use the upstream balance before rate correction. A new alert is sent only after recovery and another drop below the threshold.</p>`,
			html.EscapeString(provider.Name), html.EscapeString(provider.BaseURL), balance, provider.Automation.BalanceAlertThreshold)
}
