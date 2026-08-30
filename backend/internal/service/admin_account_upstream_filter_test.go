//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 上游筛选必须原样传到仓储层，否则列表页筛了也没效果。
func TestAdminService_ListAccounts_PassesUpstreamFilter(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  int64
	}{
		{name: "no filter", input: 0, want: 0},
		{name: "any upstream", input: AccountListUpstreamAny, want: AccountListUpstreamAny},
		{name: "specific provider", input: 7, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountRepoStubForAdminList{}
			svc := &adminServiceImpl{accountRepo: repo}

			_, _, err := svc.ListAccounts(
				context.Background(), 1, 20, "", "", "", "", 0, "", "name", "asc", tt.input,
			)
			require.NoError(t, err)
			require.Equal(t, tt.want, repo.listWithFiltersUpstream)
		})
	}
}

// 「全选筛选结果 + 批量改」把列表的筛选条件原样发回来解析成目标 id。
// 上游条件漏掉的话，批量改会命中用户在列表上根本没看到的账号。
func TestResolveBulkUpdateTargetIDs_AppliesUpstreamFilter(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		want     int64
	}{
		{name: "empty means no filter", upstream: "", want: 0},
		{name: "any upstream", upstream: "any", want: AccountListUpstreamAny},
		{name: "specific provider", upstream: "7", want: 7},
		{name: "whitespace is trimmed", upstream: "  7  ", want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountRepoStubForBulkUpdate{
				listData: []Account{{ID: 11}, {ID: 12}},
			}
			svc := &adminServiceImpl{accountRepo: repo}

			ids, err := svc.resolveBulkUpdateTargetIDs(
				context.Background(), &BulkUpdateAccountFilters{Upstream: tt.upstream},
			)
			require.NoError(t, err)
			require.Equal(t, []int64{11, 12}, ids)
			require.Equal(t, tt.want, repo.lastListFilters.upstream)
		})
	}
}

// 非法上游 id 要报错而不是静默当成「不筛选」：后者会让批量改扩大到全部账号。
func TestResolveBulkUpdateTargetIDs_RejectsInvalidUpstream(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-3"} {
		t.Run(raw, func(t *testing.T) {
			repo := &accountRepoStubForBulkUpdate{listData: []Account{{ID: 11}}}
			svc := &adminServiceImpl{accountRepo: repo}

			_, err := svc.resolveBulkUpdateTargetIDs(
				context.Background(), &BulkUpdateAccountFilters{Upstream: raw},
			)
			require.Error(t, err)
			require.False(t, repo.listCalled, "must not list accounts when the filter is invalid")
		})
	}
}
