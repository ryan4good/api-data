package importer

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	records map[string]map[string]ImportRecord
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: map[string]map[string]ImportRecord{}}
}

func (repository *MemoryRepository) Create(_ context.Context, record ImportRecord) (ImportRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.records[record.SystemID] == nil {
		repository.records[record.SystemID] = map[string]ImportRecord{}
	}
	for _, existing := range repository.records[record.SystemID] {
		if existing.ContentHash == record.ContentHash {
			return ImportRecord{}, ErrDuplicateHash
		}
	}
	repository.records[record.SystemID][record.ID] = cloneImport(record)
	return cloneImport(record), nil
}

func (repository *MemoryRepository) FindByHash(_ context.Context, systemID, hash string) (ImportRecord, bool, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	for _, record := range repository.records[systemID] {
		if record.ContentHash == hash {
			return cloneImport(record), true, nil
		}
	}
	return ImportRecord{}, false, nil
}

func (repository *MemoryRepository) List(_ context.Context, systemID string) ([]ImportRecord, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	items := make([]ImportRecord, 0, len(repository.records[systemID]))
	for _, record := range repository.records[systemID] {
		items = append(items, cloneImport(record))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (repository *MemoryRepository) Get(_ context.Context, systemID, importID string) (ImportRecord, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	record, found := repository.records[systemID][importID]
	if !found {
		return ImportRecord{}, ErrNotFound
	}
	return cloneImport(record), nil
}

func (repository *MemoryRepository) ConfirmScripts(_ context.Context, systemID, importID, reviewerID string, now time.Time) (ImportRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	record, found := repository.records[systemID][importID]
	if !found {
		return ImportRecord{}, ErrNotFound
	}
	if len(record.Conversion.Result.Report.Errors) > 0 || record.Status == StatusFailed {
		return ImportRecord{}, ErrImportHasErrors
	}
	if record.Status != StatusReady {
		return ImportRecord{}, ErrNotReady
	}
	record.Conversion.Review = Review{ScriptsConfirmed: true, ReviewedBy: reviewerID, ReviewedAt: &now}
	record.UpdatedAt = now
	repository.records[systemID][importID] = record
	return cloneImport(record), nil
}

func (repository *MemoryRepository) Apply(_ context.Context, systemID, importID string, now time.Time) (ImportRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	record, found := repository.records[systemID][importID]
	if !found {
		return ImportRecord{}, ErrNotFound
	}
	if err := validateApply(record); err != nil {
		return ImportRecord{}, err
	}
	if record.Status != StatusApplied {
		record.Status = StatusApplied
		record.UpdatedAt = now
		repository.records[systemID][importID] = record
	}
	return cloneImport(record), nil
}

func cloneImport(record ImportRecord) ImportRecord {
	record.RawDocument = append([]byte(nil), record.RawDocument...)
	return record
}

var _ Repository = (*MemoryRepository)(nil)
