package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// UpstreamCostSupplierRepository exposes all supplier/model/day cost reads from
// the durable daily rollup table.
type UpstreamCostSupplierRepository interface {
	GetSupplierCostStatsFromLedger(ctx context.Context, filters usagestats.UsageLogFilters) ([]usagestats.SupplierCostStat, error)
	GetUpstreamCostDailyFromLedger(ctx context.Context, filters usagestats.UsageLogFilters) ([]usagestats.UpstreamCostDailyStat, error)
}

type UpstreamCostLedgerSyncRepository interface {
	SyncUpstreamCostLedger(ctx context.Context, now time.Time) error
}
