package codesource

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestMySQLRepositoryUsesSystemScopedQueriesAndJSONPaths(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	r := NewMySQLRepository(db)
	source := CodeSource{ID: testSource, SystemID: testSystem, Name: "repo", SourceType: TypeGit, RepositoryURL: "https://x", DefaultRef: "main", IncludePaths: []string{"src"}, ExcludePaths: []string{"vendor"}, CredentialRef: "vault://git/repo", Status: StatusActive, CreatedBy: testUser}
	mock.ExpectExec(regexp.QuoteMeta(insertSQL)).WithArgs(testSource, testSystem, "repo", TypeGit, "https://x", nil, "main", []byte(`["src"]`), []byte(`["vendor"]`), "vault://git/repo", StatusActive, testUser).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := r.Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(listSQL)).WithArgs(testSystem).WillReturnRows(sqlmock.NewRows(columns).AddRow(testSource, testSystem, "repo", TypeGit, "https://x", nil, "main", []byte(`["src"]`), []byte(`["vendor"]`), "vault://git/repo", StatusActive, testUser, now, now))
	items, err := r.List(context.Background(), testSystem)
	if err != nil || len(items) != 1 || items[0].IncludePaths[0] != "src" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(getSQL)).WithArgs(testSystem, testSource).WillReturnRows(sqlmock.NewRows(columns).AddRow(testSource, testSystem, "repo", TypeGit, "https://x", nil, "main", []byte(`["src"]`), []byte(`["vendor"]`), "vault://git/repo", StatusActive, testUser, now, now))
	if got, found, err := r.Get(context.Background(), testSystem, testSource); err != nil || !found || got.ID != testSource {
		t.Fatalf("get=%#v found=%v err=%v", got, found, err)
	}
	mock.ExpectExec(regexp.QuoteMeta(updateSQL)).WithArgs("repo", TypeGit, "https://x", nil, "main", []byte(`["src"]`), []byte(`["vendor"]`), "vault://git/repo", StatusActive, testSystem, testSource).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(getSQL)).WithArgs(testSystem, testSource).WillReturnRows(sqlmock.NewRows(columns))
	if err := r.Update(context.Background(), source); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUpdateDoesNotReportExistingUnchangedRowAsMissing(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	r := NewMySQLRepository(db)
	source := CodeSource{ID: testSource, SystemID: testSystem, Name: "repo", SourceType: TypeGit, RepositoryURL: "https://x", Status: StatusActive}
	mock.ExpectExec(regexp.QuoteMeta(updateSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(getSQL)).WithArgs(testSystem, testSource).WillReturnRows(sqlmock.NewRows(columns).AddRow(testSource, testSystem, "repo", TypeGit, "https://x", nil, nil, []byte(`[]`), []byte(`[]`), nil, StatusActive, testUser, time.Now(), time.Now()))
	if err := r.Update(context.Background(), source); err != nil {
		t.Fatalf("unchanged update err=%v", err)
	}
}

func TestMySQLRepositoryMapsDuplicateNameConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	r := NewMySQLRepository(db)
	source := CodeSource{ID: testSource, SystemID: testSystem, Name: "repo", SourceType: TypeGit, RepositoryURL: "https://x", Status: StatusActive, CreatedBy: testUser}
	mock.ExpectExec(regexp.QuoteMeta(insertSQL)).WithArgs(testSource, testSystem, "repo", TypeGit, "https://x", nil, nil, []byte(`[]`), []byte(`[]`), nil, StatusActive, testUser).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate"})
	if err := r.Create(context.Background(), source); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("err=%v", err)
	}
}
