package system

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const listAuthorizedSystemsSQL = `SELECT bs.id, bs.system_key, bs.name, bs.description, bs.status,
       sm.role, bs.created_at, bs.updated_at
FROM business_systems AS bs
JOIN system_members AS sm
  ON sm.system_id = bs.id
 AND sm.user_id = ?
WHERE bs.status = 'active'
ORDER BY bs.name, bs.id`

const getAuthorizedSystemSQL = `SELECT bs.id, bs.system_key, bs.name, bs.description, bs.status,
       sm.role, bs.created_at, bs.updated_at
FROM business_systems AS bs
JOIN system_members AS sm
  ON sm.system_id = bs.id
 AND sm.user_id = ?
WHERE bs.id = ?
  AND bs.status = 'active'`

const roleForUserSQL = `SELECT role
FROM system_members
WHERE system_id = ?
  AND user_id = ?`

const listMembersSQL = `SELECT sm.system_id, u.id, u.display_name, u.email, sm.role, u.status
FROM system_members AS sm
JOIN users AS u
  ON u.id = sm.user_id
WHERE sm.system_id = ?
ORDER BY u.id`

const getMemberSQL = `SELECT sm.system_id, u.id, u.display_name, u.email, sm.role, u.status
FROM system_members AS sm
JOIN users AS u
  ON u.id = sm.user_id
WHERE sm.system_id = ?
  AND sm.user_id = ?`

const upsertMemberSQL = `INSERT INTO system_members (id, system_id, user_id, role)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE role = VALUES(role), updated_at = CURRENT_TIMESTAMP(3)`

type MySQLRepository struct {
	db *sql.DB
}

type MySQLPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func OpenMySQLRepository(ctx context.Context, dsn string, pool MySQLPoolConfig) (*MySQLRepository, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return NewMySQLRepository(db), nil
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Close() error {
	return r.db.Close()
}

func (r *MySQLRepository) ListAuthorized(ctx context.Context, userID string) ([]AuthorizedSystem, error) {
	rows, err := r.db.QueryContext(ctx, listAuthorizedSystemsSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("list authorized systems: %w", err)
	}
	defer rows.Close()

	items := make([]AuthorizedSystem, 0)
	for rows.Next() {
		item, err := scanAuthorizedSystem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan authorized system: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authorized systems: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) GetAuthorized(ctx context.Context, systemID, userID string) (AuthorizedSystem, bool, error) {
	item, err := scanAuthorizedSystem(r.db.QueryRowContext(ctx, getAuthorizedSystemSQL, userID, systemID))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthorizedSystem{}, false, nil
	}
	if err != nil {
		return AuthorizedSystem{}, false, fmt.Errorf("get authorized system: %w", err)
	}
	return item, true, nil
}

func (r *MySQLRepository) RoleForUser(ctx context.Context, systemID, userID string) (string, bool, error) {
	var role string
	err := r.db.QueryRowContext(ctx, roleForUserSQL, systemID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get member role: %w", err)
	}
	return role, true, nil
}

func (r *MySQLRepository) ListMembers(ctx context.Context, systemID string) ([]Member, error) {
	rows, err := r.db.QueryContext(ctx, listMembersSQL, systemID)
	if err != nil {
		return nil, fmt.Errorf("list system members: %w", err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.SystemID, &member.UserID, &member.DisplayName, &member.Email, &member.Role, &member.Status); err != nil {
			return nil, fmt.Errorf("scan system member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system members: %w", err)
	}
	return members, nil
}

func (r *MySQLRepository) UpsertMember(ctx context.Context, member Member) (Member, error) {
	id, err := newUUID()
	if err != nil {
		return Member{}, fmt.Errorf("generate member id: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, upsertMemberSQL, id, member.SystemID, member.UserID, string(member.Role)); err != nil {
		return Member{}, fmt.Errorf("upsert system member: %w", err)
	}
	stored, err := scanMember(r.db.QueryRowContext(ctx, getMemberSQL, member.SystemID, member.UserID))
	if err != nil {
		return Member{}, fmt.Errorf("read upserted system member: %w", err)
	}
	return stored, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAuthorizedSystem(row rowScanner) (AuthorizedSystem, error) {
	var item AuthorizedSystem
	var description sql.NullString
	if err := row.Scan(
		&item.System.ID,
		&item.System.Code,
		&item.System.Name,
		&description,
		&item.System.Status,
		&item.Role,
		&item.System.CreatedAt,
		&item.System.UpdatedAt,
	); err != nil {
		return AuthorizedSystem{}, err
	}
	item.System.Description = description.String
	return item, nil
}

func scanMember(row rowScanner) (Member, error) {
	var member Member
	if err := row.Scan(&member.SystemID, &member.UserID, &member.DisplayName, &member.Email, &member.Role, &member.Status); err != nil {
		return Member{}, err
	}
	return member, nil
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

var _ Repository = (*MySQLRepository)(nil)
