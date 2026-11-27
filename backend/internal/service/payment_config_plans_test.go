//go:build unit

package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestFilterPlansByActiveGroupIDs(t *testing.T) {
	activePlan := &dbent.SubscriptionPlan{ID: 1, GroupID: 10}
	inactiveGroupPlan := &dbent.SubscriptionPlan{ID: 2, GroupID: 20}
	otherActivePlan := &dbent.SubscriptionPlan{ID: 3, GroupID: 10}

	got := filterPlansByActiveGroupIDs(
		[]*dbent.SubscriptionPlan{activePlan, inactiveGroupPlan, otherActivePlan},
		map[int64]struct{}{10: {}},
	)

	require.Equal(t, []*dbent.SubscriptionPlan{activePlan, otherActivePlan}, got)
}
