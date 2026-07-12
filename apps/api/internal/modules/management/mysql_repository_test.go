package management

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var overviewColumns = []string{"system_id", "system_key", "system_name", "my_role", "api_count", "p0_pending_count", "scenario_count", "runs_passed", "runs_failed", "runs_running", "member_count", "environment_count", "code_source_count", "last_run_at"}

func TestMySQLManagementOverviewUsesMembershipScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewMySQLRepository(db)
	lastRunAt := time.Date(2026, time.July, 12, 3, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(platformRoleSQL)).WithArgs(managementUser).WillReturnRows(sqlmock.NewRows([]string{"platform_role"}).AddRow("member"))
	mock.ExpectQuery(regexp.QuoteMeta(listMemberOverviewSQL)).WithArgs(managementUser).WillReturnRows(
		sqlmock.NewRows(overviewColumns).AddRow(managementSystemA, "oms", "OMS", "owner", 12, 2, 4, 7, 1, 0, 5, 3, 2, lastRunAt),
	)

	overview, err := repository.Overview(context.Background(), managementUser)
	if err != nil || overview.Scope != ScopeMember || overview.SystemCount != 1 || overview.APICount != 12 {
		t.Fatalf("overview=%#v err=%v", overview, err)
	}
	item := overview.Systems[0]
	if item.MemberCount != 5 || item.EnvironmentCount != 3 || item.CodeSourceCount != 2 || item.LastRunAt == nil || !item.LastRunAt.Equal(lastRunAt) {
		t.Fatalf("system metrics=%#v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLManagementOverviewUsesNullForSystemWithoutRuns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewMySQLRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta(platformRoleSQL)).WithArgs(managementAdmin).WillReturnRows(sqlmock.NewRows([]string{"platform_role"}).AddRow("admin"))
	mock.ExpectQuery(regexp.QuoteMeta(getAdminOverviewSQL)).WithArgs(managementSystemA).WillReturnRows(
		sqlmock.NewRows(overviewColumns).AddRow(managementSystemA, "oms", "OMS", "platform_admin", 0, 0, 0, 0, 0, 0, 0, 0, 0, nil),
	)

	item, found, err := repository.SystemOverview(context.Background(), managementAdmin, managementSystemA)
	if err != nil || !found || item.LastRunAt != nil {
		t.Fatalf("item=%#v found=%v err=%v", item, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLManagementDetailNeverCrossesMemberScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewMySQLRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta(platformRoleSQL)).WithArgs(managementUser).WillReturnRows(sqlmock.NewRows([]string{"platform_role"}).AddRow("member"))
	mock.ExpectQuery(regexp.QuoteMeta(getMemberOverviewSQL)).WithArgs(managementUser, managementSystemB).WillReturnError(sql.ErrNoRows)

	_, found, err := repository.SystemOverview(context.Background(), managementUser, managementSystemB)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
