//go:build unit

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type webhookRepositoryStub struct {
	NotificationWebhookRepository
	settings    *panelRateLimitSettingRepo
	rejectSave  bool
	saves       int
	status      string
	statusCode  int
	message     string
	availableAt time.Time
}

func (r *webhookRepositoryStub) SaveConfig(ctx context.Context, _ string, value string) (bool, error) {
	if r.rejectSave {
		return false, nil
	}
	r.saves++
	return true, r.settings.Set(ctx, SettingKeyNotificationWebhook, value)
}

func (r *webhookRepositoryStub) Finish(_ context.Context, _ int64, _, status string, code int, message string, availableAt time.Time) error {
	r.status, r.statusCode, r.message, r.availableAt = status, code, message, availableAt
	return nil
}

func newWebhookFixture(t *testing.T) (*NotificationWebhookService, *webhookRepositoryStub) {
	t.Helper()
	repo := &webhookRepositoryStub{settings: &panelRateLimitSettingRepo{}}
	svc := NewNotificationWebhookService(repo.settings, repo, reversibleEncryptor{}, &config.Config{
		Totp: config.TotpConfig{EncryptionKeyConfigured: true},
	})
	t.Cleanup(svc.Stop)
	return svc, repo
}

func TestNotificationWebhookSettingsKeepAndClearSecret(t *testing.T) {
	ctx := context.Background()
	svc, repo := newWebhookFixture(t)
	initial, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.False(t, initial.Enabled)
	require.Empty(t, initial.Version)

	input := UpdateNotificationWebhookSettings{
		Enabled: true, URL: " https://example.com/a%2Fb/?key=token ", Secret: "signing-secret",
	}
	saved, err := svc.UpdateSettings(ctx, input)
	require.NoError(t, err)
	require.True(t, saved.HasSecret)
	require.NotEmpty(t, saved.Version)
	require.Equal(t, strings.TrimSpace(input.URL), saved.URL)
	stored, destination, err := svc.readConfig(ctx)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stored.Destination, "enc:"), "must use SecretEncryptor")
	require.Equal(t, input.Secret, destination.Secret)
	response, err := json.Marshal(saved)
	require.NoError(t, err)
	require.NotContains(t, string(response), "signing-secret")

	input.Version, input.Secret = saved.Version, ""
	unchanged, err := svc.UpdateSettings(ctx, input)
	require.NoError(t, err)
	require.Equal(t, saved.Version, unchanged.Version, "no-op save must retain queued events")
	require.Equal(t, 1, repo.saves)
	input.Enabled = false
	disabled, err := svc.UpdateSettings(ctx, input)
	require.NoError(t, err)
	require.True(t, disabled.HasSecret, "empty secret must preserve the saved secret")
	require.NotEqual(t, saved.Version, disabled.Version)
	input.Version, input.ClearSecret = disabled.Version, true
	cleared, err := svc.UpdateSettings(ctx, input)
	require.NoError(t, err)
	require.False(t, cleared.HasSecret)
	_, destination, err = svc.readConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, destination.Secret)
}

func TestNotificationWebhookRejectsStaleSettingsAndEphemeralKey(t *testing.T) {
	ctx := context.Background()
	svc, repo := newWebhookFixture(t)
	input := UpdateNotificationWebhookSettings{Enabled: true, URL: "https://example.com/hook"}
	saved, err := svc.UpdateSettings(ctx, input)
	require.NoError(t, err)
	_, err = svc.UpdateSettings(ctx, input)
	require.ErrorContains(t, err, "settings changed")
	require.Equal(t, 1, repo.saves)

	input.Version, input.URL = saved.Version, "https://example.com/new"
	repo.rejectSave = true // Another instance wins after readConfig.
	_, err = svc.UpdateSettings(ctx, input)
	require.ErrorContains(t, err, "settings changed")
	stillSaved, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, stillSaved)

	svc.encryptionKeyConfigured = false
	_, err = svc.UpdateSettings(ctx, input)
	require.ErrorContains(t, err, "TOTP_ENCRYPTION_KEY")
	require.Equal(t, 1, repo.saves)
}

func TestNotificationWebhookRejectsUnsafeURLs(t *testing.T) {
	for _, address := range []string{
		"", "http://example.com/hook", "https://localhost/hook", "https://127.0.0.1/",
		"https://10.0.0.1/", "https://169.254.169.254/", "https://[::ffff:127.0.0.1]/",
		"https://[::1]/", "https://[fc00::1]/", "https://100.64.0.1/",
		"https://metadata.google.internal/", "https://user:secret@example.com/",
		"https://example.com/#token", "https://example.com:0/",
	} {
		t.Run(address, func(t *testing.T) {
			require.Error(t, validateNotificationWebhookURL(address))
		})
	}
	require.NoError(t, validateNotificationWebhookURL("https://example.com/a%2Fb/?token=value"))
	svc, _ := newWebhookFixture(t)
	transport := svc.client.Transport.(*http.Transport)
	require.Nil(t, transport.Proxy, "environment proxies must not bypass the dial policy")
	_, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:443")
	require.Error(t, err, "the socket layer must reject private addresses too")
}

func TestNotificationWebhookSignsExactBodyAndPreservesURL(t *testing.T) {
	svc, _ := newWebhookFixture(t)
	address := "https://example.com/a%2Fb/?token=signed%2Fvalue"
	payload := []byte("{\n  \"id\": \"event-1\"\n}")
	svc.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, payload, body)
		require.Equal(t, address, req.URL.String())
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))
		require.Equal(t, "event-1", req.Header.Get("X-Webhook-Id"))
		require.Equal(t, WebhookEventRechargeCompleted, req.Header.Get("X-Webhook-Event"))
		timestamp := req.Header.Get("X-Webhook-Timestamp")
		_, err = strconv.ParseInt(timestamp, 10, 64)
		require.NoError(t, err)
		mac := hmac.New(sha256.New, []byte("secret"))
		_, _ = mac.Write(append([]byte(timestamp+"."), payload...))
		require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), req.Header.Get("X-Webhook-Signature"))
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	code, err := svc.deliver(context.Background(), notificationWebhookDestination{URL: address, Secret: "secret"}, "event-1", WebhookEventRechargeCompleted, payload)
	require.NoError(t, err)
	require.Equal(t, 204, code)
}

func TestNotificationWebhookDoesNotFollowRedirectOrExposeURLErrors(t *testing.T) {
	svc, _ := newWebhookFixture(t)
	destination := notificationWebhookDestination{URL: "https://example.com/private-token?key=secret"}
	calls := 0
	svc.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Empty(t, req.Header.Get("X-Webhook-Signature"))
		return &http.Response{
			StatusCode: 302, Header: http.Header{"Location": {"https://127.0.0.1/"}},
			Body: io.NopCloser(strings.NewReader("sensitive receiver response")),
		}, nil
	})
	code, err := svc.deliver(context.Background(), destination, "event-1", WebhookEventRechargeCompleted, []byte(`{}`))
	require.Equal(t, 302, code)
	require.EqualError(t, err, "receiver returned HTTP 302")
	require.Equal(t, 1, calls)
	svc.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(destination.URL)
	})
	_, err = svc.deliver(context.Background(), destination, "event-1", WebhookEventRechargeCompleted, []byte(`{}`))
	require.EqualError(t, err, "webhook request failed")
}

func TestNotificationWebhookRetriesStableEventAndSkipsOldConfiguration(t *testing.T) {
	svc, repo := newWebhookFixture(t)
	input := UpdateNotificationWebhookSettings{Enabled: true, URL: "https://example.com/hook"}
	settings, err := svc.UpdateSettings(context.Background(), input)
	require.NoError(t, err)
	item := &NotificationWebhookDelivery{
		ID: 1, EventID: "payment.recharge.completed:123", EventType: WebhookEventRechargeCompleted,
		ConfigVersion: settings.Version, Payload: json.RawMessage(`{"id":"payment.recharge.completed:123"}`),
	}
	calls := 0
	svc.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, []byte(item.Payload), body)
		require.Equal(t, item.EventID, req.Header.Get("X-Webhook-Id"))
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	delays := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	for attempt := 1; attempt <= WebhookMaxAttempts; attempt++ {
		item.Attempts = attempt
		before := time.Now()
		svc.processDelivery(item, "worker")
		if attempt == WebhookMaxAttempts {
			require.Equal(t, "failed", repo.status)
		} else {
			require.Equal(t, "pending", repo.status)
			require.WithinDuration(t, before.Add(delays[attempt-1]), repo.availableAt, time.Second)
		}
		require.Equal(t, 503, repo.statusCode)
	}
	require.Equal(t, 5, calls)
	input.Version, input.URL = settings.Version, "https://example.com/new"
	settings, err = svc.UpdateSettings(context.Background(), input)
	require.NoError(t, err)
	svc.processDelivery(item, "worker")
	require.Equal(t, "skipped", repo.status)
	require.Equal(t, 5, calls, "old events must never go to the new receiver")
	item.ConfigVersion = settings.Version
	input.Version, input.Enabled = settings.Version, false
	_, err = svc.UpdateSettings(context.Background(), input)
	require.NoError(t, err)
	svc.processDelivery(item, "worker")
	require.Equal(t, "skipped", repo.status)
	require.Equal(t, 5, calls)
}

func TestNotificationWebhookTestUsesSyntheticDataWithoutQueueing(t *testing.T) {
	svc, repo := newWebhookFixture(t)
	_, err := svc.UpdateSettings(context.Background(), UpdateNotificationWebhookSettings{URL: "https://example.com/hook"})
	require.NoError(t, err)
	calls := 0
	svc.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var payload struct {
			ID   string `json:"id"`
			Test bool   `json:"test"`
			Data struct {
				OrderID int64 `json:"order_id"`
				UserID  int64 `json:"user_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		require.True(t, payload.Test)
		require.True(t, strings.HasPrefix(payload.ID, "test:"))
		require.Zero(t, payload.Data.OrderID)
		require.Zero(t, payload.Data.UserID)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	require.NoError(t, svc.SendTest(context.Background()))
	require.Equal(t, 1, calls)
	require.Equal(t, 1, repo.saves)
	require.Empty(t, repo.status)
}
