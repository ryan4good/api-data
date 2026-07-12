package execution

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	runs map[string]map[string]Run
}

func (repository *MemoryRepository) EnqueueRun(_ context.Context, run Run) (Run, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.runs[run.SystemID] == nil {
		repository.runs[run.SystemID] = map[string]Run{}
	}
	for _, existing := range repository.runs[run.SystemID] {
		if run.Summary.RetryKey != "" && existing.Summary.RetryKey == run.Summary.RetryKey {
			return cloneRun(existing), true, nil
		}
	}
	repository.runs[run.SystemID][run.ID] = cloneRun(run)
	return cloneRun(run), false, nil
}

func (repository *MemoryRepository) LeaseNext(_ context.Context, systemID, workerID string, now time.Time, duration time.Duration) (Run, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var selected Run
	for _, run := range repository.runs[systemID] {
		if run.Status == RunStatusQueued && (selected.ID == "" || run.CreatedAt.Before(selected.CreatedAt)) {
			selected = run
		}
	}
	if selected.ID == "" {
		return Run{}, false, nil
	}
	expires := now.Add(duration)
	selected.Status, selected.Summary.LeaseOwner, selected.Summary.LeaseExpiresAt = RunStatusRunning, workerID, &expires
	repository.runs[systemID][selected.ID] = cloneRun(selected)
	return cloneRun(selected), true, nil
}

func (repository *MemoryRepository) Heartbeat(_ context.Context, systemID, runID, workerID string, expires time.Time) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run, found := repository.runs[systemID][runID]
	if !found || run.Status != RunStatusRunning || run.Summary.LeaseOwner != workerID {
		return ErrLeaseLost
	}
	run.Summary.LeaseExpiresAt = &expires
	repository.runs[systemID][runID] = run
	return nil
}

func (repository *MemoryRepository) CancelRun(_ context.Context, systemID, runID string, now time.Time) (Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run, found := repository.runs[systemID][runID]
	if !found {
		return Run{}, ErrRunNotFound
	}
	if run.Status != RunStatusQueued && run.Status != RunStatusRunning {
		return Run{}, ErrRunNotCancellable
	}
	run.Status, run.FinishedAt = RunStatusCancelled, now
	run.Summary.LeaseOwner, run.Summary.LeaseExpiresAt = "", nil
	repository.runs[systemID][runID] = run
	return cloneRun(run), nil
}

func (repository *MemoryRepository) RequeueExpired(_ context.Context, systemID string, now time.Time) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var count int64
	for id, run := range repository.runs[systemID] {
		if run.Status == RunStatusRunning && run.Summary.LeaseExpiresAt != nil && !run.Summary.LeaseExpiresAt.After(now) {
			run.Status, run.Summary.LeaseOwner, run.Summary.LeaseExpiresAt = RunStatusQueued, "", nil
			repository.runs[systemID][id] = run
			count++
		}
	}
	return count, nil
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{runs: map[string]map[string]Run{}}
}

func (repository *MemoryRepository) CreateRun(_ context.Context, run Run) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.runs[run.SystemID] == nil {
		repository.runs[run.SystemID] = map[string]Run{}
	}
	repository.runs[run.SystemID][run.ID] = cloneRun(run)
	return nil
}

func (repository *MemoryRepository) SaveAttempt(_ context.Context, attempt StepAttempt) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run, found := repository.runs[attempt.SystemID][attempt.RunID]
	if !found {
		return ErrRunNotFound
	}
	run.Attempts = append(run.Attempts, cloneAttempt(attempt))
	repository.runs[attempt.SystemID][attempt.RunID] = run
	return nil
}

func (repository *MemoryRepository) UpdateRun(_ context.Context, run Run) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	stored, found := repository.runs[run.SystemID][run.ID]
	if !found {
		return ErrRunNotFound
	}
	run.Attempts = stored.Attempts
	repository.runs[run.SystemID][run.ID] = cloneRun(run)
	return nil
}

func (repository *MemoryRepository) GetRun(_ context.Context, systemID, runID string) (Run, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	run, found := repository.runs[systemID][runID]
	if !found {
		return Run{}, ErrRunNotFound
	}
	return cloneRun(run), nil
}

func (repository *MemoryRepository) ListRuns(_ context.Context, systemID string) ([]Run, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	runs := make([]Run, 0, len(repository.runs[systemID]))
	for _, run := range repository.runs[systemID] {
		runs = append(runs, cloneRun(run))
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].CreatedAt.After(runs[j].CreatedAt) })
	return runs, nil
}

func cloneRun(run Run) Run {
	payload, _ := json.Marshal(run)
	var clone Run
	_ = json.Unmarshal(payload, &clone)
	return clone
}

func cloneAttempt(attempt StepAttempt) StepAttempt {
	payload, _ := json.Marshal(attempt)
	var clone StepAttempt
	_ = json.Unmarshal(payload, &clone)
	return clone
}

var _ Repository = (*MemoryRepository)(nil)
var _ AsyncRepository = (*MemoryRepository)(nil)
