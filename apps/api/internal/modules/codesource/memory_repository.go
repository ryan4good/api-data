package codesource

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	sources map[string]CodeSource
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{sources: map[string]CodeSource{}}
}
func memoryKey(systemID, id string) string { return systemID + "\x00" + id }
func (r *MemoryRepository) Create(_ context.Context, source CodeSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.sources {
		if item.SystemID == source.SystemID && strings.EqualFold(item.Name, source.Name) {
			return ErrNameConflict
		}
	}
	now := time.Now().UTC()
	source.CreatedAt = now
	source.UpdatedAt = now
	r.sources[memoryKey(source.SystemID, source.ID)] = clone(source)
	return nil
}
func (r *MemoryRepository) Update(_ context.Context, source CodeSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := memoryKey(source.SystemID, source.ID)
	existing, ok := r.sources[key]
	if !ok {
		return ErrNotFound
	}
	for otherKey, item := range r.sources {
		if otherKey != key && item.SystemID == source.SystemID && strings.EqualFold(item.Name, source.Name) {
			return ErrNameConflict
		}
	}
	source.CreatedBy, source.CreatedAt, source.UpdatedAt = existing.CreatedBy, existing.CreatedAt, time.Now().UTC()
	r.sources[key] = clone(source)
	return nil
}
func (r *MemoryRepository) Get(_ context.Context, systemID, id string) (CodeSource, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.sources[memoryKey(systemID, id)]
	return clone(item), ok, nil
}
func (r *MemoryRepository) List(_ context.Context, systemID string) ([]CodeSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []CodeSource{}
	for _, item := range r.sources {
		if item.SystemID == systemID {
			out = append(out, clone(item))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
func clone(source CodeSource) CodeSource {
	source.IncludePaths = append([]string{}, source.IncludePaths...)
	source.ExcludePaths = append([]string{}, source.ExcludePaths...)
	return source
}

var _ Repository = (*MemoryRepository)(nil)
