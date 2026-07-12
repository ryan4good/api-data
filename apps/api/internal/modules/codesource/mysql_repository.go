package codesource

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/go-sql-driver/mysql"
)

const insertSQL = `INSERT INTO code_sources (id, system_id, name, source_type, repository_url, local_path, default_ref, include_paths, exclude_paths, credential_ref, status, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const updateSQL = `UPDATE code_sources SET name = ?, source_type = ?, repository_url = ?, local_path = ?, default_ref = ?, include_paths = ?, exclude_paths = ?, credential_ref = ?, status = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE system_id = ? AND id = ?`
const getSQL = `SELECT id, system_id, name, source_type, repository_url, local_path, default_ref, include_paths, exclude_paths, credential_ref, status, created_by, created_at, updated_at FROM code_sources WHERE system_id = ? AND id = ?`
const listSQL = `SELECT id, system_id, name, source_type, repository_url, local_path, default_ref, include_paths, exclude_paths, credential_ref, status, created_by, created_at, updated_at FROM code_sources WHERE system_id = ? ORDER BY name, id`

var columns = []string{"id", "system_id", "name", "source_type", "repository_url", "local_path", "default_ref", "include_paths", "exclude_paths", "credential_ref", "status", "created_by", "created_at", "updated_at"}

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }
func encodePaths(paths []string) ([]byte, error) {
	if paths == nil {
		paths = []string{}
	}
	return json.Marshal(paths)
}
func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func duplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
func (r *MySQLRepository) Create(ctx context.Context, source CodeSource) error {
	include, err := encodePaths(source.IncludePaths)
	if err != nil {
		return err
	}
	exclude, err := encodePaths(source.ExcludePaths)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, insertSQL, source.ID, source.SystemID, source.Name, source.SourceType, optional(source.RepositoryURL), optional(source.LocalPath), optional(source.DefaultRef), include, exclude, optional(source.CredentialRef), source.Status, source.CreatedBy)
	if duplicate(err) {
		return ErrNameConflict
	}
	return err
}
func (r *MySQLRepository) Update(ctx context.Context, source CodeSource) error {
	include, err := encodePaths(source.IncludePaths)
	if err != nil {
		return err
	}
	exclude, err := encodePaths(source.ExcludePaths)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, updateSQL, source.Name, source.SourceType, optional(source.RepositoryURL), optional(source.LocalPath), optional(source.DefaultRef), include, exclude, optional(source.CredentialRef), source.Status, source.SystemID, source.ID)
	if duplicate(err) {
		return ErrNameConflict
	}
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		_, found, getErr := r.Get(ctx, source.SystemID, source.ID)
		if getErr != nil {
			return getErr
		}
		if !found {
			return ErrNotFound
		}
	}
	return nil
}
func (r *MySQLRepository) Get(ctx context.Context, systemID, id string) (CodeSource, bool, error) {
	source, err := scanSource(r.db.QueryRowContext(ctx, getSQL, systemID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CodeSource{}, false, nil
	}
	return source, err == nil, err
}
func (r *MySQLRepository) List(ctx context.Context, systemID string) ([]CodeSource, error) {
	rows, err := r.db.QueryContext(ctx, listSQL, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CodeSource{}
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	return out, rows.Err()
}

type row interface{ Scan(...any) error }

func scanSource(r row) (CodeSource, error) {
	var source CodeSource
	var repositoryURL, localPath, defaultRef, credentialRef sql.NullString
	var include, exclude []byte
	err := r.Scan(&source.ID, &source.SystemID, &source.Name, &source.SourceType, &repositoryURL, &localPath, &defaultRef, &include, &exclude, &credentialRef, &source.Status, &source.CreatedBy, &source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return source, err
	}
	source.RepositoryURL, source.LocalPath, source.DefaultRef, source.CredentialRef = repositoryURL.String, localPath.String, defaultRef.String, credentialRef.String
	source.IncludePaths, source.ExcludePaths = []string{}, []string{}
	if len(include) > 0 {
		if err := json.Unmarshal(include, &source.IncludePaths); err != nil {
			return source, err
		}
	}
	if len(exclude) > 0 {
		if err := json.Unmarshal(exclude, &source.ExcludePaths); err != nil {
			return source, err
		}
	}
	return source, nil
}

var _ Repository = (*MySQLRepository)(nil)
