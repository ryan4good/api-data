package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const scanSelectColumns = `id, system_id, code_source_id, source_ref, source_commit, language, framework,
       status, config, summary, error_message, requested_by, started_at, finished_at, created_at`

const createScanSQL = `INSERT INTO scan_runs
  (id, system_id, code_source_id, source_ref, source_commit, language, framework, status, config, requested_by, created_at)
VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)`

const getScanSQL = `SELECT ` + scanSelectColumns + `
FROM scan_runs
WHERE system_id = ?
  AND id = ?`

const listScansSQL = `SELECT ` + scanSelectColumns + `
FROM scan_runs
WHERE system_id = ?
ORDER BY created_at DESC, id`

const transitionScanSQL = `UPDATE scan_runs
SET status = ?, started_at = ?, finished_at = ?, summary = ?, error_message = NULLIF(?, '')
WHERE system_id = ?
  AND id = ?
  AND status = ?`

const upsertOperationSQL = `INSERT INTO api_operations
  (id, system_id, scan_run_id, operation_key, method, path, code_evidence, confidence, content_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  scan_run_id = VALUES(scan_run_id), method = VALUES(method), path = VALUES(path),
  code_evidence = VALUES(code_evidence), confidence = VALUES(confidence),
  content_hash = VALUES(content_hash), updated_at = CURRENT_TIMESTAMP(3)`

const listOperationsSQL = `SELECT id, system_id, scan_run_id, operation_key, operation_id, method, path,
       summary, description, code_evidence, confidence, verification_status, lifecycle_status,
       content_hash, created_at, updated_at
FROM api_operations
WHERE system_id = ?
ORDER BY operation_key, id`

var scanColumns = []string{
	"id", "system_id", "code_source_id", "source_ref", "source_commit", "language", "framework",
	"status", "config", "summary", "error_message", "requested_by", "started_at", "finished_at", "created_at",
}

var operationColumns = []string{
	"id", "system_id", "scan_run_id", "operation_key", "operation_id", "method", "path", "summary", "description",
	"code_evidence", "confidence", "verification_status", "lifecycle_status", "content_hash", "created_at", "updated_at",
}

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (r *MySQLRepository) CreateScan(ctx context.Context, scan ScanRun) error {
	_, err := r.db.ExecContext(ctx, createScanSQL,
		scan.ID, scan.SystemID, scan.CodeSourceID, scan.SourceRef, scan.SourceCommit, scan.Language, scan.Framework,
		string(scan.Status), nullableJSON(scan.Config), scan.RequestedBy, scan.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create scan run: %w", err)
	}
	return nil
}

func (r *MySQLRepository) GetScan(ctx context.Context, systemID, scanID string) (ScanRun, bool, error) {
	scan, err := scanScanRun(r.db.QueryRowContext(ctx, getScanSQL, systemID, scanID))
	if errors.Is(err, sql.ErrNoRows) {
		return ScanRun{}, false, nil
	}
	if err != nil {
		return ScanRun{}, false, fmt.Errorf("get scoped scan run: %w", err)
	}
	return scan, true, nil
}

func (r *MySQLRepository) ListScans(ctx context.Context, systemID string) ([]ScanRun, error) {
	rows, err := r.db.QueryContext(ctx, listScansSQL, systemID)
	if err != nil {
		return nil, fmt.Errorf("list scoped scan runs: %w", err)
	}
	defer rows.Close()
	items := make([]ScanRun, 0)
	for rows.Next() {
		item, err := scanScanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan scan run: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scan runs: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) TransitionScan(ctx context.Context, systemID, scanID string, from, to Status, update ScanUpdate) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}
	result, err := r.db.ExecContext(ctx, transitionScanSQL, string(to), update.StartedAt, update.FinishedAt,
		nullableJSON(update.Summary), update.ErrorMessage, systemID, scanID, string(from))
	if err != nil {
		return fmt.Errorf("transition scoped scan run: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read transition result: %w", err)
	}
	if count == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (r *MySQLRepository) UpsertOperations(ctx context.Context, systemID, scanID string, operations []APIOperation) (err error) {
	if len(operations) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin operation upsert: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, operation := range operations {
		if operation.SystemID != systemID || operation.ScanRunID != scanID {
			return errors.New("operation scope does not match scan scope")
		}
		if _, err = tx.ExecContext(ctx, upsertOperationSQL,
			operation.ID, systemID, scanID, operation.OperationKey, operation.Method, operation.Path,
			nullableJSON(operation.CodeEvidence), operation.Confidence, operation.ContentHash,
		); err != nil {
			return fmt.Errorf("upsert scoped api operation: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit operation upsert: %w", err)
	}
	return nil
}

func (r *MySQLRepository) ListOperations(ctx context.Context, systemID string) ([]APIOperation, error) {
	rows, err := r.db.QueryContext(ctx, listOperationsSQL, systemID)
	if err != nil {
		return nil, fmt.Errorf("list scoped api operations: %w", err)
	}
	defer rows.Close()
	items := make([]APIOperation, 0)
	for rows.Next() {
		item, err := scanAPIOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan api operation: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api operations: %w", err)
	}
	return items, nil
}

type scannerRow interface{ Scan(dest ...any) error }

func scanScanRun(row scannerRow) (ScanRun, error) {
	var item ScanRun
	var sourceRef, sourceCommit, language, framework, errorMessage sql.NullString
	var config, summary []byte
	var startedAt, finishedAt sql.NullTime
	if err := row.Scan(
		&item.ID, &item.SystemID, &item.CodeSourceID, &sourceRef, &sourceCommit, &language, &framework,
		&item.Status, &config, &summary, &errorMessage, &item.RequestedBy, &startedAt, &finishedAt, &item.CreatedAt,
	); err != nil {
		return ScanRun{}, err
	}
	item.SourceRef, item.SourceCommit, item.Language, item.Framework = sourceRef.String, sourceCommit.String, language.String, framework.String
	item.Config, item.Summary, item.ErrorMessage = config, summary, errorMessage.String
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	return item, nil
}

func scanAPIOperation(row scannerRow) (APIOperation, error) {
	var item APIOperation
	var scanRunID, operationID, summary, description sql.NullString
	var evidence []byte
	var confidence sql.NullFloat64
	if err := row.Scan(
		&item.ID, &item.SystemID, &scanRunID, &item.OperationKey, &operationID, &item.Method, &item.Path,
		&summary, &description, &evidence, &confidence, &item.VerificationStatus, &item.LifecycleStatus,
		&item.ContentHash, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return APIOperation{}, err
	}
	item.ScanRunID, item.OperationID, item.Summary, item.Description = scanRunID.String, operationID.String, summary.String, description.String
	item.CodeEvidence = evidence
	if confidence.Valid {
		item.Confidence = &confidence.Float64
	}
	return item, nil
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

var _ Repository = (*MySQLRepository)(nil)
