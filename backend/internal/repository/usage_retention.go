package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Publish the notice atomically with the policy; concurrent saves serialize on the setting row.
func (r *settingRepository) SetUsageRetentionSettings(ctx context.Context, policy, fallback service.UsageRetentionSettings, actorID int64) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	initial, err := json.Marshal(fallback)
	if err != nil {
		return err
	}
	if err := client.Setting.Create().SetKey(service.SettingKeyUsageRetention).SetValue(string(initial)).SetUpdatedAt(time.Now()).
		OnConflictColumns(setting.FieldKey).DoNothing().Exec(ctx); err != nil && !isSQLNoRowsError(err) {
		return err
	}
	row, err := client.Setting.Query().Where(setting.KeyEQ(service.SettingKeyUsageRetention)).ForUpdate().Only(ctx)
	if err != nil {
		return err
	}
	var old service.UsageRetentionSettings
	if err := json.Unmarshal([]byte(row.Value), &old); err != nil {
		return err
	}
	if err := old.Validate(); err != nil {
		return err
	}
	if old != policy {
		content := "使用记录自动清理已关闭，后续使用明细不再按保留天数自动删除。已删除的数据无法恢复。"
		if policy.Enabled {
			content = fmt.Sprintf("使用记录保留策略已更新：保留最近 %d 天的请求使用明细，每 6 小时定时清理更早的数据。历史明细及相关统计查询可能受影响，请及时导出所需记录。此操作不会退还已扣费用，也不会重置余额、订阅或额度。已删除的数据无法恢复。", policy.RetentionDays)
		}
		if err := client.Announcement.Create().SetTitle("使用记录保留策略变更").SetContent(content).
			SetStatus(service.AnnouncementStatusActive).SetNotifyMode(service.AnnouncementNotifyModePopup).
			SetTargeting(service.AnnouncementTargeting{}).SetCreatedBy(actorID).SetUpdatedBy(actorID).Exec(ctx); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	if err := client.Setting.Update().Where(setting.IDEQ(row.ID)).SetValue(string(encoded)).SetUpdatedAt(time.Now()).Exec(ctx); err != nil {
		return err
	}
	return tx.Commit()
}
