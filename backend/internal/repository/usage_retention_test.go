package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageRetentionSaveNoticeIsAtomic(t *testing.T) {
	for _, tt := range []struct {
		name             string
		same, failNotice bool
	}{
		{name: "changed policy publishes notice"},
		{name: "unchanged policy avoids duplicate notice", same: true},
		{name: "notice failure rolls back policy", failNotice: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := &settingRepository{client: client}
			old := service.UsageRetentionSettings{Enabled: true, RetentionDays: 90}
			next := service.UsageRetentionSettings{Enabled: true, RetentionDays: 30}
			if tt.same {
				next = old
			}
			mock.ExpectBegin()
			// An existing row makes INSERT ... DO NOTHING RETURNING id return no rows.
			mock.ExpectQuery(`INSERT INTO "settings"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery(`SELECT .* FROM "settings" .* FOR UPDATE`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "key", "value", "updated_at"}).AddRow(1, service.SettingKeyUsageRetention, `{"enabled":true,"retention_days":90}`, time.Now()))
			if !tt.same {
				insert := mock.ExpectQuery(`INSERT INTO "announcements"`)
				if tt.failNotice {
					insert.WillReturnError(errors.New("notice failed"))
				} else {
					insert.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
				}
			}
			if tt.failNotice {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec(`UPDATE "settings"`).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = repo.SetUsageRetentionSettings(context.Background(), next, old, 1)
			if tt.failNotice {
				require.ErrorContains(t, err, "notice failed")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
