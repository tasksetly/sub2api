package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// WithGlobalIPBlacklistUpdate owns the commit so the service only publishes its
// cache after the blacklist and any accompanying user deletion are durable.
// All blacklist writers share the same transaction-scoped lock across nodes.
func (r *settingRepository) WithGlobalIPBlacklistUpdate(ctx context.Context, fn func(context.Context) error) error {
	if ent.TxFromContext(ctx) != nil {
		return fmt.Errorf("global IP blacklist update must own its transaction")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := ent.NewTxContext(ctx, tx)
	exec := sqlExecutorFromEntClient(tx.Client())
	if exec == nil {
		return fmt.Errorf("global IP blacklist SQL executor is not configured")
	}
	// PostgreSQL serializes writers across instances. Do not wait on the
	// process-wide sync.Mutex registry here: that wait ignores cancellation.
	if _, err := exec.ExecContext(txCtx, "SELECT pg_advisory_xact_lock($1)", advisoryLockHash("settings:"+service.SettingKeyGlobalIPBlacklist)); err != nil {
		return err
	}
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit()
}
