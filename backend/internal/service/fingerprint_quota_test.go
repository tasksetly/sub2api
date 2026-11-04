package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- 测试替身 ---

type fingerprintLockCacheStub struct {
	mu       sync.Mutex
	held     map[string]string
	keysSeen []string
	failWith error
}

func newFingerprintLockCacheStub() *fingerprintLockCacheStub {
	return &fingerprintLockCacheStub{held: map[string]string{}}
}

func (c *fingerprintLockCacheStub) TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failWith != nil {
		return false, c.failWith
	}
	c.keysSeen = append(c.keysSeen, key)
	if _, taken := c.held[key]; taken {
		return false, nil
	}
	c.held[key] = owner
	return true, nil
}

func (c *fingerprintLockCacheStub) ReleaseLeaderLock(ctx context.Context, key, owner string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.held[key]; ok && cur == owner {
		delete(c.held, key)
	}
	return nil
}

type fingerprintCounterStub struct {
	perAccount map[int64]int64
	global     int64
	err        error
}

func (c *fingerprintCounterStub) CountSince(ctx context.Context, accountID int64, since time.Time) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.perAccount[accountID], nil
}

func (c *fingerprintCounterStub) CountAllSince(ctx context.Context, since time.Time) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.global, nil
}

func newFingerprintQuotaForTest(cache LeaderLockCache, counter FingerprintQuotaCounter, s FingerprintSettings) *FingerprintQuota {
	return NewFingerprintQuota(cache, nil, counter, func(context.Context) FingerprintSettings { return s })
}

// --- 测试 ---

func TestQuotaConcurrencyGateIsClusterWide(t *testing.T) {
	cache := newFingerprintLockCacheStub()
	q := newFingerprintQuotaForTest(cache, &fingerprintCounterStub{}, FingerprintSettings{
		MaxConcurrency:    2,
		DailyLimitGlobal:  100,
		DailyLimitAccount: 100,
	})
	ctx := context.Background()

	r1, skip1 := q.Acquire(ctx, 1)
	if skip1 != "" || r1 == nil {
		t.Fatalf("第 1 次应获得槽位，got skip=%q", skip1)
	}
	r2, skip2 := q.Acquire(ctx, 2)
	if skip2 != "" || r2 == nil {
		t.Fatalf("第 2 次应获得槽位，got skip=%q", skip2)
	}
	_, skip3 := q.Acquire(ctx, 3)
	if skip3 != FingerprintSkipConcurrency {
		t.Fatalf("第 3 次应被并发闸拦下，got %q", skip3)
	}

	// 闸必须走 LeaderLockCache（Redis），而不是进程内计数——多副本下进程内闸等于没限。
	if len(cache.keysSeen) == 0 {
		t.Fatal("并发闸没有经过 LeaderLockCache，多副本部署下形同虚设")
	}
	for _, k := range cache.keysSeen {
		if !strings.HasPrefix(k, "fingerprint:slot:") {
			t.Fatalf("意外的锁 key %q", k)
		}
	}
}

func TestQuotaReleaseFreesSlot(t *testing.T) {
	cache := newFingerprintLockCacheStub()
	q := newFingerprintQuotaForTest(cache, &fingerprintCounterStub{}, FingerprintSettings{
		MaxConcurrency:    1,
		DailyLimitGlobal:  100,
		DailyLimitAccount: 100,
	})
	ctx := context.Background()

	release, skip := q.Acquire(ctx, 1)
	if skip != "" {
		t.Fatalf("首次应获得槽位，got %q", skip)
	}
	if _, skip := q.Acquire(ctx, 2); skip != FingerprintSkipConcurrency {
		t.Fatalf("槽位占满时应拦下，got %q", skip)
	}
	release()
	if _, skip := q.Acquire(ctx, 3); skip != "" {
		t.Fatalf("释放后应能再次获得槽位，got %q", skip)
	}
}

// Review Focus #5：cron 被改成每分钟时，每日次数上限必须真的拦得住。
func TestQuotaDailyLimitBlocksRunawayCron(t *testing.T) {
	ctx := context.Background()

	t.Run("全局上限", func(t *testing.T) {
		q := newFingerprintQuotaForTest(newFingerprintLockCacheStub(),
			&fingerprintCounterStub{global: 5},
			FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 5, DailyLimitAccount: 100})
		if _, skip := q.Acquire(ctx, 1); skip != FingerprintSkipDailyGlobal {
			t.Fatalf("达到全局上限应拦下，got %q", skip)
		}
	})

	t.Run("单账号上限", func(t *testing.T) {
		q := newFingerprintQuotaForTest(newFingerprintLockCacheStub(),
			&fingerprintCounterStub{perAccount: map[int64]int64{7: 2}},
			FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 100, DailyLimitAccount: 2})
		if _, skip := q.Acquire(ctx, 7); skip != FingerprintSkipDailyAccount {
			t.Fatalf("达到单账号上限应拦下，got %q", skip)
		}
		if _, skip := q.Acquire(ctx, 8); skip != "" {
			t.Fatalf("别的账号不该被连坐，got %q", skip)
		}
	})
}

func TestQuotaChecksDailyLimitBeforeTakingSlot(t *testing.T) {
	cache := newFingerprintLockCacheStub()
	q := newFingerprintQuotaForTest(cache,
		&fingerprintCounterStub{global: 99},
		FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 99, DailyLimitAccount: 100})

	if _, skip := q.Acquire(context.Background(), 1); skip != FingerprintSkipDailyGlobal {
		t.Fatalf("got %q", skip)
	}
	if len(cache.keysSeen) != 0 {
		t.Fatal("配额已满时不应再去抢并发槽位")
	}
}

func TestQuotaDailyWindowStartsAtUTCMidnight(t *testing.T) {
	now := time.Date(2026, 11, 4, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))
	start := fingerprintDailyWindowStart(now)
	if start.Location() != time.UTC {
		t.Fatalf("窗口起点时区 = %v, want UTC", start.Location())
	}
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		t.Fatalf("窗口起点 = %v, want UTC 零点", start)
	}
	if !start.Before(now) {
		t.Fatalf("窗口起点 %v 应早于 now %v", start, now)
	}
}

// 计数查询失败时放行：宁可多跑一次检测，也不要因为数据库抖动让整个功能静默停摆。
func TestQuotaAllowsWhenCounterFails(t *testing.T) {
	q := newFingerprintQuotaForTest(newFingerprintLockCacheStub(),
		&fingerprintCounterStub{err: errors.New("db down")},
		FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 1, DailyLimitAccount: 1})

	if _, skip := q.Acquire(context.Background(), 1); skip != "" {
		t.Fatalf("计数失败时应放行，got %q", skip)
	}
}

func TestQuotaSkippedAcquireReturnsNoopRelease(t *testing.T) {
	q := newFingerprintQuotaForTest(newFingerprintLockCacheStub(),
		&fingerprintCounterStub{global: 10},
		FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 10, DailyLimitAccount: 100})

	release, skip := q.Acquire(context.Background(), 1)
	if skip == "" {
		t.Fatal("本应被拦下")
	}
	if release == nil {
		t.Fatal("被拦下时也要返回可安全调用的 release，避免调用方 nil 解引用")
	}
	release() // 不得 panic
}
