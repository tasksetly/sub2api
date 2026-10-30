package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type scheduledSuccessGroupsAccountRepo struct {
	AccountRepository
	added     []int64
	failGroup int64
}

func (r *scheduledSuccessGroupsAccountRepo) AddToGroup(_ context.Context, accountID, groupID int64, priority int) error {
	r.added = append(r.added, groupID)
	if groupID == r.failGroup {
		return errors.New("group unavailable")
	}
	return nil
}

func (r *scheduledSuccessGroupsAccountRepo) RemoveFromGroup(context.Context, int64, int64) error {
	return nil
}

func TestScheduledTestSuccessAddsSelectedGroups(t *testing.T) {
	legacyID := int64(9)
	for _, tt := range []struct {
		name      string
		groups    []int64
		legacy    *int64
		failGroup int64
		want      []int64
	}{
		{name: "multiple groups", groups: []int64{11, 12}, want: []int64{11, 12}},
		{name: "continues after one group fails", groups: []int64{11, 12}, failGroup: 11, want: []int64{11, 12}},
		{name: "legacy single group", legacy: &legacyID, want: []int64{9}},
		{name: "multiple groups take precedence", groups: []int64{11, 12}, legacy: &legacyID, want: []int64{11, 12}},
		{name: "no selection"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &scheduledSuccessGroupsAccountRepo{failGroup: tt.failGroup}
			runner := &ScheduledTestRunnerService{accountTestSvc: &AccountTestService{accountRepo: repo}}
			runner.applySuccessActions(context.Background(), &ScheduledTestPlan{
				AccountID: 1, SuccessAddGroupIDs: tt.groups, SuccessAddGroupID: tt.legacy,
			})
			if !reflect.DeepEqual(repo.added, tt.want) {
				t.Fatalf("added groups = %v, want %v", repo.added, tt.want)
			}
		})
	}
}
