package system

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"bizdevops/apps/api/internal/modules/access"
	"github.com/DATA-DOG/go-sqlmock"
)

const (
	mysqlTestSystemID = "11111111-1111-4111-8111-111111111111"
	mysqlTestUserID   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func newMockRepository(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), mock
}

func TestMySQLRepositoryListsOnlyAuthorizedSystems(t *testing.T) {
	repo, mock := newMockRepository(t)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listAuthorizedSystemsSQL)).
		WithArgs(mysqlTestUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "system_key", "name", "description", "status", "role", "created_at", "updated_at"}).
			AddRow(mysqlTestSystemID, "billing", "Billing", "payments", "active", "maintainer", now, now))

	items, err := repo.ListAuthorized(context.Background(), mysqlTestUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].System.ID != mysqlTestSystemID || items[0].Role != access.RoleMaintainer {
		t.Fatalf("unexpected items: %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetsSystemWithUserAndSystemScope(t *testing.T) {
	repo, mock := newMockRepository(t)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(getAuthorizedSystemSQL)).
		WithArgs(mysqlTestUserID, mysqlTestSystemID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "system_key", "name", "description", "status", "role", "created_at", "updated_at"}).
			AddRow(mysqlTestSystemID, "billing", "Billing", nil, "active", "owner", now, now))

	item, found, err := repo.GetAuthorized(context.Background(), mysqlTestSystemID, mysqlTestUserID)
	if err != nil || !found || item.System.ID != mysqlTestSystemID || item.System.Description != "" {
		t.Fatalf("item=%#v found=%v err=%v", item, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryTreatsMissingScopedRowsAsNotFound(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(getAuthorizedSystemSQL)).
		WithArgs(mysqlTestUserID, mysqlTestSystemID).
		WillReturnError(sql.ErrNoRows)

	_, found, err := repo.GetAuthorized(context.Background(), mysqlTestSystemID, mysqlTestUserID)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestMySQLRepositoryScopesRoleAndMemberQueries(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(roleForUserSQL)).
		WithArgs(mysqlTestSystemID, mysqlTestUserID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))

	role, found, err := repo.RoleForUser(context.Background(), mysqlTestSystemID, mysqlTestUserID)
	if err != nil || !found || role != "owner" {
		t.Fatalf("role=%q found=%v err=%v", role, found, err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(listMembersSQL)).
		WithArgs(mysqlTestSystemID).
		WillReturnRows(sqlmock.NewRows([]string{"system_id", "user_id", "display_name", "email", "role", "status"}).
			AddRow(mysqlTestSystemID, mysqlTestUserID, "Alice", "alice@example.test", "owner", "active"))
	members, err := repo.ListMembers(context.Background(), mysqlTestSystemID)
	if err != nil || len(members) != 1 || members[0].SystemID != mysqlTestSystemID || members[0].Status != MemberActive {
		t.Fatalf("members=%#v err=%v", members, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpsertsMemberWithBothScopeKeys(t *testing.T) {
	repo, mock := newMockRepository(t)
	member := Member{SystemID: mysqlTestSystemID, UserID: mysqlTestUserID, Role: access.RoleRunner, Status: MemberActive}
	mock.ExpectExec(regexp.QuoteMeta(upsertMemberSQL)).
		WithArgs(sqlmock.AnyArg(), mysqlTestSystemID, mysqlTestUserID, "runner").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(getMemberSQL)).
		WithArgs(mysqlTestSystemID, mysqlTestUserID).
		WillReturnRows(sqlmock.NewRows([]string{"system_id", "user_id", "display_name", "email", "role", "status"}).
			AddRow(mysqlTestSystemID, mysqlTestUserID, "E2E Member", "member@example.test", "runner", "active"))

	stored, err := repo.UpsertMember(context.Background(), member)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DisplayName != "E2E Member" || stored.Email != "member@example.test" || stored.Role != access.RoleRunner {
		t.Fatalf("stored member=%#v", stored)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
