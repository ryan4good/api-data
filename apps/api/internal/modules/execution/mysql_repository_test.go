package execution

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLRepositorySaveAttemptUsesScopedTransactionForAssertions(t *testing.T) {
	repository, mock := newExecutionSQLMock(t)
	now := fixedNow()
	attempt := StepAttempt{
		ID: "attempt-1", SystemID: testSystemID, RunID: "run-1", StepID: "step-1", AttemptNo: 1, Position: 1,
		Status: StepStatusFailed, RequestSnapshot: map[string]any{"headers": map[string]any{"Authorization": "[REDACTED]"}},
		ResponseSnapshot: map[string]any{"status": 500}, ErrorMessage: "failed", DurationMS: 10,
		Assertions: []AssertionResult{{ID: "assert-1", Key: "status", Type: "status", Status: AssertionFailed, Expected: 200, Actual: 500}},
		StartedAt:  now, FinishedAt: now, CreatedAt: now,
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertStepAttemptSQL)).
		WithArgs(attempt.ID, testSystemID, attempt.RunID, attempt.StepID, 1, 1, StepStatusFailed, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "failed", 10, now, now, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertAssertionSQL)).
		WithArgs("assert-1", testSystemID, attempt.ID, "status", "status", AssertionFailed, sqlmock.AnyArg(), sqlmock.AnyArg(), "", 0, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repository.SaveAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("SaveAttempt() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetRunScopesRunAttemptsAndAssertions(t *testing.T) {
	repository, mock := newExecutionSQLMock(t)
	now := fixedNow()
	runColumns := []string{"id", "system_id", "scenario_id", "scenario_version_id", "environment_id", "status", "trigger_type", "input_variables", "output_variables", "summary", "requested_by", "started_at", "finished_at", "created_at"}
	mock.ExpectQuery(regexp.QuoteMeta(selectRunSQL+" WHERE system_id = ? AND id = ?")).
		WithArgs(testSystemID, "run-1").
		WillReturnRows(sqlmock.NewRows(runColumns).AddRow("run-1", testSystemID, testScenarioID, testVersionID, testEnvID, RunStatusPassed, "manual", []byte(`{}`), []byte(`{}`), []byte(`{"outcome":"succeeded","summary":{"totalSteps":1,"executedSteps":1}}`), testUserID, now, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(selectAttemptSQL+" WHERE system_id = ? AND scenario_run_id = ? ORDER BY position, attempt_no")).
		WithArgs(testSystemID, "run-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "system_id", "scenario_run_id", "scenario_step_id", "attempt_no", "position", "status", "request_snapshot", "response_snapshot", "extracted_variables", "error_message", "duration_ms", "started_at", "finished_at", "created_at"}).
			AddRow("attempt-1", testSystemID, "run-1", "step-1", 1, 1, StepStatusPassed, []byte(`{}`), []byte(`{}`), []byte(`{}`), nil, 5, now, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(selectAssertionSQL+" WHERE system_id = ? AND step_run_id = ? ORDER BY created_at, id")).
		WithArgs(testSystemID, "attempt-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assertion_key", "assertion_type", "status", "expected_value", "actual_value", "message", "duration_ms"}))

	run, err := repository.GetRun(context.Background(), testSystemID, "run-1")
	if err != nil || len(run.Attempts) != 1 || run.Outcome != OutcomeSucceeded {
		t.Fatalf("GetRun() = %#v, %v", run, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newExecutionSQLMock(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), mock
}
