package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const platformRoleSQL = `SELECT platform_role FROM users WHERE id = ? AND status = 'active'`

const metricSelectSQL = `SELECT bs.id, bs.system_key, bs.name, %s,
 (SELECT COUNT(*) FROM api_operations ao WHERE ao.system_id = bs.id AND ao.lifecycle_status = 'active') AS api_count,
 (SELECT COUNT(*) FROM scenario_candidates sc WHERE sc.system_id = bs.id AND sc.review_status = 'pending' AND JSON_UNQUOTE(JSON_EXTRACT(sc.candidate_bundle, '$.priority')) = 'P0') AS p0_pending_count,
 (SELECT COUNT(*) FROM scenarios s WHERE s.system_id = bs.id AND s.status <> 'archived') AS scenario_count,
 (SELECT COUNT(*) FROM scenario_runs sr WHERE sr.system_id = bs.id AND sr.status = 'passed' AND sr.created_at >= UTC_TIMESTAMP(3) - INTERVAL 1 DAY) AS runs_passed,
 (SELECT COUNT(*) FROM scenario_runs sr WHERE sr.system_id = bs.id AND sr.status IN ('failed','timed_out') AND sr.created_at >= UTC_TIMESTAMP(3) - INTERVAL 1 DAY) AS runs_failed,
 (SELECT COUNT(*) FROM scenario_runs sr WHERE sr.system_id = bs.id AND sr.status IN ('queued','running') AND sr.created_at >= UTC_TIMESTAMP(3) - INTERVAL 1 DAY) AS runs_running,
 (SELECT COUNT(*) FROM system_members member WHERE member.system_id = bs.id) AS member_count,
 (SELECT COUNT(*) FROM environments environment WHERE environment.system_id = bs.id AND environment.status = 'active') AS environment_count,
 (SELECT COUNT(*) FROM code_sources source WHERE source.system_id = bs.id AND source.status = 'active') AS code_source_count,
 (SELECT MAX(sr.created_at) FROM scenario_runs sr WHERE sr.system_id = bs.id) AS last_run_at
FROM business_systems bs %s WHERE bs.status = 'active' %s`

var (
	listMemberOverviewSQL = fmt.Sprintf(metricSelectSQL, "sm.role", "JOIN system_members sm ON sm.system_id = bs.id AND sm.user_id = ?", "ORDER BY bs.name, bs.id")
	getMemberOverviewSQL  = fmt.Sprintf(metricSelectSQL, "sm.role", "JOIN system_members sm ON sm.system_id = bs.id AND sm.user_id = ?", "AND bs.id = ?")
	listAdminOverviewSQL  = fmt.Sprintf(metricSelectSQL, "'platform_admin'", "", "ORDER BY bs.name, bs.id")
	getAdminOverviewSQL   = fmt.Sprintf(metricSelectSQL, "'platform_admin'", "", "AND bs.id = ?")
)

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (repository *MySQLRepository) Overview(ctx context.Context, userID string) (Overview, error) {
	admin, err := repository.isAdmin(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	query, args, scope := listMemberOverviewSQL, []any{userID}, ScopeMember
	if admin {
		query, args, scope = listAdminOverviewSQL, nil, ScopePlatform
	}
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Overview{}, fmt.Errorf("list management overview: %w", err)
	}
	defer rows.Close()
	items := make([]SystemOverview, 0)
	for rows.Next() {
		item, err := scanSystemOverview(rows)
		if err != nil {
			return Overview{}, fmt.Errorf("scan management overview: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Overview{}, fmt.Errorf("iterate management overview: %w", err)
	}
	return buildOverview(scope, items), nil
}

func (repository *MySQLRepository) SystemOverview(ctx context.Context, userID, systemID string) (SystemOverview, bool, error) {
	admin, err := repository.isAdmin(ctx, userID)
	if err != nil {
		return SystemOverview{}, false, err
	}
	query, args := getMemberOverviewSQL, []any{userID, systemID}
	if admin {
		query, args = getAdminOverviewSQL, []any{systemID}
	}
	item, err := scanSystemOverview(repository.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return SystemOverview{}, false, nil
	}
	if err != nil {
		return SystemOverview{}, false, fmt.Errorf("get management system overview: %w", err)
	}
	return item, true, nil
}

func (repository *MySQLRepository) isAdmin(ctx context.Context, userID string) (bool, error) {
	var role string
	if err := repository.db.QueryRowContext(ctx, platformRoleSQL, userID).Scan(&role); err != nil {
		return false, fmt.Errorf("get platform role: %w", err)
	}
	return role == "admin", nil
}

type overviewScanner interface{ Scan(...any) error }

func scanSystemOverview(row overviewScanner) (SystemOverview, error) {
	var item SystemOverview
	err := row.Scan(&item.SystemID, &item.SystemKey, &item.SystemName, &item.MyRole, &item.APICount, &item.P0PendingCount, &item.ScenarioCount, &item.Runs24h.Passed, &item.Runs24h.Failed, &item.Runs24h.Running, &item.MemberCount, &item.EnvironmentCount, &item.CodeSourceCount, &item.LastRunAt)
	if err == nil {
		item.Risks = buildOverview(ScopeMember, []SystemOverview{item}).Systems[0].Risks
	}
	return item, err
}

var _ Repository = (*MySQLRepository)(nil)
