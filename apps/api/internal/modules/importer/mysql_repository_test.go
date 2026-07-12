package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLRepositoryGetAndListUseExplicitSystemScope(t *testing.T) {
	repository, mock := newImporterMock(t)
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	payload := storedConversionJSON(t, StoredConversion{})
	columns := []string{"id", "system_id", "format", "file_name", "content_hash", "status", "raw_document", "conversion_report", "error_message", "imported_by", "created_at", "updated_at"}
	mock.ExpectQuery(regexp.QuoteMeta(selectImportBase+" WHERE system_id = ? AND id = ?")).
		WithArgs(testSystemID, "import-1").
		WillReturnRows(sqlmock.NewRows(columns).AddRow("import-1", testSystemID, DBFormatPostman21, "a.json", "hash", StatusReady, []byte(`{}`), payload, nil, testUserID, now, now))
	if _, err := repository.Get(context.Background(), testSystemID, "import-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(selectImportBase + " WHERE system_id = ? ORDER BY created_at DESC, id DESC")).
		WithArgs(testSystemID).
		WillReturnRows(sqlmock.NewRows(columns))
	if _, err := repository.List(context.Background(), testSystemID); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryApplyLocksScopedRowAndCommits(t *testing.T) {
	repository, mock := newImporterMock(t)
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	conversion := StoredConversion{Result: Result{Report: ImportReport{Counts: Counts{Scripts: 1}}}, Review: Review{ScriptsConfirmed: true, ReviewedBy: testUserID, ReviewedAt: &now}}
	payload := storedConversionJSON(t, conversion)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectImportBase+" WHERE system_id = ? AND id = ? FOR UPDATE")).
		WithArgs(testSystemID, "import-1").
		WillReturnRows(importRow("import-1", testSystemID, StatusReady, payload, now))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scenario_imports SET status = ?, updated_at = ? WHERE system_id = ? AND id = ? AND status = ?")).
		WithArgs(StatusApplied, sqlmock.AnyArg(), testSystemID, "import-1", StatusReady).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repository.Apply(context.Background(), testSystemID, "import-1", now)
	if err != nil || result.Status != StatusApplied {
		t.Fatalf("Apply() = %#v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryApplyRollsBackWhenScriptsAreUnreviewed(t *testing.T) {
	repository, mock := newImporterMock(t)
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	payload := storedConversionJSON(t, StoredConversion{Result: Result{Report: ImportReport{Counts: Counts{Scripts: 1}}}})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectImportBase+" WHERE system_id = ? AND id = ? FOR UPDATE")).
		WithArgs(testSystemID, "import-1").
		WillReturnRows(importRow("import-1", testSystemID, StatusReady, payload, now))
	mock.ExpectRollback()

	_, err := repository.Apply(context.Background(), testSystemID, "import-1", now)
	if !errors.Is(err, ErrScriptsUnreviewed) {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryConfirmScriptsLocksAndUpdatesScopedRow(t *testing.T) {
	repository, mock := newImporterMock(t)
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	payload := storedConversionJSON(t, StoredConversion{Result: Result{Report: ImportReport{Counts: Counts{Scripts: 1}}}})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectImportBase+" WHERE system_id = ? AND id = ? FOR UPDATE")).
		WithArgs(testSystemID, "import-1").
		WillReturnRows(importRow("import-1", testSystemID, StatusReady, payload, now))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scenario_imports SET conversion_report = ?, updated_at = ? WHERE system_id = ? AND id = ? AND status = ?")).
		WithArgs(sqlmock.AnyArg(), now, testSystemID, "import-1", StatusReady).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	record, err := repository.ConfirmScripts(context.Background(), testSystemID, "import-1", testUserID, now)
	if err != nil {
		t.Fatalf("ConfirmScripts() error = %v", err)
	}
	if !record.Conversion.Review.ScriptsConfirmed || record.Conversion.Review.ReviewedBy != testUserID {
		t.Fatalf("review = %#v", record.Conversion.Review)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newImporterMock(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), mock
}

func storedConversionJSON(t *testing.T, value StoredConversion) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func importRow(id, systemID, status string, conversion []byte, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "system_id", "format", "file_name", "content_hash", "status", "raw_document", "conversion_report", "error_message", "imported_by", "created_at", "updated_at"}).
		AddRow(id, systemID, DBFormatPostman21, "a.json", "hash", status, []byte(`{}`), conversion, sql.NullString{}, testUserID, now, now)
}
