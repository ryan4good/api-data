package management

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var overviewColumns = []string{"system_id", "system_key", "system_name", "my_role", "api_count", "p0_pending_count", "scenario_count", "runs_passed", "runs_failed", "runs_running"}

func TestMySQLManagementOverviewUsesMembershipScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewMySQLRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta(platformRoleSQL)).WithArgs(managementUser).WillReturnRows(sqlmock.NewRows([]string{"platform_role"}).AddRow("member"))
	mock.ExpectQuery(regexp.QuoteMeta(listMemberOverviewSQL)).WithArgs(managementUser).WillReturnRows(
		sqlmock.NewRows(overviewColumns).AddRow(managementSystemA, "oms", "OMS", "owner", 12, 2, 4, 7, 1, 0),
	)

	overview, err := repository.Overview(context.Background(), managementUser)
	if err != nil || overview.Scope != ScopeMember || overview.SystemCount != 1 || overview.APICount != 12 {
		t.Fatalf("overview=%#v err=%v", overview, err)
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
