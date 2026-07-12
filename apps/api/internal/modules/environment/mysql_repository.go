package environment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const upsertEnvironmentSQL = `INSERT INTO environments (id, system_id, environment_key, name, variables, status, created_by) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=VALUES(name), variables=VALUES(variables), status=VALUES(status), updated_at=CURRENT_TIMESTAMP(3)`
const getEnvironmentSQL = `SELECT id, system_id, environment_key, name, variables, status, created_by, created_at, updated_at FROM environments WHERE system_id = ? AND id = ?`
const listEnvironmentsSQL = `SELECT id, system_id, environment_key, name, variables, status, created_by, created_at, updated_at FROM environments WHERE system_id = ? ORDER BY environment_key, id`
const upsertSecretReferenceSQL = `INSERT INTO environment_secrets (id, system_id, environment_id, variable_key, secret_ref, created_by) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE secret_ref=VALUES(secret_ref), updated_at=CURRENT_TIMESTAMP(3)`
const listSecretReferencesSQL = `SELECT id, system_id, environment_id, variable_key, secret_ref, created_by, created_at, updated_at FROM environment_secrets WHERE system_id = ? AND environment_id = ? ORDER BY variable_key, id`

var environmentColumns = []string{"id", "system_id", "environment_key", "name", "variables", "status", "created_by", "created_at", "updated_at"}
var secretReferenceColumns = []string{"id", "system_id", "environment_id", "variable_key", "secret_ref", "created_by", "created_at", "updated_at"}

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }
func (r *MySQLRepository) UpsertEnvironment(ctx context.Context, e Environment) error {
	raw, err := json.Marshal(e.Variables)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, upsertEnvironmentSQL, e.ID, e.SystemID, e.Key, e.Name, raw, e.Status, e.CreatedBy)
	return err
}
func (r *MySQLRepository) GetEnvironment(ctx context.Context, systemID, id string) (Environment, bool, error) {
	e, err := scanEnvironment(r.db.QueryRowContext(ctx, getEnvironmentSQL, systemID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Environment{}, false, nil
	}
	return e, err == nil, err
}
func (r *MySQLRepository) ListEnvironments(ctx context.Context, systemID string) ([]Environment, error) {
	rows, err := r.db.QueryContext(ctx, listEnvironmentsSQL, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Environment{}
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r *MySQLRepository) UpsertSecretReference(ctx context.Context, ref SecretReference) error {
	if ref.SecretRef == "" {
		return errors.New("secret reference required")
	}
	_, err := r.db.ExecContext(ctx, upsertSecretReferenceSQL, ref.ID, ref.SystemID, ref.EnvironmentID, ref.VariableKey, ref.SecretRef, ref.CreatedBy)
	if err != nil {
		return fmt.Errorf("upsert secret reference: %w", err)
	}
	return nil
}
func (r *MySQLRepository) ListSecretReferences(ctx context.Context, systemID, environmentID string) ([]SecretReference, error) {
	rows, err := r.db.QueryContext(ctx, listSecretReferencesSQL, systemID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SecretReference{}
	for rows.Next() {
		var ref SecretReference
		if err := rows.Scan(&ref.ID, &ref.SystemID, &ref.EnvironmentID, &ref.VariableKey, &ref.SecretRef, &ref.CreatedBy, &ref.CreatedAt, &ref.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

type row interface{ Scan(...any) error }

func scanEnvironment(r row) (Environment, error) {
	var e Environment
	var raw []byte
	if err := r.Scan(&e.ID, &e.SystemID, &e.Key, &e.Name, &raw, &e.Status, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return e, err
	}
	e.Variables = map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &e.Variables); err != nil {
			return e, err
		}
	}
	return e, nil
}

var _ Repository = (*MySQLRepository)(nil)
