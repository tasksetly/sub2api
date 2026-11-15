package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

func (s *NotificationWebhookService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.startOnce.Do(func() {
		for range 2 {
			s.wg.Add(1)
			go s.runWorker()
		}
	})
}

func (s *NotificationWebhookService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.cancel()
		s.wg.Wait()
		s.client.CloseIdleConnections()
	})
}

func (s *NotificationWebhookService) runWorker() {
	defer s.wg.Done()
	workerID := uuid.NewString()
	nextCleanup := time.Time{}
	for s.ctx.Err() == nil {
		if time.Now().After(nextCleanup) {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			if err := s.repo.Cleanup(ctx); err != nil && s.ctx.Err() == nil {
				slog.Warn("webhook delivery cleanup failed", "error", err)
			}
			cancel()
			nextCleanup = time.Now().Add(time.Hour)
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		item, err := s.repo.Claim(ctx, workerID)
		cancel()
		if err != nil && s.ctx.Err() == nil {
			slog.Warn("webhook delivery claim failed", "error", err)
		}
		if err == nil && item != nil {
			s.processDelivery(item, workerID)
			continue
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func webhookRetryDelay(attempts int) time.Duration {
	delays := [...]time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	if attempts < 1 {
		return delays[0]
	}
	if attempts > len(delays) {
		return delays[len(delays)-1]
	}
	return delays[attempts-1]
}

func (s *NotificationWebhookService) processDelivery(item *NotificationWebhookDelivery, workerID string) {
	ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
	defer cancel()
	status, statusCode, message := "delivered", 0, ""
	config, destination, err := s.readConfig(ctx)
	if err == nil && (!config.Enabled || config.Version != item.ConfigVersion) {
		status, message = "skipped", "webhook disabled or configuration changed"
	} else if err == nil {
		statusCode, err = s.deliver(ctx, destination, item.EventID, item.EventType, item.Payload)
	}
	if err != nil {
		status, message = "pending", "webhook configuration unavailable"
		if config.Version == item.ConfigVersion && config.Enabled {
			message = "webhook request failed"
		}
		if statusCode != 0 {
			message = fmt.Sprintf("receiver returned HTTP %d", statusCode)
		}
		if item.Attempts >= WebhookMaxAttempts {
			status = "failed"
		}
	}
	// Persist the outcome even if HTTP delivery used its whole timeout. Leases
	// recover unfinished work after a crash; receivers deduplicate by event_id.
	finishCtx, finishCancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer finishCancel()
	if err := s.repo.Finish(finishCtx, item.ID, workerID, status, statusCode, message, time.Now().Add(webhookRetryDelay(item.Attempts))); err != nil && s.ctx.Err() == nil {
		slog.Warn("webhook delivery result could not be saved", "delivery_id", item.ID, "error", err)
	}
}

func (s *NotificationWebhookService) deliver(ctx context.Context, destination notificationWebhookDestination, eventID, eventType string, payload []byte) (int, error) {
	if err := validateNotificationWebhookURL(destination.URL); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destination.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, errors.New("invalid webhook request")
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Sub2API-Webhook/1.0")
	req.Header.Set("X-Webhook-Id", eventID)
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-Timestamp", timestamp)
	if destination.Secret != "" {
		mac := hmac.New(sha256.New, []byte(destination.Secret))
		_, _ = mac.Write([]byte(timestamp + "."))
		_, _ = mac.Write(payload)
		req.Header.Set("X-Webhook-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		// http/url errors can contain destination credentials. Never expose them.
		return 0, errors.New("webhook request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("receiver returned HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (s *NotificationWebhookService) SendTest(ctx context.Context) error {
	_, destination, err := s.readConfig(ctx)
	if err != nil {
		return err
	}
	if destination.URL == "" {
		return infraerrors.BadRequest("WEBHOOK_URL_INVALID", "save a webhook URL before sending a test")
	}
	eventID := "test:" + uuid.NewString()
	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]any{
		"id": eventID, "event": WebhookEventRechargeCompleted, "created_at": now, "test": true,
		"data": map[string]any{
			"order_id": 0, "out_trade_no": "TEST", "user_id": 0,
			"user_email": "test@example.com", "user_name": "Webhook test",
			"order_type": "balance", "status": "COMPLETED", "payment_type": "test",
			"pay_amount": 10, "pay_currency": "CNY", "credited_amount": 1.4,
			"bonus_amount": 0, "balance_currency": "USD", "paid_at": now, "completed_at": now,
		},
	})
	if err != nil {
		return err
	}
	_, err = s.deliver(ctx, destination, eventID, WebhookEventRechargeCompleted, payload)
	if err != nil {
		return infraerrors.BadRequest("WEBHOOK_TEST_FAILED", err.Error())
	}
	return nil
}
