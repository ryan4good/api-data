package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const selectRunSQL = `SELECT id, system_id, scenario_id, scenario_version_id, environment_id, status, trigger_type, input_variables, output_variables, summary, requested_by, started_at, finished_at, created_at FROM scenario_runs`
const selectAttemptSQL = `SELECT id, system_id, scenario_run_id, scenario_step_id, attempt_no, position, status, request_snapshot, response_snapshot, extracted_variables, error_message, duration_ms, started_at, finished_at, created_at FROM scenario_step_runs`
const selectAssertionSQL = `SELECT id, assertion_key, assertion_type, status, expected_value, actual_value, message, duration_ms FROM scenario_assertion_results`
const insertStepAttemptSQL = `INSERT INTO scenario_step_runs (id, system_id, scenario_run_id, scenario_step_id, attempt_no, position, status, request_snapshot, response_snapshot, extracted_variables, error_message, duration_ms, started_at, finished_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const insertAssertionSQL = `INSERT INTO scenario_assertion_results (id, system_id, step_run_id, assertion_key, assertion_type, status, expected_value, actual_value, message, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const selectLeaseCandidateSQL = `SELECT id FROM scenario_runs WHERE system_id = ? AND status = ? ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`
const updateLeaseSQL = `UPDATE scenario_runs SET status = ?, summary = ? WHERE system_id = ? AND id = ? AND status = ?`
const heartbeatLeaseSQL = `UPDATE scenario_runs SET summary = JSON_SET(summary, '$.summary.leaseExpiresAt', ?) WHERE system_id = ? AND id = ? AND status = ? AND JSON_UNQUOTE(JSON_EXTRACT(summary, '$.summary.leaseOwner')) = ?`

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (repository *MySQLRepository) CreateRun(ctx context.Context, run Run) error {
	input, err := encodeJSON(run.InputVariables)
	if err != nil {
		return err
	}
	summary, err := encodeRunSummary(run)
	if err != nil {
		return err
	}
	_, err = repository.db.ExecContext(ctx, `INSERT INTO scenario_runs
        (id, system_id, scenario_id, scenario_version_id, environment_id, status, trigger_type, input_variables, summary, requested_by, started_at, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.SystemID, run.ScenarioID, run.ScenarioVersionID, run.EnvironmentID, run.Status, run.TriggerType,
		input, summary, run.RequestedBy, run.StartedAt, run.CreatedAt)
	return err
}

func (repository *MySQLRepository) SaveAttempt(ctx context.Context, attempt StepAttempt) (err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	requestSnapshot, err := encodeJSON(attempt.RequestSnapshot)
	if err != nil {
		return err
	}
	responseSnapshot, err := encodeJSON(attempt.ResponseSnapshot)
	if err != nil {
		return err
	}
	extracted, err := encodeJSON(attempt.ExtractedVariables)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, insertStepAttemptSQL,
		attempt.ID, attempt.SystemID, attempt.RunID, attempt.StepID, attempt.AttemptNo, attempt.Position, attempt.Status,
		requestSnapshot, responseSnapshot, extracted, nullableString(attempt.ErrorMessage), attempt.DurationMS,
		attempt.StartedAt, attempt.FinishedAt, attempt.CreatedAt)
	if err != nil {
		return err
	}
	for _, assertion := range attempt.Assertions {
		expected, encodeErr := encodeJSON(assertion.Expected)
		if encodeErr != nil {
			return encodeErr
		}
		actual, encodeErr := encodeJSON(assertion.Actual)
		if encodeErr != nil {
			return encodeErr
		}
		_, err = tx.ExecContext(ctx, insertAssertionSQL,
			assertion.ID, attempt.SystemID, attempt.ID, assertion.Key, assertion.Type, assertion.Status,
			expected, actual, assertion.Message, assertion.DurationMS, attempt.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (repository *MySQLRepository) UpdateRun(ctx context.Context, run Run) error {
	output, err := encodeJSON(run.OutputVariables)
	if err != nil {
		return err
	}
	summary, err := encodeRunSummary(run)
	if err != nil {
		return err
	}
	result, err := repository.db.ExecContext(ctx, `UPDATE scenario_runs SET status = ?, output_variables = ?, summary = ?, finished_at = ? WHERE system_id = ? AND id = ?`,
		run.Status, output, summary, run.FinishedAt, run.SystemID, run.ID)
	if err != nil {
		return err
	}
	return requireUpdatedRun(result)
}

func (repository *MySQLRepository) GetRun(ctx context.Context, systemID, runID string) (Run, error) {
	run, err := scanRun(repository.db.QueryRowContext(ctx, selectRunSQL+" WHERE system_id = ? AND id = ?", systemID, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, err
	}
	run.Attempts, err = repository.listAttempts(ctx, systemID, runID)
	return run, err
}

func (repository *MySQLRepository) ListRuns(ctx context.Context, systemID string) ([]Run, error) {
	rows, err := repository.db.QueryContext(ctx, selectRunSQL+" WHERE system_id = ? ORDER BY created_at DESC, id DESC", systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range runs {
		runs[index].Attempts, err = repository.listAttempts(ctx, systemID, runs[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return runs, nil
}

func (repository *MySQLRepository) listAttempts(ctx context.Context, systemID, runID string) ([]StepAttempt, error) {
	rows, err := repository.db.QueryContext(ctx, selectAttemptSQL+" WHERE system_id = ? AND scenario_run_id = ? ORDER BY position, attempt_no", systemID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attempts []StepAttempt
	for rows.Next() {
		attempt, scanErr := scanAttempt(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attempt.Assertions, scanErr = repository.listAssertions(ctx, systemID, attempt.ID)
		if scanErr != nil {
			return nil, scanErr
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func (repository *MySQLRepository) listAssertions(ctx context.Context, systemID, attemptID string) ([]AssertionResult, error) {
	rows, err := repository.db.QueryContext(ctx, selectAssertionSQL+" WHERE system_id = ? AND step_run_id = ? ORDER BY created_at, id", systemID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var assertions []AssertionResult
	for rows.Next() {
		var assertion AssertionResult
		var expected, actual []byte
		var message sql.NullString
		var duration sql.NullInt64
		if err := rows.Scan(&assertion.ID, &assertion.Key, &assertion.Type, &assertion.Status, &expected, &actual, &message, &duration); err != nil {
			return nil, err
		}
		_ = decodeJSON(expected, &assertion.Expected)
		_ = decodeJSON(actual, &assertion.Actual)
		assertion.Message = message.String
		assertion.DurationMS = int(duration.Int64)
		assertions = append(assertions, assertion)
	}
	return assertions, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanRun(row scanner) (Run, error) {
	var run Run
	var input, output, summary []byte
	var started, finished sql.NullTime
	err := row.Scan(&run.ID, &run.SystemID, &run.ScenarioID, &run.ScenarioVersionID, &run.EnvironmentID, &run.Status,
		&run.TriggerType, &input, &output, &summary, &run.RequestedBy, &started, &finished, &run.CreatedAt)
	if err != nil {
		return Run{}, err
	}
	_ = decodeJSON(input, &run.InputVariables)
	_ = decodeJSON(output, &run.OutputVariables)
	var stored struct {
		Outcome string     `json:"outcome"`
		Summary RunSummary `json:"summary"`
	}
	if err := decodeJSON(summary, &stored); err != nil {
		return Run{}, err
	}
	run.Outcome, run.Summary = stored.Outcome, stored.Summary
	if started.Valid {
		run.StartedAt = started.Time
	}
	if finished.Valid {
		run.FinishedAt = finished.Time
	}
	return run, nil
}

func scanAttempt(row scanner) (StepAttempt, error) {
	var attempt StepAttempt
	var request, response, extracted []byte
	var errorMessage sql.NullString
	var duration sql.NullInt64
	var started, finished sql.NullTime
	err := row.Scan(&attempt.ID, &attempt.SystemID, &attempt.RunID, &attempt.StepID, &attempt.AttemptNo, &attempt.Position,
		&attempt.Status, &request, &response, &extracted, &errorMessage, &duration, &started, &finished, &attempt.CreatedAt)
	if err != nil {
		return StepAttempt{}, err
	}
	_ = decodeJSON(request, &attempt.RequestSnapshot)
	_ = decodeJSON(response, &attempt.ResponseSnapshot)
	_ = decodeJSON(extracted, &attempt.ExtractedVariables)
	attempt.ErrorMessage, attempt.DurationMS = errorMessage.String, int(duration.Int64)
	if started.Valid {
		attempt.StartedAt = started.Time
	}
	if finished.Valid {
		attempt.FinishedAt = finished.Time
	}
	return attempt, nil
}

func encodeRunSummary(run Run) ([]byte, error) {
	return json.Marshal(struct {
		Outcome string     `json:"outcome"`
		Summary RunSummary `json:"summary"`
	}{run.Outcome, run.Summary})
}

func encodeJSON(value any) ([]byte, error) { return json.Marshal(value) }

func decodeJSON(data []byte, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func requireUpdatedRun(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrRunNotFound
	}
	return nil
}

var _ Repository = (*MySQLRepository)(nil)
