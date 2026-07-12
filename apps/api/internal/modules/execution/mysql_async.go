package execution

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (repository *MySQLRepository) EnqueueRun(ctx context.Context, run Run) (stored Run, existing bool, err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var lockedSystem string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM business_systems WHERE id = ? FOR UPDATE`, run.SystemID).Scan(&lockedSystem); err != nil {
		return Run{}, false, err
	}
	stored, err = scanRun(tx.QueryRowContext(ctx, selectRunSQL+` WHERE system_id = ? AND JSON_UNQUOTE(JSON_EXTRACT(summary, '$.summary.retryKey')) = ? ORDER BY created_at LIMIT 1`, run.SystemID, run.Summary.RetryKey))
	if err == nil {
		if err = tx.Commit(); err != nil {
			return Run{}, false, err
		}
		return stored, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, err
	}
	input, err := encodeJSON(run.InputVariables)
	if err != nil {
		return Run{}, false, err
	}
	summary, err := encodeRunSummary(run)
	if err != nil {
		return Run{}, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scenario_runs
        (id, system_id, scenario_id, scenario_version_id, environment_id, status, trigger_type, input_variables, summary, requested_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.SystemID, run.ScenarioID, run.ScenarioVersionID, run.EnvironmentID, run.Status, run.TriggerType,
		input, summary, run.RequestedBy, run.CreatedAt)
	if err != nil {
		return Run{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return Run{}, false, err
	}
	return run, false, nil
}

func (repository *MySQLRepository) LeaseNext(ctx context.Context, systemID, workerID string, now time.Time, duration time.Duration) (run Run, found bool, err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var runID string
	if err = tx.QueryRowContext(ctx, selectLeaseCandidateSQL, systemID, RunStatusQueued).Scan(&runID); errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return Run{}, false, nil
	} else if err != nil {
		return Run{}, false, err
	}
	run, err = scanRun(tx.QueryRowContext(ctx, selectRunSQL+" WHERE system_id = ? AND id = ? FOR UPDATE", systemID, runID))
	if err != nil {
		return Run{}, false, err
	}
	expires := now.Add(duration).UTC()
	run.Status, run.Summary.LeaseOwner, run.Summary.LeaseExpiresAt = RunStatusRunning, workerID, &expires
	summary, err := encodeRunSummary(run)
	if err != nil {
		return Run{}, false, err
	}
	result, err := tx.ExecContext(ctx, updateLeaseSQL, RunStatusRunning, summary, systemID, runID, RunStatusQueued)
	if err != nil {
		return Run{}, false, err
	}
	if err = requireUpdatedRun(result); err != nil {
		return Run{}, false, ErrLeaseLost
	}
	if err = tx.Commit(); err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

func (repository *MySQLRepository) Heartbeat(ctx context.Context, systemID, runID, workerID string, expires time.Time) error {
	result, err := repository.db.ExecContext(ctx, heartbeatLeaseSQL, expires.UTC().Format(time.RFC3339Nano), systemID, runID, RunStatusRunning, workerID)
	if err != nil {
		return err
	}
	if err := requireUpdatedRun(result); err != nil {
		return ErrLeaseLost
	}
	return nil
}

func (repository *MySQLRepository) CancelRun(ctx context.Context, systemID, runID string, now time.Time) (run Run, err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	run, err = scanRun(tx.QueryRowContext(ctx, selectRunSQL+" WHERE system_id = ? AND id = ? FOR UPDATE", systemID, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, err
	}
	if run.Status != RunStatusQueued && run.Status != RunStatusRunning {
		return Run{}, ErrRunNotCancellable
	}
	run.Status, run.FinishedAt = RunStatusCancelled, now.UTC()
	run.Summary.LeaseOwner, run.Summary.LeaseExpiresAt = "", nil
	summary, err := encodeRunSummary(run)
	if err != nil {
		return Run{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE scenario_runs SET status = ?, summary = ?, finished_at = ? WHERE system_id = ? AND id = ? AND status IN (?, ?)`,
		RunStatusCancelled, summary, run.FinishedAt, systemID, runID, RunStatusQueued, RunStatusRunning)
	if err != nil {
		return Run{}, err
	}
	if err = requireUpdatedRun(result); err != nil {
		return Run{}, ErrRunNotCancellable
	}
	if err = tx.Commit(); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (repository *MySQLRepository) RequeueExpired(ctx context.Context, systemID string, now time.Time) (int64, error) {
	result, err := repository.db.ExecContext(ctx, `UPDATE scenario_runs
        SET status = ?, summary = JSON_REMOVE(summary, '$.summary.leaseOwner', '$.summary.leaseExpiresAt')
        WHERE system_id = ? AND status = ?
          AND JSON_UNQUOTE(JSON_EXTRACT(summary, '$.summary.leaseExpiresAt')) <= ?`,
		RunStatusQueued, systemID, RunStatusRunning, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var _ AsyncRepository = (*MySQLRepository)(nil)
