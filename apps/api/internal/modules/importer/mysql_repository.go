package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const selectImportBase = `SELECT id, system_id, format, file_name, content_hash, status, raw_document, conversion_report, error_message, imported_by, created_at, updated_at FROM scenario_imports`

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repository *MySQLRepository) Create(ctx context.Context, record ImportRecord) (ImportRecord, error) {
	conversion, err := json.Marshal(record.Conversion)
	if err != nil {
		return ImportRecord{}, err
	}
	_, err = repository.db.ExecContext(ctx, `INSERT INTO scenario_imports
        (id, system_id, format, file_name, content_hash, status, raw_document, conversion_report, error_message, imported_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.SystemID, record.Format, record.FileName, record.ContentHash, record.Status,
		[]byte(record.RawDocument), conversion, nullableString(record.ErrorMessage), record.ImportedBy, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return ImportRecord{}, err
	}
	return record, nil
}

func (repository *MySQLRepository) FindByHash(ctx context.Context, systemID, hash string) (ImportRecord, bool, error) {
	record, err := scanImport(repository.db.QueryRowContext(ctx, selectImportBase+" WHERE system_id = ? AND content_hash = ?", systemID, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return ImportRecord{}, false, nil
	}
	return record, err == nil, err
}

func (repository *MySQLRepository) List(ctx context.Context, systemID string) ([]ImportRecord, error) {
	rows, err := repository.db.QueryContext(ctx, selectImportBase+" WHERE system_id = ? ORDER BY created_at DESC, id DESC", systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []ImportRecord
	for rows.Next() {
		record, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (repository *MySQLRepository) Get(ctx context.Context, systemID, importID string) (ImportRecord, error) {
	record, err := scanImport(repository.db.QueryRowContext(ctx, selectImportBase+" WHERE system_id = ? AND id = ?", systemID, importID))
	if errors.Is(err, sql.ErrNoRows) {
		return ImportRecord{}, ErrNotFound
	}
	return record, err
}

func (repository *MySQLRepository) ConfirmScripts(ctx context.Context, systemID, importID, reviewerID string, now time.Time) (record ImportRecord, err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportRecord{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	record, err = getForUpdate(ctx, tx, systemID, importID)
	if err != nil {
		return ImportRecord{}, err
	}
	if len(record.Conversion.Result.Report.Errors) > 0 || record.Status == StatusFailed {
		return ImportRecord{}, ErrImportHasErrors
	}
	if record.Status != StatusReady {
		return ImportRecord{}, ErrNotReady
	}
	record.Conversion.Review = Review{ScriptsConfirmed: true, ReviewedBy: reviewerID, ReviewedAt: &now}
	payload, err := json.Marshal(record.Conversion)
	if err != nil {
		return ImportRecord{}, err
	}
	result, err := tx.ExecContext(ctx, "UPDATE scenario_imports SET conversion_report = ?, updated_at = ? WHERE system_id = ? AND id = ? AND status = ?", payload, now, systemID, importID, StatusReady)
	if err != nil {
		return ImportRecord{}, err
	}
	if err = requireOneRow(result); err != nil {
		return ImportRecord{}, err
	}
	if err = tx.Commit(); err != nil {
		return ImportRecord{}, err
	}
	record.UpdatedAt = now
	return record, nil
}

func (repository *MySQLRepository) Apply(ctx context.Context, systemID, importID string, now time.Time) (record ImportRecord, err error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportRecord{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	record, err = getForUpdate(ctx, tx, systemID, importID)
	if err != nil {
		return ImportRecord{}, err
	}
	if err = validateApply(record); err != nil {
		return ImportRecord{}, err
	}
	if record.Status == StatusApplied {
		if err = tx.Commit(); err != nil {
			return ImportRecord{}, err
		}
		return record, nil
	}
	result, err := tx.ExecContext(ctx, "UPDATE scenario_imports SET status = ?, updated_at = ? WHERE system_id = ? AND id = ? AND status = ?", StatusApplied, now, systemID, importID, StatusReady)
	if err != nil {
		return ImportRecord{}, err
	}
	if err = requireOneRow(result); err != nil {
		return ImportRecord{}, err
	}
	if err = tx.Commit(); err != nil {
		return ImportRecord{}, err
	}
	record.Status = StatusApplied
	record.UpdatedAt = now
	return record, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanImport(row rowScanner) (ImportRecord, error) {
	var record ImportRecord
	var raw, conversion []byte
	var errorMessage sql.NullString
	err := row.Scan(&record.ID, &record.SystemID, &record.Format, &record.FileName, &record.ContentHash, &record.Status,
		&raw, &conversion, &errorMessage, &record.ImportedBy, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return ImportRecord{}, err
	}
	record.RawDocument = append(json.RawMessage(nil), raw...)
	if len(conversion) > 0 {
		if err := json.Unmarshal(conversion, &record.Conversion); err != nil {
			return ImportRecord{}, err
		}
	}
	if errorMessage.Valid {
		record.ErrorMessage = errorMessage.String
	}
	return record, nil
}

func getForUpdate(ctx context.Context, tx *sql.Tx, systemID, importID string) (ImportRecord, error) {
	record, err := scanImport(tx.QueryRowContext(ctx, selectImportBase+" WHERE system_id = ? AND id = ? FOR UPDATE", systemID, importID))
	if errors.Is(err, sql.ErrNoRows) {
		return ImportRecord{}, ErrNotFound
	}
	return record, err
}

func requireOneRow(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotReady
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ Repository = (*MySQLRepository)(nil)
