package execution

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLLeaseNextUsesScopedSkipLockedTransaction(t *testing.T) {
	repository, mock := newExecutionSQLMock(t)
	now := fixedNow()
	expires := now.Add(time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectLeaseCandidateSQL)).
		WithArgs(testSystemID, RunStatusQueued).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("run-1"))
	mock.ExpectQuery(regexp.QuoteMeta(selectRunSQL+" WHERE system_id = ? AND id = ? FOR UPDATE")).
		WithArgs(testSystemID, "run-1").
		WillReturnRows(asyncRunRow("run-1", testSystemID, RunStatusQueued, now))
	mock.ExpectExec(regexp.QuoteMeta(updateLeaseSQL)).
		WithArgs(RunStatusRunning, sqlmock.AnyArg(), testSystemID, "run-1", RunStatusQueued).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	run, found, err := repository.LeaseNext(context.Background(), testSystemID, "worker-a", now, time.Minute)
	if err != nil || !found || run.Summary.LeaseOwner != "worker-a" || !run.Summary.LeaseExpiresAt.Equal(expires) {
		t.Fatalf("LeaseNext() = %#v, %v, %v", run, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLHeartbeatIsScopedAndLeaseOwned(t *testing.T) {
	repository, mock := newExecutionSQLMock(t)
	now := fixedNow()
	mock.ExpectExec(regexp.QuoteMeta(heartbeatLeaseSQL)).
		WithArgs(sqlmock.AnyArg(), testSystemID, "run-1", RunStatusRunning, "worker-a").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repository.Heartbeat(context.Background(), testSystemID, "run-1", "worker-a", now.Add(time.Minute)); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func asyncRunRow(id, systemID, status string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "system_id", "scenario_id", "scenario_version_id", "environment_id", "status", "trigger_type", "input_variables", "output_variables", "summary", "requested_by", "started_at", "finished_at", "created_at"}).
		AddRow(id, systemID, testScenarioID, testVersionID, testEnvID, status, "api", []byte(`{}`), nil, []byte(`{"outcome":"","summary":{"totalSteps":1,"executedSteps":0}}`), testUserID, nil, nil, now)
}
