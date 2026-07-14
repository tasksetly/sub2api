package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ticketStorageSettingRepoStub struct {
	mu   sync.Mutex
	data map[string]string
}

func newTicketStorageSettingRepoStub() *ticketStorageSettingRepoStub {
	return &ticketStorageSettingRepoStub{data: make(map[string]string)}
}

func (r *ticketStorageSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.data[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *ticketStorageSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.data[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *ticketStorageSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
	return nil
}

func (r *ticketStorageSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]string)
	for _, key := range keys {
		if value, ok := r.data[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *ticketStorageSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range values {
		r.data[key] = value
	}
	return nil
}

func (r *ticketStorageSettingRepoStub) GetAll(_ context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]string, len(r.data))
	for key, value := range r.data {
		result[key] = value
	}
	return result, nil
}

func (r *ticketStorageSettingRepoStub) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, key)
	return nil
}

type ticketStorageEncryptorStub struct{}

func (ticketStorageEncryptorStub) Encrypt(value string) (string, error) {
	return "encrypted:" + value, nil
}

func (ticketStorageEncryptorStub) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "encrypted:") {
		return "", fmt.Errorf("not encrypted")
	}
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type ticketStorageObjectStoreStub struct {
	headBucketCalls int
}

func (*ticketStorageObjectStoreStub) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (*ticketStorageObjectStoreStub) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (*ticketStorageObjectStoreStub) Delete(context.Context, string) error {
	return nil
}

func (*ticketStorageObjectStoreStub) PresignURL(context.Context, string, time.Duration) (string, error) {
	return "https://example.test/object", nil
}

func (s *ticketStorageObjectStoreStub) HeadBucket(context.Context) error {
	s.headBucketCalls++
	return nil
}

func TestSupportTicketAttachmentManagerPreservesEncryptsAndReloadsSecret(t *testing.T) {
	repo := newTicketStorageSettingRepoStub()
	var factoryConfigs []config.SupportTicketAttachmentConfig
	factory := func(_ context.Context, cfg config.SupportTicketAttachmentConfig) (SupportTicketAttachmentObjectStore, error) {
		factoryConfigs = append(factoryConfigs, cfg)
		return &ticketStorageObjectStoreStub{}, nil
	}
	appCfg := &config.Config{SupportTicket: config.SupportTicketConfig{
		Attachments: config.SupportTicketAttachmentConfig{
			Enabled: true, Endpoint: "https://old.example.test", Region: "auto",
			Bucket: "tickets", AccessKeyID: "access", SecretAccessKey: "env-secret",
			Prefix: "support-tickets", MaxFileSizeMB: 10, MaxAttachmentsMessage: 4,
			URLExpiryMinutes: 15,
		},
	}}

	manager, err := ProvideSupportTicketAttachmentManager(repo, appCfg, ticketStorageEncryptorStub{}, factory)
	require.NoError(t, err)
	require.True(t, manager.GetConfig().SecretConfigured)
	require.Empty(t, manager.GetConfig().SecretAccessKey)

	updated, err := manager.UpdateConfig(context.Background(), SupportTicketAttachmentStorageConfig{
		Enabled: true, Endpoint: "https://new.example.test", Region: "auto",
		Bucket: "tickets", AccessKeyID: "access", Prefix: "ticket-images",
		MaxFileSizeMB: 20, MaxAttachmentsPerMessage: 5, URLExpiryMinutes: 30,
	})
	require.NoError(t, err)
	require.True(t, updated.SecretConfigured)
	require.Empty(t, updated.SecretAccessKey)
	require.Equal(t, "env-secret", manager.CurrentConfig().SecretAccessKey)
	require.Equal(t, "https://new.example.test", manager.CurrentConfig().Endpoint)
	require.Len(t, factoryConfigs, 2)
	require.Equal(t, "env-secret", factoryConfigs[1].SecretAccessKey)

	var stored SupportTicketAttachmentStorageConfig
	require.NoError(t, json.Unmarshal([]byte(repo.data[settingKeySupportTicketAttachmentStorage]), &stored))
	require.Equal(t, "encrypted:env-secret", stored.SecretAccessKey)
	require.NotContains(t, repo.data[settingKeySupportTicketAttachmentStorage], `"env-secret"`)

	reloaded, err := ProvideSupportTicketAttachmentManager(
		repo,
		&config.Config{},
		ticketStorageEncryptorStub{},
		factory,
	)
	require.NoError(t, err)
	require.Equal(t, "env-secret", reloaded.CurrentConfig().SecretAccessKey)
	require.Equal(t, "ticket-images", reloaded.CurrentConfig().Prefix)
}

func TestSupportTicketAttachmentManagerHotUpdatesPolicyAndTestsSavedSecret(t *testing.T) {
	repo := newTicketStorageSettingRepoStub()
	store := &ticketStorageObjectStoreStub{}
	factory := func(_ context.Context, _ config.SupportTicketAttachmentConfig) (SupportTicketAttachmentObjectStore, error) {
		return store, nil
	}
	manager, err := ProvideSupportTicketAttachmentManager(
		repo,
		&config.Config{},
		ticketStorageEncryptorStub{},
		factory,
	)
	require.NoError(t, err)
	ticketService := NewSupportTicketService(nil, manager, &config.Config{}, nil)
	require.False(t, ticketService.AttachmentPolicy().Enabled)

	_, err = manager.UpdateConfig(context.Background(), SupportTicketAttachmentStorageConfig{
		Enabled: true, Endpoint: "https://r2.example.test", Region: "auto",
		Bucket: "tickets", AccessKeyID: "access", SecretAccessKey: "secret",
		Prefix: "support-tickets", MaxFileSizeMB: 12, MaxAttachmentsPerMessage: 3,
		URLExpiryMinutes: 20,
	})
	require.NoError(t, err)
	policy := ticketService.AttachmentPolicy()
	require.True(t, policy.Enabled)
	require.Equal(t, int64(12*1024*1024), policy.MaxFileSizeBytes)
	require.Equal(t, 3, policy.MaxAttachmentsPerMessage)

	err = manager.TestConnection(context.Background(), SupportTicketAttachmentStorageConfig{
		Endpoint: "https://r2.example.test", Region: "auto", Bucket: "tickets",
		AccessKeyID: "access", MaxFileSizeMB: 12, MaxAttachmentsPerMessage: 3,
		URLExpiryMinutes: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, store.headBucketCalls)
}
