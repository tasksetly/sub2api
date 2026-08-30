package admin

import (
	"context"
	"log/slog"
)

// enrichUpstreamProviders 给账号列表回填「这个账号来自哪个上游」的名称。
//
// accounts 表只存了 upstream_provider_id，前端要显示名称。让前端自己再拉一次
// 上游列表拼接是可行的，但账号页并不一定有上游管理权限的上下文，且分页后拼接
// 容易漏——所以在这里一次性批量补齐。
//
// 失败不致命：来源列空着总比整个账号列表打不开好，只记日志。
func (h *AccountHandler) enrichUpstreamProviders(ctx context.Context, items []AccountWithConcurrency) {
	if h.upstreamProviders == nil || len(items) == 0 {
		return
	}

	seen := make(map[int64]struct{})
	for i := range items {
		account := items[i].Account
		if account == nil || account.UpstreamProviderID == nil {
			continue
		}
		seen[*account.UpstreamProviderID] = struct{}{}
	}
	if len(seen) == 0 {
		return
	}

	ids := make([]int64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	names, err := h.upstreamProviders.ProviderNamesByIDs(ctx, ids)
	if err != nil {
		slog.Warn("account_list_upstream_provider_names_failed", "error", err)
		return
	}
	for i := range items {
		account := items[i].Account
		if account == nil || account.UpstreamProviderID == nil {
			continue
		}
		if name, ok := names[*account.UpstreamProviderID]; ok {
			account.UpstreamProviderName = name
		}
	}
}
