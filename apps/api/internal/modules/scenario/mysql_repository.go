package scenario

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const lockCandidateSQL = `SELECT id, system_id, discovery_id, candidate_key, name, description, candidate_bundle, review_status, promoted_scenario_id
FROM scenario_candidates WHERE system_id = ? AND discovery_id = ? AND id = ? FOR UPDATE`
const insertScenarioSQL = `INSERT INTO scenarios (id, system_id, scenario_key, name, description, status, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)`
const insertVersionSQL = `INSERT INTO scenario_versions (id, system_id, scenario_id, version_no, source_type, bundle_document, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
const insertStepSQL = `INSERT INTO scenario_steps (id, system_id, scenario_version_id, step_key, name, position, step_type, api_operation_id, depends_on, request_config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)`
const updateCurrentVersionSQL = `UPDATE scenarios SET current_version_id = ?, updated_at = ? WHERE system_id = ? AND id = ?`
const markCandidatePromotedSQL = `UPDATE scenario_candidates SET promoted_scenario_id = ?, updated_at = ? WHERE system_id = ? AND discovery_id = ? AND id = ? AND review_status = ? AND promoted_scenario_id IS NULL`
const listScenariosSQL = `SELECT id, system_id, scenario_key, name, description, status, current_version_id, created_by, created_at, updated_at FROM scenarios WHERE system_id = ? ORDER BY scenario_key, id`
const getScenarioSQL = `SELECT id, system_id, scenario_key, name, description, status, current_version_id, created_by, created_at, updated_at FROM scenarios WHERE system_id = ? AND id = ?`
const lockScenarioSQL = `SELECT id, system_id, scenario_key, name, description, status, current_version_id, created_by, created_at, updated_at FROM scenarios WHERE system_id = ? AND id = ? FOR UPDATE`
const nextVersionNoSQL = `SELECT COALESCE(MAX(version_no), 0) + 1 AS next_version_no FROM scenario_versions WHERE system_id = ? AND scenario_id = ?`
const updateScenarioRevisionSQL = `UPDATE scenarios SET name = ?, description = NULLIF(?, ''), status = ?, current_version_id = ?, updated_at = ? WHERE system_id = ? AND id = ? AND current_version_id = ?`
const getVersionSQL = `SELECT id, system_id, scenario_id, version_no, source_type, bundle_document, created_by, created_at FROM scenario_versions WHERE system_id = ? AND id = ?`
const listStepsSQL = `SELECT id, system_id, scenario_version_id, step_key, name, position, step_type, api_operation_id, depends_on, request_config, created_at FROM scenario_steps WHERE system_id = ? AND scenario_version_id = ? ORDER BY position, id`

var candidatePromotionColumns = []string{"id", "system_id", "discovery_id", "candidate_key", "name", "description", "candidate_bundle", "review_status", "promoted_scenario_id"}
var scenarioColumns = []string{"id", "system_id", "scenario_key", "name", "description", "status", "current_version_id", "created_by", "created_at", "updated_at"}

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }
func (r *MySQLRepository) Promote(ctx context.Context, systemID, discoveryID, candidateID, userID string) (result PromotionResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	c, err := scanPromotionCandidate(tx.QueryRowContext(ctx, lockCandidateSQL, systemID, discoveryID, candidateID))
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrCandidateNotFound
	}
	if err != nil {
		return result, err
	}
	if c.PromotedScenarioID != "" {
		d, found, e := getDetail(ctx, tx, systemID, c.PromotedScenarioID)
		if e != nil {
			return result, e
		}
		if !found {
			return result, ErrPromotionConflict
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
		return PromotionResult{Detail: d, Created: false}, nil
	}
	if c.ReviewStatus != "accepted" {
		return result, ErrCandidateNotAccepted
	}
	now := time.Now().UTC()
	scenarioID, e := scenarioUUID()
	if e != nil {
		return result, e
	}
	versionID, e := scenarioUUID()
	if e != nil {
		return result, e
	}
	raw, e := json.Marshal(c.Bundle)
	if e != nil {
		return result, e
	}
	sc := Scenario{ID: scenarioID, SystemID: systemID, Key: c.Key, Name: c.Name, Description: c.Description, Status: "draft", CurrentVersionID: versionID, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	version := Version{ID: versionID, SystemID: systemID, ScenarioID: scenarioID, VersionNo: 1, SourceType: "generated", Bundle: c.Bundle, BundleDocument: raw, CreatedBy: userID, CreatedAt: now}
	if _, err = tx.ExecContext(ctx, insertScenarioSQL, scenarioID, systemID, c.Key, c.Name, c.Description, "draft", userID, now, now); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, insertVersionSQL, versionID, systemID, scenarioID, 1, "generated", raw, userID, now); err != nil {
		return result, err
	}
	steps, e := makeSteps(systemID, versionID, c.Bundle, now)
	if e != nil {
		return result, e
	}
	for _, s := range steps {
		depends, _ := json.Marshal(s.DependsOn)
		if _, err = tx.ExecContext(ctx, insertStepSQL, s.ID, systemID, versionID, s.Key, s.Name, s.Position, "http", s.OperationID, depends, s.RequestConfig, s.CreatedAt); err != nil {
			return result, err
		}
	}
	res, e := tx.ExecContext(ctx, updateCurrentVersionSQL, versionID, now, systemID, scenarioID)
	if e != nil {
		return result, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return result, e
	}
	if n != 1 {
		return result, ErrPromotionConflict
	}
	res, e = tx.ExecContext(ctx, markCandidatePromotedSQL, scenarioID, now, systemID, discoveryID, candidateID, "accepted")
	if e != nil {
		return result, e
	}
	n, e = res.RowsAffected()
	if e != nil {
		return result, e
	}
	if n != 1 {
		return result, ErrPromotionConflict
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return PromotionResult{Detail: Detail{Scenario: sc, Version: version, Steps: steps}, Created: true}, nil
}
func (r *MySQLRepository) List(ctx context.Context, systemID string) ([]Scenario, error) {
	rows, err := r.db.QueryContext(ctx, listScenariosSQL, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Scenario{}
	for rows.Next() {
		s, err := scanScenario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *MySQLRepository) Get(ctx context.Context, systemID, scenarioID string) (Detail, bool, error) {
	return getDetail(ctx, r.db, systemID, scenarioID)
}
func (r *MySQLRepository) Update(ctx context.Context, systemID, scenarioID, userID string, input UpdateRequest) (detail Detail, err error) {
	if err = ValidateUpdate(input); err != nil {
		return detail, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return detail, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	current, err := scanScenario(tx.QueryRowContext(ctx, lockScenarioSQL, systemID, scenarioID))
	if errors.Is(err, sql.ErrNoRows) {
		return detail, ErrScenarioNotFound
	}
	if err != nil {
		return detail, err
	}
	var versionNo int
	if err = tx.QueryRowContext(ctx, nextVersionNoSQL, systemID, scenarioID).Scan(&versionNo); err != nil {
		return detail, err
	}
	versionID, err := scenarioUUID()
	if err != nil {
		return detail, err
	}
	now := time.Now().UTC()
	raw, err := json.Marshal(input)
	if err != nil {
		return detail, err
	}
	if _, err = tx.ExecContext(ctx, insertVersionSQL, versionID, systemID, scenarioID, versionNo, "manual", raw, userID, now); err != nil {
		return detail, err
	}
	steps, err := makeManualSteps(systemID, versionID, input.Steps, now)
	if err != nil {
		return detail, err
	}
	for _, step := range steps {
		depends, marshalErr := json.Marshal(step.DependsOn)
		if marshalErr != nil {
			return detail, marshalErr
		}
		if _, err = tx.ExecContext(ctx, insertStepSQL, step.ID, systemID, versionID, step.Key, step.Name, step.Position, step.Type, step.OperationID, depends, nullableJSON(step.RequestConfig), step.CreatedAt); err != nil {
			return detail, err
		}
	}
	result, err := tx.ExecContext(ctx, updateScenarioRevisionSQL, input.Name, input.Description, input.Status, versionID, now, systemID, scenarioID, current.CurrentVersionID)
	if err != nil {
		return detail, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return detail, err
	}
	if affected != 1 {
		return detail, ErrRevisionConflict
	}
	if err = tx.Commit(); err != nil {
		return detail, err
	}
	current.Name = input.Name
	current.Description = input.Description
	current.Status = input.Status
	current.CurrentVersionID = versionID
	current.UpdatedAt = now
	version := Version{ID: versionID, SystemID: systemID, ScenarioID: scenarioID, VersionNo: versionNo, SourceType: "manual", BundleDocument: raw, CreatedBy: userID, CreatedAt: now}
	return Detail{Scenario: current, Version: version, Steps: steps}, nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getDetail(ctx context.Context, q queryer, systemID, scenarioID string) (Detail, bool, error) {
	s, err := scanScenario(q.QueryRowContext(ctx, getScenarioSQL, systemID, scenarioID))
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, false, nil
	}
	if err != nil {
		return Detail{}, false, err
	}
	v, err := scanVersion(q.QueryRowContext(ctx, getVersionSQL, systemID, s.CurrentVersionID))
	if err != nil {
		return Detail{}, false, err
	}
	rows, err := q.QueryContext(ctx, listStepsSQL, systemID, v.ID)
	if err != nil {
		return Detail{}, false, err
	}
	defer rows.Close()
	steps := []Step{}
	for rows.Next() {
		st, err := scanStep(rows)
		if err != nil {
			return Detail{}, false, err
		}
		steps = append(steps, st)
	}
	return Detail{Scenario: s, Version: v, Steps: steps}, true, rows.Err()
}

type row interface{ Scan(...any) error }

func scanPromotionCandidate(r row) (PromotionCandidate, error) {
	var c PromotionCandidate
	var desc, promoted sql.NullString
	var raw []byte
	if err := r.Scan(&c.ID, &c.SystemID, &c.DiscoveryID, &c.Key, &c.Name, &desc, &raw, &c.ReviewStatus, &promoted); err != nil {
		return c, err
	}
	c.Description = desc.String
	c.PromotedScenarioID = promoted.String
	if err := json.Unmarshal(raw, &c.Bundle); err != nil {
		return c, fmt.Errorf("decode candidate bundle: %w", err)
	}
	return c, nil
}
func scanScenario(r row) (Scenario, error) {
	var s Scenario
	var desc, current sql.NullString
	if err := r.Scan(&s.ID, &s.SystemID, &s.Key, &s.Name, &desc, &s.Status, &current, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return s, err
	}
	s.Description = desc.String
	s.CurrentVersionID = current.String
	return s, nil
}
func scanVersion(r row) (Version, error) {
	var v Version
	if err := r.Scan(&v.ID, &v.SystemID, &v.ScenarioID, &v.VersionNo, &v.SourceType, &v.BundleDocument, &v.CreatedBy, &v.CreatedAt); err != nil {
		return v, err
	}
	if err := json.Unmarshal(v.BundleDocument, &v.Bundle); err != nil {
		return v, err
	}
	return v, nil
}
func scanStep(r row) (Step, error) {
	var s Step
	var op sql.NullString
	var depends, requestConfig []byte
	if err := r.Scan(&s.ID, &s.SystemID, &s.VersionID, &s.Key, &s.Name, &s.Position, &s.Type, &op, &depends, &requestConfig, &s.CreatedAt); err != nil {
		return s, err
	}
	s.OperationID = op.String
	s.RequestConfig = append(json.RawMessage(nil), requestConfig...)
	if len(depends) > 0 {
		_ = json.Unmarshal(depends, &s.DependsOn)
	}
	return s, nil
}

var _ Repository = (*MySQLRepository)(nil)
