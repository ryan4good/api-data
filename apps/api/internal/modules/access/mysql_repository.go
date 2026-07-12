package access

import (
	"context"
	"database/sql"
	"errors"
)

const userColumns = `id, email, display_name, password_hash, platform_role, status`
const findUserByEmailSQL = `SELECT ` + userColumns + ` FROM users WHERE email = ? LIMIT 1`
const findUserByIDSQL = `SELECT ` + userColumns + ` FROM users WHERE id = ? LIMIT 1`

type MySQLUserRepository struct{ db *sql.DB }

func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository { return &MySQLUserRepository{db: db} }

func (repository *MySQLUserRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(repository.db.QueryRowContext(ctx, findUserByEmailSQL, normalizeEmail(email)))
}

func (repository *MySQLUserRepository) FindByID(ctx context.Context, id string) (User, error) {
	return scanUser(repository.db.QueryRowContext(ctx, findUserByIDSQL, id))
}

type rowScanner interface{ Scan(...any) error }

func scanUser(row rowScanner) (User, error) {
	var user User
	var passwordHash sql.NullString
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &passwordHash, &user.PlatformRole, &user.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	user.PasswordHash = passwordHash.String
	return user, nil
}
