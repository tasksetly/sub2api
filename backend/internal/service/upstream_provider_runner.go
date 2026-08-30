package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// 30 分钟同步一次：余额和倍率不是高频变化的量，更密只是徒增上游负载。
	upstreamProviderSyncInterval = 30 * time.Minute
	// TTL 必须大于一轮同步的最坏耗时，否则锁会在跑到一半时过期。
	// 每个上游最多 5 次 HTTP 调用 × 15s 超时，留足余量。
	upstreamProviderSyncLockTTL   = 10 * time.Minute
	upstreamProviderSyncLockKey   = "upstream:provider:sync:leader"
	upstreamProviderSyncStartWait = time.Minute
)

// UpstreamProviderSyncRunner 周期性同步所有启用了同步的上游供应商。
//
// 只读同步：拉余额、并发、分组倍率写入快照供比价，绝不改本地账号的
// concurrency/rate_multiplier。
type UpstreamProviderSyncRunner struct {
	svc       *UpstreamProviderService
	lockCache LeaderLockCache
	db        *sql.DB

	instanceID string

	mu      sync.Mutex
	started bool
	stopped bool

	parentCtx    context.Context
	parentCancel context.CancelFunc
	wg           sync.WaitGroup
}

func NewUpstreamProviderSyncRunner(
	svc *UpstreamProviderService, lockCache LeaderLockCache, db *sql.DB,
) *UpstreamProviderSyncRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamProviderSyncRunner{
		svc:          svc,
		lockCache:    lockCache,
		db:           db,
		instanceID:   uuid.NewString(),
		parentCtx:    ctx,
		parentCancel: cancel,
	}
}

// ProvideUpstreamProviderSyncRunner 启动进程级的周期同步。
func ProvideUpstreamProviderSyncRunner(
	svc *UpstreamProviderService, lockCache LeaderLockCache, db *sql.DB,
) *UpstreamProviderSyncRunner {
	runner := NewUpstreamProviderSyncRunner(svc, lockCache, db)
	runner.Start()
	return runner
}

func (r *UpstreamProviderSyncRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.wg.Add(1)
	r.mu.Unlock()
	go r.runLoop()
}

func (r *UpstreamProviderSyncRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.parentCancel()
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *UpstreamProviderSyncRunner) runLoop() {
	defer r.wg.Done()

	// 启动后等一会儿再首次同步：进程刚起来时先让常规流量把缓存热起来，
	// 不和启动阶段抢资源。
	select {
	case <-r.parentCtx.Done():
		return
	case <-time.After(upstreamProviderSyncStartWait):
	}
	r.runOnce()

	ticker := time.NewTicker(upstreamProviderSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.parentCtx.Done():
			return
		case <-ticker.C:
			r.runOnce()
		}
	}
}

// runOnce 取到 leader 锁后同步一轮。拿不到锁说明别的实例正在跑，跳过本轮。
func (r *UpstreamProviderSyncRunner) runOnce() {
	lockCtx, cancel := context.WithTimeout(r.parentCtx, 2*time.Second)
	release, acquired := tryAcquireSingletonLeaderLock(
		lockCtx, r.lockCache, r.db,
		upstreamProviderSyncLockKey, r.instanceID, upstreamProviderSyncLockTTL,
	)
	cancel()
	if !acquired {
		return
	}
	defer release()

	succeeded, failed := r.svc.SyncAll(r.parentCtx)
	if succeeded > 0 || failed > 0 {
		slog.Info("upstream_provider_sync_cycle",
			"succeeded", succeeded, "failed", failed)
	}
}
