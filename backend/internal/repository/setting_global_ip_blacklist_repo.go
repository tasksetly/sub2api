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
	release, err := lockRepositoryScopedKeys(txCtx, tx.Client(), exec, "settings:"+service.SettingKeyGlobalIPBlacklist)
	if err != nil {
		return err
	}
	defer release()
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit()
}
