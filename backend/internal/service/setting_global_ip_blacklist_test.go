package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// Existing policy tests save against a fresh snapshot. Concurrency tests below
// pass their original snapshot directly to exercise stale-page rejection.
func saveCurrentGlobalIPBlacklistForTest(ctx context.Context, svc *SettingService, input *GlobalIPBlacklistSettings, currentIP string) (*GlobalIPBlacklistSettings, error) {
	current, err := svc.GetGlobalIPBlacklistSettings(ctx)
	if err != nil {
		return nil, err
	}
	request := *input
	request.Version = current.Version
	return svc.SetGlobalIPBlacklistSettings(ctx, &request, currentIP)
}

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
	saved, err := saveCurrentGlobalIPBlacklistForTest(ctx, svc, input, "192.0.2.1")
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
	_, err = saveCurrentGlobalIPBlacklistForTest(ctx, svc, &GlobalIPBlacklistSettings{Enabled: true}, "192.0.2.1")
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
			_, err := saveCurrentGlobalIPBlacklistForTest(context.Background(), svc, input, tc.currentIP)
			require.Error(t, err)
			require.Empty(t, repo.values, "Rejected settings must not be persisted")
			input.Enabled = false
			_, err = saveCurrentGlobalIPBlacklistForTest(context.Background(), svc, input, tc.currentIP)
			require.NoError(t, err)
		})
	}
}

func TestGlobalIPBlacklistRefreshRetainsPolicyOnFailure(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{}
	svc := newPanelRateLimitTestService(repo)
	_, err := saveCurrentGlobalIPBlacklistForTest(ctx, svc, &GlobalIPBlacklistSettings{Enabled: true, Rules: []string{"203.0.113.10"}}, "192.0.2.1")
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
	// Repair corrupt storage before another administrator reads and saves it.
	require.NoError(t, repo.Set(ctx, SettingKeyGlobalIPBlacklist, `{"enabled":true,"rules":["203.0.113.10"]}`))
	// A second instance sees persistent changes when its bounded cache expires.
	otherNode := newPanelRateLimitTestService(repo)
	_, err = saveCurrentGlobalIPBlacklistForTest(ctx, otherNode, &GlobalIPBlacklistSettings{}, "192.0.2.1")
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
	previous := svc.globalIPBlacklistCache.Load().settings.Version
	_, err = saveCurrentGlobalIPBlacklistForTest(ctx, svc, &GlobalIPBlacklistSettings{}, "192.0.2.1")
	require.ErrorContains(t, err, "write failed")
	require.Equal(t, previous, svc.globalIPBlacklistCache.Load().settings.Version)
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
	_, err := saveCurrentGlobalIPBlacklistForTest(ctx, svc, &GlobalIPBlacklistSettings{Rules: []string{"198.51.100.0/24"}}, "192.0.2.1")
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

func requireGlobalIPBlacklistDecision(t *testing.T, svc *SettingService, clientIP string, wantBlocked bool, wantErr error) {
	t.Helper()
	type decision struct {
		blocked bool
		err     error
	}
	result := make(chan decision, 1)
	go func() {
		blocked, err := svc.IsGlobalIPBlocked(context.Background(), clientIP)
		result <- decision{blocked: blocked, err: err}
	}()
	select {
	case got := <-result:
		require.ErrorIs(t, got.err, wantErr)
		require.Equal(t, wantBlocked, got.blocked, clientIP)
	case <-time.After(2 * time.Second):
		t.Fatal("request waited for a pending blacklist operation")
	}
}

func TestGlobalIPBlacklistSlowDeleteDoesNotBlockRequests(t *testing.T) {
	for _, warm := range []bool{true, false} {
		name := "cold cache"
		if warm {
			name = "expired cache"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{values: map[string]string{
				SettingKeyGlobalIPBlacklist: `{"enabled":true,"rules":["198.51.100.1"]}`,
			}}}
			svc := newPanelRateLimitTestService(repo)
			if warm {
				blocked, err := svc.IsGlobalIPBlocked(ctx, "198.51.100.1")
				require.NoError(t, err)
				require.True(t, blocked)
				expireGlobalIPBlacklistCache(svc)
			}
			started := make(chan context.Context, 1)
			release := make(chan struct{})
			var once sync.Once
			finish := func() { once.Do(func() { close(release) }) }
			defer finish()
			done := make(chan error, 1)
			go func() {
				done <- svc.deleteUserAndBlockRegistrationIP(ctx, "203.0.113.10", "192.0.2.1", func(opCtx context.Context) error {
					started <- opCtx
					select {
					case <-release:
						return nil
					case <-opCtx.Done():
						return opCtx.Err()
					}
				})
			}()
			select {
			case opCtx := <-started:
				deadline, ok := opCtx.Deadline()
				require.True(t, ok, "The deletion must share the bounded write context")
				require.LessOrEqual(t, time.Until(deadline), 10*time.Second)
			case <-time.After(2 * time.Second):
				t.Fatal("deletion did not start")
			}
			if warm {
				requireGlobalIPBlacklistDecision(t, svc, "198.51.100.1", true, nil)
				requireGlobalIPBlacklistDecision(t, svc, "192.0.2.1", false, nil)
				requireGlobalIPBlacklistDecision(t, svc, "203.0.113.10", false, nil)
			} else {
				requireGlobalIPBlacklistDecision(t, svc, "192.0.2.1", false, ErrServiceUnavailable)
			}
			finish()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("deletion did not finish")
			}
			requireGlobalIPBlacklistDecision(t, svc, "198.51.100.1", true, nil)
			requireGlobalIPBlacklistDecision(t, svc, "203.0.113.10", true, nil)
		})
	}
}

type globalIPBlacklistSlowReadRepo struct {
	*panelRateLimitSettingRepo
	started chan struct{}
	release chan struct{}
}

func (r *globalIPBlacklistSlowReadRepo) GetValue(ctx context.Context, key string) (string, error) {
	r.started <- struct{}{}
	select {
	case <-r.release:
		return r.panelRateLimitSettingRepo.GetValue(ctx, key)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestGlobalIPBlacklistSlowRefreshDoesNotBlockOtherReaders(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyGlobalIPBlacklist: `{"enabled":true,"rules":["203.0.113.10"]}`,
	}}
	svc := newPanelRateLimitTestService(repo)
	requireGlobalIPBlacklistDecision(t, svc, "203.0.113.10", true, nil)
	expireGlobalIPBlacklistCache(svc)
	slow := &globalIPBlacklistSlowReadRepo{panelRateLimitSettingRepo: repo, started: make(chan struct{}, 1), release: make(chan struct{})}
	svc.settingRepo = slow
	var once sync.Once
	finish := func() { once.Do(func() { close(slow.release) }) }
	defer finish()
	done := make(chan error, 1)
	go func() {
		_, err := svc.IsGlobalIPBlocked(context.Background(), "192.0.2.1")
		done <- err
	}()
	select {
	case <-slow.started:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not start")
	}
	requireGlobalIPBlacklistDecision(t, svc, "203.0.113.10", true, nil)
	requireGlobalIPBlacklistDecision(t, svc, "192.0.2.1", false, nil)
	finish()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestGlobalIPBlacklistQueuedWritersHonorDeadline(t *testing.T) {
	for _, operation := range []string{"save", "delete"} {
		t.Run(operation, func(t *testing.T) {
			repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
			svc := newPanelRateLimitTestService(repo)
			snapshot, err := svc.GetGlobalIPBlacklistSettings(context.Background())
			require.NoError(t, err)
			mu := svc.globalIPBlacklistMutex()
			require.NoError(t, mu.Lock(context.Background()))
			defer mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if operation == "save" {
					_, err := svc.SetGlobalIPBlacklistSettings(ctx, snapshot, "192.0.2.1")
					done <- err
					return
				}
				done <- svc.deleteUserAndBlockRegistrationIP(ctx, "203.0.113.10", "192.0.2.1", func(context.Context) error {
					return errors.New("expired deletion must not run")
				})
			}()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.DeadlineExceeded)
			case <-time.After(2 * time.Second):
				t.Fatal("writer ignored its deadline while waiting for the local gate")
			}
			values, err := repo.GetAll(context.Background())
			require.NoError(t, err)
			require.Empty(t, values)
		})
	}
}

func TestGlobalIPBlacklistVersionRoundTrip(t *testing.T) {
	for _, raw := range []string{"", `{"enabled":false, "rules":["203.0.113.10"]}`} {
		t.Run(raw, func(t *testing.T) {
			ctx := context.Background()
			repo := &panelRateLimitSettingRepo{}
			if raw != "" {
				require.NoError(t, repo.Set(ctx, SettingKeyGlobalIPBlacklist, raw))
			}
			svc := newPanelRateLimitTestService(repo)
			snapshot, err := svc.GetGlobalIPBlacklistSettings(ctx)
			require.NoError(t, err)
			require.Regexp(t, "^[a-f0-9]{64}$", snapshot.Version)
			originalVersion := snapshot.Version
			snapshot.Enabled = true
			snapshot.Rules = []string{"198.51.100.1"}
			saved, err := svc.SetGlobalIPBlacklistSettings(ctx, snapshot, "192.0.2.1")
			require.NoError(t, err)
			require.NotEqual(t, originalVersion, saved.Version)
			latest, err := svc.GetGlobalIPBlacklistSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, saved, latest)
			value, err := repo.GetValue(ctx, SettingKeyGlobalIPBlacklist)
			require.NoError(t, err)
			require.JSONEq(t, `{"enabled":true,"rules":["198.51.100.1"]}`, value, "Versions must not be persisted recursively")
			// A database recovery edit must also invalidate an open page.
			require.NoError(t, repo.Set(ctx, SettingKeyGlobalIPBlacklist, `{"enabled":false,"rules":[]}`))
			_, err = svc.SetGlobalIPBlacklistSettings(ctx, saved, "192.0.2.1")
			require.Equal(t, "IP_BLACKLIST_VERSION_CONFLICT", infraerrors.Reason(err))
		})
	}
}

func TestGlobalIPBlacklistStaleSaveCannotUndoAutomaticBan(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		ctx := context.Background()
		repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
		pageNode := newPanelRateLimitTestService(repo)
		deleteNode := newPanelRateLimitTestService(repo)
		stale, err := pageNode.GetGlobalIPBlacklistSettings(ctx)
		require.NoError(t, err)
		requireGlobalIPBlacklistDecision(t, pageNode, "203.0.113.10", false, nil)
		require.NoError(t, deleteNode.deleteUserAndBlockRegistrationIP(ctx, "203.0.113.10", "192.0.2.1", func(context.Context) error { return nil }))
		stale.Enabled = enabled // Neither an outdated list nor a disable may erase the ban.
		_, err = pageNode.SetGlobalIPBlacklistSettings(ctx, stale, "192.0.2.1")
		require.Equal(t, "IP_BLACKLIST_VERSION_CONFLICT", infraerrors.Reason(err))
		latest, err := pageNode.GetGlobalIPBlacklistSettings(ctx)
		require.NoError(t, err)
		require.True(t, latest.Enabled)
		require.Equal(t, []string{"203.0.113.10"}, latest.Rules)
		require.NotEqual(t, stale.Version, latest.Version)
		expireGlobalIPBlacklistCache(pageNode)
		requireGlobalIPBlacklistDecision(t, pageNode, "203.0.113.10", true, nil)
		requireGlobalIPBlacklistDecision(t, deleteNode, "203.0.113.10", true, nil)
		// Once the administrator reviews a fresh snapshot, explicit removal works.
		latest.Rules = []string{}
		_, err = pageNode.SetGlobalIPBlacklistSettings(ctx, latest, "192.0.2.1")
		require.NoError(t, err)
		requireGlobalIPBlacklistDecision(t, pageNode, "203.0.113.10", false, nil)
	}
}

func TestGlobalIPBlacklistConcurrentSavesAcrossInstancesRejectStaleVersion(t *testing.T) {
	ctx := context.Background()
	repo := &globalIPBlacklistTxSettingRepo{panelRateLimitSettingRepo: &panelRateLimitSettingRepo{}}
	snapshot, err := newPanelRateLimitTestService(repo).GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	type result struct {
		rule string
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, rule := range []string{"203.0.113.10", "203.0.113.20"} {
		go func(rule string) {
			<-start
			svc := newPanelRateLimitTestService(repo)
			_, err := svc.SetGlobalIPBlacklistSettings(ctx, &GlobalIPBlacklistSettings{
				Enabled: true, Rules: []string{rule}, Version: snapshot.Version,
			}, "192.0.2.1")
			results <- result{rule: rule, err: err}
		}(rule)
	}
	close(start)
	successes, conflicts, winningRule := 0, 0, ""
	for range 2 {
		select {
		case got := <-results:
			if got.err == nil {
				successes++
				winningRule = got.rule
			} else {
				require.Equal(t, "IP_BLACKLIST_VERSION_CONFLICT", infraerrors.Reason(got.err))
				conflicts++
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent saves did not finish")
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	latest, err := newPanelRateLimitTestService(repo).GetGlobalIPBlacklistSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{winningRule}, latest.Rules)
}

func TestGlobalIPBlacklistSaveRequiresVersion(t *testing.T) {
	repo := &panelRateLimitSettingRepo{}
	svc := newPanelRateLimitTestService(repo)
	_, err := svc.SetGlobalIPBlacklistSettings(context.Background(), &GlobalIPBlacklistSettings{}, "192.0.2.1")
	require.Equal(t, "IP_BLACKLIST_VERSION_REQUIRED", infraerrors.Reason(err))
	require.Empty(t, repo.values)
}
