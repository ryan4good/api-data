package scanner

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu         sync.RWMutex
	scans      map[string]ScanRun
	operations map[string]APIOperation
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{scans: make(map[string]ScanRun), operations: make(map[string]APIOperation)}
}

func scopedKey(systemID, id string) string { return systemID + "\x00" + id }

func (r *MemoryRepository) CreateScan(_ context.Context, scan ScanRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopedKey(scan.SystemID, scan.ID)
	if _, exists := r.scans[key]; exists {
		return fmt.Errorf("create scan: duplicate id %s", scan.ID)
	}
	r.scans[key] = scan
	return nil
}

func (r *MemoryRepository) GetScan(_ context.Context, systemID, scanID string) (ScanRun, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	scan, found := r.scans[scopedKey(systemID, scanID)]
	return scan, found, nil
}

func (r *MemoryRepository) ListScans(_ context.Context, systemID string) ([]ScanRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ScanRun, 0)
	for _, scan := range r.scans {
		if scan.SystemID == systemID {
			items = append(items, scan)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (r *MemoryRepository) TransitionScan(_ context.Context, systemID, scanID string, from, to Status, update ScanUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopedKey(systemID, scanID)
	scan, found := r.scans[key]
	if !found {
		return ErrScanNotFound
	}
	if scan.Status != from || !CanTransition(from, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, scan.Status, to)
	}
	scan.Status = to
	scan.StartedAt = update.StartedAt
	scan.FinishedAt = update.FinishedAt
	scan.Summary = append([]byte(nil), update.Summary...)
	scan.ErrorMessage = update.ErrorMessage
	r.scans[key] = scan
	return nil
}

func (r *MemoryRepository) UpsertOperations(_ context.Context, systemID, scanID string, operations []APIOperation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, found := r.scans[scopedKey(systemID, scanID)]; !found {
		return ErrScanNotFound
	}
	for _, operation := range operations {
		if operation.SystemID != systemID || operation.ScanRunID != scanID {
			return fmt.Errorf("operation scope does not match scan scope")
		}
	}
	for _, operation := range operations {
		key := scopedKey(systemID, operation.OperationKey)
		if previous, found := r.operations[key]; found {
			operation.ID = previous.ID
			operation.CreatedAt = previous.CreatedAt
		}
		r.operations[key] = operation
	}
	return nil
}

func (r *MemoryRepository) ListOperations(_ context.Context, systemID string) ([]APIOperation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]APIOperation, 0)
	for _, operation := range r.operations {
		if operation.SystemID == systemID {
			items = append(items, operation)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OperationKey < items[j].OperationKey })
	return items, nil
}

var _ Repository = (*MemoryRepository)(nil)
