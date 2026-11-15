package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func expireGlobalIPBlacklistCache(svc *SettingService) {
	cached := *svc.globalIPBlacklistCache.Load()
	cached.expiresAt = time.Time{}
	svc.globalIPBlacklistCache.Store(&cached)
}

func TestGlobalIPBlacklistDefaultsAndCorruptStorage(t *testing.T) {
	ctx := context.Background()
	svc := newPanelRateLimitTestService(&panelRateLimitSettingRepo{})
	settings, err := svc.GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Rules)
	blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)

	for _, raw := range []string{
		"", "{broken", "null", "{}", `{"enabled":true}`, `{"rules":[]}`,
		`{"enabled":true,"rules":null}`, `{"enabled":null,"rules":[]}`,
		`{"enabled":true,"rules":["not-an-ip"]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyGlobalIPBlacklist: raw}}
			svc := newPanelRateLimitTestService(repo)
			_, err := svc.GetGlobalIPBlacklistSettings(ctx)
			require.Error(t, err)
			_, err = svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
			require.ErrorIs(t, err, ErrServiceUnavailable)
		})
	}
}

func TestGlobalIPBlacklistNormalizationAndValidation(t *testing.T) {
	settings, err := normalizeGlobalIPBlacklist(&GlobalIPBlacklistSettings{Enabled: true, Rules: []string{
		" 203.0.113.10 ", "::ffff:203.0.113.10", "2001:0db8:0000::1", "2001:db8::1",
		"198.51.100.42/24", "198.51.100.0/24", "::ffff:198.51.100.99/120", "2001:db8:1234::1/32",
	}})
	require.NoError(t, err)
	require.Equal(t, []string{"203.0.113.10", "2001:db8::1", "198.51.100.0/24", "2001:db8::/32"}, settings.Rules)

	for _, rule := range []string{"", "example.com", "1.2.3.4:80", "1.2.3.4/33", "2001:db8::/129", "::ffff:192.0.2.1/80", "fe80::1%eth0"} {
		_, err := normalizeGlobalIPBlacklist(&GlobalIPBlacklistSettings{Rules: []string{rule}})
		require.Error(t, err, rule)
	}
	_, err = normalizeGlobalIPBlacklist(nil)
	require.Error(t, err)
	_, err = normalizeGlobalIPBlacklist(&GlobalIPBlacklistSettings{Rules: make([]string, GlobalIPBlacklistMaxRules+1)})
	require.Error(t, err)
}

func TestGlobalIPBlacklistEnforcementAndImmediateUnblock(t *testing.T) {
	ctx := context.Background()
	svc := newPanelRateLimitTestService(&panelRateLimitSettingRepo{})
	_, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10") // Populate the default cache first.
	require.NoError(t, err)
	input := &GlobalIPBlacklistSettings{Enabled: true, Rules: []string{"203.0.113.10", "198.51.100.0/24", "2001:db8::/32"}}
	saved, err := svc.SetGlobalIPBlacklistSettings(ctx, input, "192.0.2.1")
	require.NoError(t, err)
	// Neither the input nor a mutable API response owns the cached snapshot.
	input.Rules[0], saved.Rules[1] = "192.0.2.1", "192.0.2.1"
	for _, clientIP := range []string{"203.0.113.10", "::ffff:203.0.113.10", "198.51.100.255", "2001:db8:1234::1"} {
		blocked, err := svc.IsGlobalIPBlocked(ctx, clientIP)
		require.NoError(t, err)
		require.True(t, blocked, clientIP)
	}
	for _, clientIP := range []string{"192.0.2.1", "198.51.101.1", "2001:db9::1"} {
		blocked, err := svc.IsGlobalIPBlocked(ctx, clientIP)
		require.NoError(t, err)
		require.False(t, blocked, clientIP)
	}
	stored, err := svc.GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	stored.Enabled = false
	_, err = svc.SetGlobalIPBlacklistSettings(ctx, stored, "192.0.2.1")
	require.NoError(t, err)
	blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)
	_, err = svc.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{Enabled: true}, "192.0.2.1")
	require.NoError(t, err)
	blocked, err = svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)
}

func TestGlobalIPBlacklistRejectsSelfBlock(t *testing.T) {
	for _, tc := range []struct{ currentIP, rule string }{
		{"203.0.113.10", "203.0.113.10"}, {"203.0.113.10", "203.0.113.0/24"},
		{"203.0.113.10", "0.0.0.0/0"}, {"::ffff:203.0.113.10", "203.0.113.10"},
		{"2001:db8::1", "2001:db8::/32"}, {"2001:db8::1", "::/0"},
	} {
		t.Run(tc.currentIP+"/"+tc.rule, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{}
			svc := newPanelRateLimitTestService(repo)
			input := &GlobalIPBlacklistSettings{Enabled: true, Rules: []string{tc.rule}}
			_, err := svc.SetGlobalIPBlacklistSettings(context.Background(), input, tc.currentIP)
			require.Error(t, err)
			require.Empty(t, repo.values, "Rejected settings must not be persisted")
			input.Enabled = false
			_, err = svc.SetGlobalIPBlacklistSettings(context.Background(), input, tc.currentIP)
			require.NoError(t, err)
		})
	}
}

func TestGlobalIPBlacklistRefreshRetainsPolicyOnFailure(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{}
	svc := newPanelRateLimitTestService(repo)
	_, err := svc.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{Enabled: true, Rules: []string{"203.0.113.10"}}, "192.0.2.1")
	require.NoError(t, err)
	for _, corrupt := range []bool{false, true} {
		if corrupt {
			repo.getValueErr = nil
			require.NoError(t, repo.Set(ctx, SettingKeyGlobalIPBlacklist, "null"))
		} else {
			repo.getValueErr = errors.New("database unavailable")
		}
		expireGlobalIPBlacklistCache(svc)
		blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
		require.NoError(t, err)
		require.True(t, blocked, "A failed refresh must not drop existing bans")
	}
	// A second instance sees persistent changes when its bounded cache expires.
	otherNode := newPanelRateLimitTestService(repo)
	_, err = otherNode.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{}, "192.0.2.1")
	require.NoError(t, err)
	expireGlobalIPBlacklistCache(svc)
	blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)

	repo.getValueErr = errors.New("database unavailable")
	cold := newPanelRateLimitTestService(repo)
	_, err = cold.IsGlobalIPBlocked(ctx, "192.0.2.1")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	calls := repo.getValueCalls
	_, err = cold.IsGlobalIPBlocked(ctx, "192.0.2.1")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Equal(t, calls, repo.getValueCalls, "Failure retries are bounded too")
}

type globalIPBlacklistWriteFailureRepo struct {
	*panelRateLimitSettingRepo
}

func (r *globalIPBlacklistWriteFailureRepo) Set(_ context.Context, _, _ string) error {
	return errors.New("write failed")
}

func TestGlobalIPBlacklistWriteFailureKeepsPolicy(t *testing.T) {
	repo := &globalIPBlacklistWriteFailureRepo{&panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyGlobalIPBlacklist: `{"enabled":true,"rules":["203.0.113.10"]}`,
	}}}
	svc := newPanelRateLimitTestService(repo)
	ctx := context.Background()
	blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.True(t, blocked)
	_, err = svc.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{}, "192.0.2.1")
	require.Error(t, err)
	blocked, err = svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.True(t, blocked)
	require.Contains(t, repo.values[SettingKeyGlobalIPBlacklist], "203.0.113.10")
}

type globalIPBlacklistTxSettingRepo struct {
	*panelRateLimitSettingRepo
	txMu      sync.Mutex
	commitErr error
}

func (r *globalIPBlacklistTxSettingRepo) WithGlobalIPBlacklistUpdate(ctx context.Context, fn func(context.Context) error) error {
	r.txMu.Lock()
	defer r.txMu.Unlock()
	before, err := r.GetAll(ctx)
	if err != nil {
		return err
	}
	err = fn(ctx)
	if err == nil {
		err = r.commitErr
	}
	if err != nil {
		r.mu.Lock()
		r.values = before
		r.mu.Unlock()
	}
	return err
}

func TestGlobalIPBlacklistDeleteMergesRulesAndEnablesPolicy(t *testing.T) {
	ctx := context.Background()
	repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
	svc := newPanelRateLimitTestService(repo)
	_, err := svc.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{Rules: []string{"198.51.100.0/24"}}, "192.0.2.1")
	require.NoError(t, err)
	called := 0
	deleteUser := func(context.Context) error {
		called++
		return nil
	}
	for _, clientIP := range []string{"203.0.113.10", "::ffff:203.0.113.10", "198.51.100.1"} {
		require.NoError(t, svc.deleteUserAndBlockRegistrationIP(ctx, clientIP, "192.0.2.1", deleteUser))
	}
	require.Equal(t, 3, called)
	settings, err := svc.GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, []string{"198.51.100.0/24", "203.0.113.10"}, settings.Rules)
	blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
	require.NoError(t, err)
	require.True(t, blocked)
}

func TestGlobalIPBlacklistFailedDeleteOrCommitDoesNotPublishBan(t *testing.T) {
	for _, commitFails := range []bool{false, true} {
		repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
		svc := newPanelRateLimitTestService(repo)
		ctx := context.Background()
		_, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
		require.NoError(t, err)
		failure := errors.New("delete or commit failed")
		if commitFails {
			repo.commitErr = failure
		}
		err = svc.deleteUserAndBlockRegistrationIP(ctx, "203.0.113.10", "192.0.2.1", func(context.Context) error {
			if !commitFails {
				return failure
			}
			return nil
		})
		require.ErrorIs(t, err, failure)
		blocked, err := svc.IsGlobalIPBlocked(ctx, "203.0.113.10")
		require.NoError(t, err)
		require.False(t, blocked, "A rolled-back policy must never enter the request cache")
		settings, err := svc.GetGlobalIPBlacklistSettings(ctx)
		require.NoError(t, err)
		require.False(t, settings.Enabled)
		require.Empty(t, settings.Rules)
	}
}

func TestGlobalIPBlacklistConcurrentDeletesAcrossInstancesKeepBothIPs(t *testing.T) {
	repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
	ctx := context.Background()
	results := make(chan error, 2)
	for _, clientIP := range []string{"203.0.113.10", "203.0.113.20"} {
		go func(clientIP string) {
			svc := newPanelRateLimitTestService(repo)
			results <- svc.deleteUserAndBlockRegistrationIP(ctx, clientIP, "192.0.2.1", func(context.Context) error { return nil })
		}(clientIP)
	}
	for range 2 {
		require.NoError(t, <-results)
	}
	settings, err := newPanelRateLimitTestService(repo).GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"203.0.113.10", "203.0.113.20"}, settings.Rules)
}
