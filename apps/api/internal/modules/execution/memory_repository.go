package execution

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	runs map[string]map[string]Run
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
