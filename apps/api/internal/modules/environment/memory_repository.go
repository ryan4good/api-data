package environment

import (
	"context"
	"errors"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu           sync.RWMutex
	environments map[string]Environment
	references   map[string]SecretReference
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{environments: map[string]Environment{}, references: map[string]SecretReference{}}
}
func scoped(systemID, id string) string { return systemID + "\x00" + id }
func (r *MemoryRepository) UpsertEnvironment(_ context.Context, e Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.environments[scoped(e.SystemID, e.ID)] = cloneEnvironment(e)
	return nil
}
func (r *MemoryRepository) GetEnvironment(_ context.Context, systemID, id string) (Environment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.environments[scoped(systemID, id)]
	return cloneEnvironment(e), ok, nil
}
func (r *MemoryRepository) ListEnvironments(_ context.Context, systemID string) ([]Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Environment{}
	for _, e := range r.environments {
		if e.SystemID == systemID {
			out = append(out, cloneEnvironment(e))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func (r *MemoryRepository) UpsertSecretReference(_ context.Context, ref SecretReference) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.environments[scoped(ref.SystemID, ref.EnvironmentID)]
	if !ok || e.SystemID != ref.SystemID {
		return ErrEnvironmentNotFound
	}
	if ref.SecretRef == "" {
		return errors.New("secret reference is required")
	}
	r.references[scoped(ref.SystemID, ref.EnvironmentID+"\x00"+ref.VariableKey)] = ref
	return nil
}
func (r *MemoryRepository) ListSecretReferences(_ context.Context, systemID, environmentID string) ([]SecretReference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.environments[scoped(systemID, environmentID)]; !ok {
		return nil, ErrEnvironmentNotFound
	}
	out := []SecretReference{}
	for _, ref := range r.references {
		if ref.SystemID == systemID && ref.EnvironmentID == environmentID {
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VariableKey < out[j].VariableKey })
	return out, nil
}
func cloneEnvironment(e Environment) Environment {
	copy := e
	copy.Variables = map[string]string{}
	for k, v := range e.Variables {
		copy.Variables[k] = v
	}
	return copy
}

var _ Repository = (*MemoryRepository)(nil)
