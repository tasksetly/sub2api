package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// fingerprintSlotLockTTL 是并发槽位锁的崩溃兜底时长。
// 必须大于单次检测的最坏耗时（SDK 默认 120s 超时），否则锁会在检测中途过期。
const fingerprintSlotLockTTL = 6 * time.Minute

// FingerprintQuotaCounter 统计已发生的检测次数，用于每日配额。
//
// 计数走数据库而不是 Redis 计数器：每次检测都会落一条结果，计数天然跨实例、
// 跨重启准确；Redis 计数器被驱逐就会让当日配额凭空翻倍。
type FingerprintQuotaCounter interface {
	CountSince(ctx context.Context, accountID int64, since time.Time) (int64, error)
	CountAllSince(ctx context.Context, since time.Time) (int64, error)
}

// FingerprintQuota 是指纹检测的成本刹车：集群级并发闸 + 每日次数上限。
//
// 并发闸只限同时跑几个，不限一天跑多少；两者都要，否则一个被改成每分钟的 cron
// 就能绕过并发闸持续烧钱。
type FingerprintQuota struct {
	lockCache  LeaderLockCache
	db         *sql.DB
	counter    FingerprintQuotaCounter
	settings   func(context.Context) FingerprintSettings
	instanceID string
}

// NewFingerprintQuota 创建配额闸。
func NewFingerprintQuota(
	lockCache LeaderLockCache,
	db *sql.DB,
	counter FingerprintQuotaCounter,
	settings func(context.Context) FingerprintSettings,
) *FingerprintQuota {
	return &FingerprintQuota{
		lockCache:  lockCache,
		db:         db,
		counter:    counter,
		settings:   settings,
		instanceID: uuid.NewString(),
	}
}

// Acquire 申请一次检测的许可。
//
// skipReason 非空表示本次应跳过——调用方必须据此写一条带 skip_reason 的结果，
// 不能静默跳过。返回的 release 在任何情况下都非 nil，可安全调用。
func (q *FingerprintQuota) Acquire(ctx context.Context, accountID int64) (func(), string) {
	noop := func() {}
	if q == nil {
		return noop, ""
	}

	settings := q.currentSettings(ctx)

	// 先查每日配额：配额已满时不该再去抢并发槽位，抢到还得立刻还回去。
	if reason := q.checkDailyLimits(ctx, accountID, settings); reason != "" {
		return noop, reason
	}

	release, ok := q.acquireSlot(ctx, settings.MaxConcurrency)
	if !ok {
		return noop, FingerprintSkipConcurrency
	}
	return release, ""
}

// checkDailyLimits 返回被拦下的原因，空字符串表示放行。
//
// 计数查询出错时放行：宁可多跑一次检测，也不要因为数据库抖动让整个功能静默停摆
// ——那会变成"看起来在跑，实际一次都没测"的沉默失败。
func (q *FingerprintQuota) checkDailyLimits(ctx context.Context, accountID int64, settings FingerprintSettings) string {
	if q.counter == nil {
		return ""
	}
	since := fingerprintDailyWindowStart(time.Now())

	if settings.DailyLimitGlobal > 0 {
		used, err := q.counter.CountAllSince(ctx, since)
		if err == nil && used >= int64(settings.DailyLimitGlobal) {
			return FingerprintSkipDailyGlobal
		}
	}
	if settings.DailyLimitAccount > 0 {
		used, err := q.counter.CountSince(ctx, accountID, since)
		if err == nil && used >= int64(settings.DailyLimitAccount) {
			return FingerprintSkipDailyAccount
		}
	}
	return ""
}

// acquireSlot 逐个尝试并发槽位，抢到哪个用哪个。
//
// 槽位锁走 LeaderLockCache（Redis 实现），所以闸是集群级的。进程内信号量在
// N 个副本下实际放行 maxConcurrency × N 个检测，等于没限。
func (q *FingerprintQuota) acquireSlot(ctx context.Context, maxConcurrency int) (func(), bool) {
	if maxConcurrency <= 0 {
		maxConcurrency = FingerprintDefaultMaxConcurrency
	}
	for i := 0; i < maxConcurrency; i++ {
		key := fmt.Sprintf("fingerprint:slot:%d", i)
		lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		release, ok := tryAcquireSingletonLeaderLock(lockCtx, q.lockCache, q.db, key, q.instanceID, fingerprintSlotLockTTL)
		cancel()
		if ok {
			return release, true
		}
	}
	return nil, false
}

func (q *FingerprintQuota) currentSettings(ctx context.Context) FingerprintSettings {
	if q.settings == nil {
		return FingerprintSettings{
			MaxConcurrency:    FingerprintDefaultMaxConcurrency,
			DailyLimitGlobal:  FingerprintDefaultDailyLimitGlobal,
			DailyLimitAccount: FingerprintDefaultDailyLimitAccount,
		}
	}
	return q.settings(ctx)
}

// fingerprintDailyWindowStart 返回当日配额窗口的起点（UTC 零点）。
// 固定用 UTC，避免部署时区变更让某一天的配额凭空翻倍或清零。
func fingerprintDailyWindowStart(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
