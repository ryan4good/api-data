package discovery

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newDiscoverySQLMock(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), mock
}

func TestMySQLCreateWithCandidatesIsTransactional(t *testing.T) {
	repo, mock := newDiscoverySQLMock(t)
	discovery := Discovery{ID: testDiscovery, SystemID: testSystemA, Type: TypeCode, Name: "Orders", InputDocument: []byte(`{}`), Status: StatusReady, RequestedBy: testUser, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	candidate := Candidate{ID: testCandidate, SystemID: testSystemA, DiscoveryID: testDiscovery, Key: "orders-create", Name: "Create order", Confidence: .9, Evidence: []byte(`{}`), Bundle: []byte(`{}`), ReviewStatus: ReviewPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertDiscoverySQL)).WithArgs(sqlmock.AnyArg(), testSystemA, sqlmock.AnyArg(), "code", "Orders", sqlmock.AnyArg(), sqlmock.AnyArg(), "ready", sqlmock.AnyArg(), testUser, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertCandidateSQL)).WithArgs(sqlmock.AnyArg(), testSystemA, testDiscovery, "orders-create", "Create order", sqlmock.AnyArg(), .9, sqlmock.AnyArg(), sqlmock.AnyArg(), "pending", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.CreateWithCandidates(context.Background(), discovery, []Candidate{candidate}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLQueriesAndReviewsAlwaysUseSystemScope(t *testing.T) {
	repo, mock := newDiscoverySQLMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(listDiscoveriesSQL)).WithArgs(testSystemA).WillReturnRows(sqlmock.NewRows(discoveryColumns))
	if _, err := repo.ListDiscoveries(context.Background(), testSystemA); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(listCandidatesSQL)).WithArgs(testSystemA, testDiscovery).WillReturnRows(sqlmock.NewRows(candidateColumns))
	if _, err := repo.ListCandidates(context.Background(), testSystemA, testDiscovery); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta(reviewCandidateSQL)).WithArgs("accepted", "ok", testUser, sqlmock.AnyArg(), sqlmock.AnyArg(), testSystemA, testCandidate, "pending").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(getCandidateSQL)).WithArgs(testSystemA, testCandidate).WillReturnError(sql.ErrNoRows)
	_, err := repo.ReviewCandidate(context.Background(), testSystemA, testCandidate, ReviewAccepted, "ok", testUser, time.Now())
	if err != ErrCandidateNotFound {
		t.Fatalf("err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLCreateRollsBackCandidateFailure(t *testing.T) {
	repo, mock := newDiscoverySQLMock(t)
	discovery := Discovery{ID: testDiscovery, SystemID: testSystemA, Type: TypeCode, Name: "Orders", InputDocument: []byte(`{}`), Status: StatusReady, RequestedBy: testUser, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	candidate := Candidate{ID: testCandidate, SystemID: testSystemA, DiscoveryID: testDiscovery, Key: "orders", Evidence: []byte(`{}`), Bundle: []byte(`{}`), ReviewStatus: ReviewPending}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertDiscoverySQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertCandidateSQL)).WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()
	if err := repo.CreateWithCandidates(context.Background(), discovery, []Candidate{candidate}); err == nil {
		t.Fatal("error=nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
