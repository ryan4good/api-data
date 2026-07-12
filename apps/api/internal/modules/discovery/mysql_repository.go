package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const insertDiscoverySQL = `INSERT INTO scenario_discoveries
  (id, system_id, code_source_id, discovery_type, name, input_document, config, status, summary, requested_by, created_at, updated_at)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const insertCandidateSQL = `INSERT INTO scenario_candidates
  (id, system_id, discovery_id, candidate_key, name, description, confidence, evidence, candidate_bundle, review_status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)`
const listDiscoveriesSQL = `SELECT id, system_id, code_source_id, discovery_type, name, input_document, config, status, summary,
       error_message, requested_by, started_at, finished_at, created_at, updated_at
FROM scenario_discoveries WHERE system_id = ? ORDER BY created_at DESC, id`
const listCandidatesSQL = `SELECT id, system_id, discovery_id, candidate_key, name, description, confidence, evidence, candidate_bundle,
       review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
FROM scenario_candidates WHERE system_id = ? AND discovery_id = ? ORDER BY candidate_key, id`
const getCandidateSQL = `SELECT id, system_id, discovery_id, candidate_key, name, description, confidence, evidence, candidate_bundle,
       review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
FROM scenario_candidates WHERE system_id = ? AND id = ?`
const reviewCandidateSQL = `UPDATE scenario_candidates
SET review_status = ?, review_note = NULLIF(?, ''), reviewed_by = ?, reviewed_at = ?, updated_at = ?
WHERE system_id = ? AND id = ? AND review_status = ?`

var discoveryColumns = []string{"id", "system_id", "code_source_id", "discovery_type", "name", "input_document", "config", "status", "summary", "error_message", "requested_by", "started_at", "finished_at", "created_at", "updated_at"}
var candidateColumns = []string{"id", "system_id", "discovery_id", "candidate_key", "name", "description", "confidence", "evidence", "candidate_bundle", "review_status", "review_note", "reviewed_by", "reviewed_at", "created_at", "updated_at"}

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (r *MySQLRepository) CreateWithCandidates(ctx context.Context, d Discovery, candidates []Candidate) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin discovery transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	_, err = tx.ExecContext(ctx, insertDiscoverySQL, d.ID, d.SystemID, d.CodeSourceID, string(d.Type), d.Name, d.InputDocument, nullableJSON(d.Config), string(d.Status), nullableJSON(d.Summary), d.RequestedBy, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert discovery: %w", err)
	}
	for _, c := range candidates {
		if c.SystemID != d.SystemID || c.DiscoveryID != d.ID {
			return errors.New("candidate scope mismatch")
		}
		_, err = tx.ExecContext(ctx, insertCandidateSQL, c.ID, c.SystemID, c.DiscoveryID, c.Key, c.Name, c.Description, c.Confidence, c.Evidence, c.Bundle, string(c.ReviewStatus), c.CreatedAt, c.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert candidate: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit discovery: %w", err)
	}
	return nil
}
func (r *MySQLRepository) ListDiscoveries(ctx context.Context, systemID string) ([]Discovery, error) {
	rows, err := r.db.QueryContext(ctx, listDiscoveriesSQL, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Discovery{}
	for rows.Next() {
		item, err := scanDiscovery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *MySQLRepository) ListCandidates(ctx context.Context, systemID, discoveryID string) ([]Candidate, error) {
	rows, err := r.db.QueryContext(ctx, listCandidatesSQL, systemID, discoveryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Candidate{}
	for rows.Next() {
		item, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *MySQLRepository) ReviewCandidate(ctx context.Context, systemID, candidateID string, status ReviewStatus, note, reviewerID string, at time.Time) (Candidate, error) {
	result, err := r.db.ExecContext(ctx, reviewCandidateSQL, string(status), note, reviewerID, at, at, systemID, candidateID, string(ReviewPending))
	if err != nil {
		return Candidate{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Candidate{}, err
	}
	if n == 0 {
		item, lookupErr := scanCandidate(r.db.QueryRowContext(ctx, getCandidateSQL, systemID, candidateID))
		if errors.Is(lookupErr, sql.ErrNoRows) {
			return Candidate{}, ErrCandidateNotFound
		}
		if lookupErr != nil {
			return Candidate{}, lookupErr
		}
		if item.ReviewStatus != ReviewPending {
			return Candidate{}, ErrAlreadyReviewed
		}
		return Candidate{}, ErrCandidateNotFound
	}
	item, err := scanCandidate(r.db.QueryRowContext(ctx, getCandidateSQL, systemID, candidateID))
	if errors.Is(err, sql.ErrNoRows) {
		return Candidate{}, ErrCandidateNotFound
	}
	return item, err
}

type rowScanner interface{ Scan(...any) error }

func scanDiscovery(row rowScanner) (Discovery, error) {
	var d Discovery
	var code, errorMessage sql.NullString
	var config, summary []byte
	var started, finished sql.NullTime
	if err := row.Scan(&d.ID, &d.SystemID, &code, &d.Type, &d.Name, &d.InputDocument, &config, &d.Status, &summary, &errorMessage, &d.RequestedBy, &started, &finished, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return d, err
	}
	d.CodeSourceID = code.String
	d.Config = config
	d.Summary = summary
	d.ErrorMessage = errorMessage.String
	if started.Valid {
		d.StartedAt = &started.Time
	}
	if finished.Valid {
		d.FinishedAt = &finished.Time
	}
	return d, nil
}
func scanCandidate(row rowScanner) (Candidate, error) {
	var c Candidate
	var description, note, reviewer sql.NullString
	var reviewed sql.NullTime
	if err := row.Scan(&c.ID, &c.SystemID, &c.DiscoveryID, &c.Key, &c.Name, &description, &c.Confidence, &c.Evidence, &c.Bundle, &c.ReviewStatus, &note, &reviewer, &reviewed, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	c.Description = description.String
	c.ReviewNote = note.String
	c.ReviewedBy = reviewer.String
	if reviewed.Valid {
		c.ReviewedAt = &reviewed.Time
	}
	var bundle CandidateBundle
	if err := json.Unmarshal(c.Bundle, &bundle); err != nil {
		return c, fmt.Errorf("decode candidate bundle: %w", err)
	}
	c.Priority = bundle.Priority
	c.RequiresReview = bundle.RequiresReview
	c.SourceRefs = bundle.SourceRefs
	c.Steps = bundle.Steps
	return c, nil
}
func nullableJSON(v []byte) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

var _ Repository = (*MySQLRepository)(nil)
