package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type scheduledGroupsPlanRepo struct {
	service.ScheduledTestPlanRepository
	plan *service.ScheduledTestPlan
}

func (r *scheduledGroupsPlanRepo) GetByID(context.Context, int64) (*service.ScheduledTestPlan, error) {
	return r.plan, nil
}

func (r *scheduledGroupsPlanRepo) Update(_ context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	r.plan = plan
	return plan, nil
}

func TestScheduledTestUpdateSuccessGroupSelection(t *testing.T) {
	for _, tt := range []struct {
		name       string
		body       string
		wantGroups []int64
		wantLegacy int64
	}{
		{name: "replace selection", body: `{"success_add_group_ids":[11,12]}`, wantGroups: []int64{11, 12}},
		{name: "clear legacy and multiple selection", body: `{"success_add_group_ids":[]}`, wantGroups: []int64{}},
		{name: "omitted selection is preserved", body: `{"enabled":false}`, wantGroups: []int64{8}, wantLegacy: 9},
		{name: "multiple selection wins", body: `{"success_add_group_ids":[11,12],"success_add_group_id":9}`, wantGroups: []int64{11, 12}},
		{name: "legacy client replaces selection", body: `{"success_add_group_id":10}`, wantGroups: []int64{}, wantLegacy: 10},
		{name: "legacy client clears selection", body: `{"success_add_group_id":0}`, wantGroups: []int64{}},
		{name: "deduplicates selection", body: `{"success_add_group_ids":[11,12,11]}`, wantGroups: []int64{11, 12}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			legacyID := int64(9)
			repo := &scheduledGroupsPlanRepo{plan: &service.ScheduledTestPlan{
				ID: 1, AccountID: 1, CronExpression: "*/30 * * * *",
				SuccessAddGroupID: &legacyID, SuccessAddGroupIDs: []int64{8},
			}}
			handler := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
			router := gin.New()
			router.PUT("/plans/:id", handler.Update)
			request := httptest.NewRequest(http.MethodPut, "/plans/1", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if !reflect.DeepEqual(repo.plan.SuccessAddGroupIDs, tt.wantGroups) {
				t.Fatalf("groups = %v, want %v", repo.plan.SuccessAddGroupIDs, tt.wantGroups)
			}
			if tt.wantLegacy == 0 {
				if repo.plan.SuccessAddGroupID != nil {
					t.Fatalf("legacy group was not cleared: %d", *repo.plan.SuccessAddGroupID)
				}
			} else if repo.plan.SuccessAddGroupID == nil || *repo.plan.SuccessAddGroupID != tt.wantLegacy {
				t.Fatalf("legacy group = %v, want %d", repo.plan.SuccessAddGroupID, tt.wantLegacy)
			}
		})
	}
}
