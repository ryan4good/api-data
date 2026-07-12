package discovery

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	discoveries map[string]Discovery
	candidates  map[string]Candidate
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{discoveries: map[string]Discovery{}, candidates: map[string]Candidate{}}
}
func memoryKey(systemID, id string) string { return systemID + "\x00" + id }
func (r *MemoryRepository) CreateWithCandidates(_ context.Context, discovery Discovery, candidates []Candidate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, candidate := range candidates {
		if candidate.SystemID != discovery.SystemID || candidate.DiscoveryID != discovery.ID {
			return errors.New("candidate scope mismatch")
		}
	}
	r.discoveries[memoryKey(discovery.SystemID, discovery.ID)] = discovery
	for _, candidate := range candidates {
		r.candidates[memoryKey(candidate.SystemID, candidate.ID)] = candidate
	}
	return nil
}
func (r *MemoryRepository) ListDiscoveries(_ context.Context, systemID string) ([]Discovery, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Discovery{}
	for _, item := range r.discoveries {
		if item.SystemID == systemID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (r *MemoryRepository) ListCandidates(_ context.Context, systemID, discoveryID string) ([]Candidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Candidate{}
	for _, item := range r.candidates {
		if item.SystemID == systemID && item.DiscoveryID == discoveryID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func (r *MemoryRepository) ReviewCandidate(_ context.Context, systemID, candidateID string, status ReviewStatus, note, reviewerID string, reviewedAt time.Time) (Candidate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := memoryKey(systemID, candidateID)
	item, ok := r.candidates[key]
	if !ok {
		return Candidate{}, ErrCandidateNotFound
	}
	if item.ReviewStatus != ReviewPending {
		return Candidate{}, ErrAlreadyReviewed
	}
	item.ReviewStatus = status
	item.ReviewNote = note
	item.ReviewedBy = reviewerID
	item.ReviewedAt = &reviewedAt
	item.UpdatedAt = reviewedAt
	r.candidates[key] = item
	return item, nil
}

var _ Repository = (*MemoryRepository)(nil)
