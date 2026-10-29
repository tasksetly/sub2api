package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *upstreamProviderRepository) loadFinance(ctx context.Context, providers []*service.UpstreamProvider) error {
	if len(providers) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(providers))
	index := make(map[int64]*service.UpstreamProvider, len(providers))
	for _, p := range providers {
		ids = append(ids, p.ID)
		index[p.ID] = p
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT upstream_provider_id, snapshot FROM upstream_provider_finance WHERE upstream_provider_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("load upstream finance: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		var finance service.UpstreamFinance
		if err := json.Unmarshal(raw, &finance); err != nil {
			return fmt.Errorf("decode upstream finance: %w", err)
		}
		if p := index[id]; p != nil && finance.ProviderType == p.ProviderType && finance.BaseURL == p.BaseURL {
			p.Finance = &finance
		}
	}
	return rows.Err()
}

func (r *upstreamProviderRepository) saveFinance(ctx context.Context, id int64, finance *service.UpstreamFinance) error {
	if finance == nil {
		return nil
	}
	raw, err := json.Marshal(finance)
	if err != nil {
		return fmt.Errorf("encode upstream finance: %w", err)
	}
	_, err = r.sql.ExecContext(ctx, `
        INSERT INTO upstream_provider_finance (upstream_provider_id, snapshot, synced_at)
        VALUES ($1, $2::jsonb, $3)
        ON CONFLICT (upstream_provider_id) DO UPDATE
        SET snapshot = EXCLUDED.snapshot, synced_at = EXCLUDED.synced_at
        WHERE upstream_provider_finance.synced_at <= EXCLUDED.synced_at
    `, id, string(raw), finance.SyncedAt)
	if err != nil {
		return fmt.Errorf("save upstream finance: %w", err)
	}
	return nil
}
