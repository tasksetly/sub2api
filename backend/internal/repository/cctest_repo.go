package repository

import (
	"context"
	"database/sql"
	"errors"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type cctestRepository struct{ db *sql.DB }

func NewCCTestRepository(db *sql.DB) service.CCTestRepository { return &cctestRepository{db: db} }

func (r *cctestRepository) SaveSettings(ctx context.Context, version, raw string) (bool, error) {
	var result sql.Result
	var err error
	if version == "" {
		result, err = r.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO NOTHING`, service.SettingKeyCCTest, raw)
	} else {
		result, err = r.db.ExecContext(ctx, `UPDATE settings SET value=$2,updated_at=NOW() WHERE key=$1 AND value::jsonb->>'version'=$3`, service.SettingKeyCCTest, raw, version)
	}
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *cctestRepository) Create(ctx context.Context, runs []*service.CCTestRun, admin bool) error {
	if len(runs) == 0 {
		return errors.New("empty CCTest batch")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize admission across instances; quota checks and inserts are atomic.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(267001)`); err != nil {
		return err
	}
	var active, daily, global int
	if err := tx.QueryRowContext(ctx, `SELECT
COUNT(*) FILTER(WHERE owner_id=$1 AND status IN ('queued','starting','running') AND expires_at>NOW()),
COUNT(*) FILTER(WHERE owner_id=$1 AND created_at>NOW()-INTERVAL '24 hours'),
COUNT(*) FILTER(WHERE status IN ('queued','starting','running') AND expires_at>NOW())
FROM cctest_runs`, runs[0].OwnerID).Scan(&active, &daily, &global); err != nil {
		return err
	}
	limit := 1
	if admin {
		limit = 20
	}
	if active+len(runs) > limit || global+len(runs) > 100 || (!admin && daily+len(runs) > 10) {
		return infraerrors.TooManyRequests("CCTEST_LIMIT", "CCTest limit reached: users may run one test at a time and ten per 24 hours; admins may queue twenty")
	}
	for _, run := range runs {
		if err := tx.QueryRowContext(ctx, `INSERT INTO cctest_runs
(id,owner_id,account_id,api_key_id,target_name,model,token_audit,relay_hash,credentials,relay_url)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at,expires_at`, run.ID, run.OwnerID, run.AccountID, run.APIKeyID, run.TargetName, run.Model, run.TokenAudit, run.RelayHash, run.Credentials, run.RelayURL).Scan(&run.CreatedAt, &run.ExpiresAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const cctestColumns = `id,owner_id,account_id,api_key_id,target_name,model,token_audit,status,task_id,relay_hash,credentials,relay_url,request_count,result,error,created_at,expires_at,lease_id`

func scanCCTest(row interface{ Scan(...any) error }) (*service.CCTestRun, error) {
	run := &service.CCTestRun{}
	err := row.Scan(&run.ID, &run.OwnerID, &run.AccountID, &run.APIKeyID, &run.TargetName, &run.Model, &run.TokenAudit, &run.Status, &run.TaskID, &run.RelayHash, &run.Credentials, &run.RelayURL, &run.RequestCount, &run.Result, &run.Error, &run.CreatedAt, &run.ExpiresAt, &run.LeaseID)
	return run, err
}

func (r *cctestRepository) List(ctx context.Context, ownerID int64, admin bool, before string) ([]*service.CCTestRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+cctestColumns+` FROM cctest_runs
WHERE (($2 AND account_id IS NOT NULL) OR (NOT $2 AND owner_id=$1 AND api_key_id IS NOT NULL))
AND ($3='' OR (created_at,id)<(SELECT created_at,id FROM cctest_runs WHERE id=NULLIF($3,'')::uuid)) ORDER BY created_at DESC,id DESC LIMIT 50`, ownerID, admin, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*service.CCTestRun, 0)
	for rows.Next() {
		run, err := scanCCTest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

func (r *cctestRepository) Cancel(ctx context.Context, id string, ownerID int64, admin bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE cctest_runs SET status='cancelled',credentials='',lease_id='',lease_until=NULL
WHERE id=$1 AND (($3 AND account_id IS NOT NULL) OR (NOT $3 AND owner_id=$2 AND api_key_id IS NOT NULL)) AND status IN ('queued','starting','running')`, id, ownerID, admin)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return infraerrors.NotFound("CCTEST_NOT_ACTIVE", "test is unavailable or already finished")
	}
	return nil
}

func (r *cctestRepository) Claim(ctx context.Context) (*service.CCTestRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(267002)`); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM cctest_runs WHERE status IN ('done','error','cancelled') AND created_at<NOW()-INTERVAL '90 days'`); err != nil {
		return nil, err
	}
	// Never repeat a POST after a crashed/ambiguous submission: it spends a credit.
	if _, err = tx.ExecContext(ctx, `UPDATE cctest_runs SET status='error',error=CASE WHEN status='starting' THEN 'Submission interrupted; check CCTest credit history before retrying' ELSE 'Test timed out' END,credentials='',lease_id='',lease_until=NULL
WHERE status IN ('queued','starting','running') AND (expires_at<=NOW() OR (status='starting' AND lease_until<NOW()))`); err != nil {
		return nil, err
	}
	run, err := scanCCTest(tx.QueryRowContext(ctx, `SELECT `+cctestColumns+` FROM cctest_runs
WHERE ((status='running' AND next_poll_at<=NOW()) OR (status='queued' AND (SELECT COUNT(*) FROM cctest_runs WHERE status IN ('starting','running'))<4))
AND (lease_until IS NULL OR lease_until<NOW()) ORDER BY next_poll_at,created_at LIMIT 1 FOR UPDATE SKIP LOCKED`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	run.LeaseID = uuid.NewString()
	if run.Status == "queued" {
		run.Status = "starting"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cctest_runs SET status=$2,lease_id=$3,lease_until=NOW()+INTERVAL '45 seconds' WHERE id=$1`, run.ID, run.Status, run.LeaseID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *cctestRepository) Update(ctx context.Context, run *service.CCTestRun) error {
	_, err := r.db.ExecContext(ctx, `UPDATE cctest_runs SET status=$3,task_id=$4,result=$5,error=$6,
credentials=CASE WHEN $3 IN ('done','error','cancelled') THEN '' ELSE credentials END,
lease_id='',lease_until=NULL,next_poll_at=NOW()+INTERVAL '3 seconds'
WHERE id=$1 AND lease_id=$2 AND status IN ('starting','running')`, run.ID, run.LeaseID, run.Status, run.TaskID, []byte(run.Result), run.Error)
	return err
}

func (r *cctestRepository) ConsumeRelay(ctx context.Context, hash, model string) (*service.CCTestRun, error) {
	run, err := scanCCTest(r.db.QueryRowContext(ctx, `UPDATE cctest_runs SET request_count=request_count+1
WHERE relay_hash=$1 AND model=$2 AND status IN ('starting','running') AND expires_at>NOW()
AND request_count<CASE WHEN token_audit THEN 60 ELSE 30 END RETURNING `+cctestColumns, hash, model))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.Unauthorized("CCTEST_RELAY_EXPIRED", "test credential expired, model mismatched, or request limit reached")
	}
	return run, err
}
