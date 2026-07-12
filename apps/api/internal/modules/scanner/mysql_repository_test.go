package scanner

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newScannerMockRepository(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), mock
}

func TestMySQLRepositoryScopesScanQueriesBySystemID(t *testing.T) {
	repo, mock := newScannerMockRepository(t)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(getScanSQL)).
		WithArgs(testSystemA, "scan-a").
		WillReturnRows(sqlmock.NewRows(scanColumns).
			AddRow("scan-a", testSystemA, testSource, nil, nil, nil, nil, "queued", nil, nil, nil, testUser, nil, nil, now))

	scan, found, err := repo.GetScan(context.Background(), testSystemA, "scan-a")
	if err != nil || !found || scan.SystemID != testSystemA {
		t.Fatalf("scan=%#v found=%v err=%v", scan, found, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(listOperationsSQL)).
		WithArgs(testSystemA).
		WillReturnRows(sqlmock.NewRows(operationColumns))
	if _, err := repo.ListOperations(context.Background(), testSystemA); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryScopesListsAndTransitionsBySystemID(t *testing.T) {
	repo, mock := newScannerMockRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(listScansSQL)).
		WithArgs(testSystemA).
		WillReturnRows(sqlmock.NewRows(scanColumns))
	if _, err := repo.ListScans(context.Background(), testSystemA); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(transitionScanSQL)).
		WithArgs("running", &started, nil, nil, "", testSystemA, "scan-a", "queued").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.TransitionScan(context.Background(), testSystemA, "scan-a", StatusQueued, StatusRunning, ScanUpdate{StartedAt: &started}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReturnsNotFoundForScopedScan(t *testing.T) {
	repo, mock := newScannerMockRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(getScanSQL)).WithArgs(testSystemB, "scan-a").WillReturnError(sql.ErrNoRows)
	_, found, err := repo.GetScan(context.Background(), testSystemB, "scan-a")
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestMySQLRepositoryUpsertsOperationsInTransactionWithSystemScope(t *testing.T) {
	repo, mock := newScannerMockRepository(t)
	operation := APIOperation{
		ID: "op-a", SystemID: testSystemA, ScanRunID: "scan-a", OperationKey: "GET /orders",
		Method: "GET", Path: "/orders", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(upsertOperationSQL)).
		WithArgs(operation.ID, testSystemA, "scan-a", operation.OperationKey, operation.Method, operation.Path,
			sqlmock.AnyArg(), sqlmock.AnyArg(), operation.ContentHash).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.UpsertOperations(context.Background(), testSystemA, "scan-a", []APIOperation{operation}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRollsBackWholeOperationBatch(t *testing.T) {
	repo, mock := newScannerMockRepository(t)
	operation := APIOperation{ID: "op-a", SystemID: testSystemA, ScanRunID: "scan-a", OperationKey: "GET /orders", Method: "GET", Path: "/orders", ContentHash: "hash"}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(upsertOperationSQL)).WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()
	if err := repo.UpsertOperations(context.Background(), testSystemA, "scan-a", []APIOperation{operation}); err == nil {
		t.Fatal("UpsertOperations error=nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
