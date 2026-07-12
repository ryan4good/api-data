package scenario

import (
	"bizdevops/apps/api/internal/modules/discovery"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu         sync.RWMutex
	candidates map[string]PromotionCandidate
	scenarios  map[string]Scenario
	details    map[string]Detail
}

func NewMemoryRepository(candidates []PromotionCandidate) *MemoryRepository {
	r := &MemoryRepository{candidates: map[string]PromotionCandidate{}, scenarios: map[string]Scenario{}, details: map[string]Detail{}}
	for _, c := range candidates {
		r.candidates[key(c.SystemID, c.ID)] = c
	}
	return r
}
func key(systemID, id string) string { return systemID + "\x00" + id }
func (r *MemoryRepository) Promote(_ context.Context, systemID, discoveryID, candidateID, userID string) (PromotionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ck := key(systemID, candidateID)
	c, ok := r.candidates[ck]
	if !ok || c.DiscoveryID != discoveryID {
		return PromotionResult{}, ErrCandidateNotFound
	}
	if c.PromotedScenarioID != "" {
		d, ok := r.details[key(systemID, c.PromotedScenarioID)]
		if !ok {
			return PromotionResult{}, ErrPromotionConflict
		}
		return PromotionResult{Detail: d, Created: false}, nil
	}
	if c.ReviewStatus != "accepted" {
		return PromotionResult{}, ErrCandidateNotAccepted
	}
	now := time.Now().UTC()
	scenarioID, err := scenarioUUID()
	if err != nil {
		return PromotionResult{}, err
	}
	versionID, err := scenarioUUID()
	if err != nil {
		return PromotionResult{}, err
	}
	sc := Scenario{ID: scenarioID, SystemID: systemID, Key: c.Key, Name: c.Name, Description: c.Description, Status: "draft", CurrentVersionID: versionID, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	raw, _ := json.Marshal(c.Bundle)
	version := Version{ID: versionID, SystemID: systemID, ScenarioID: scenarioID, VersionNo: 1, SourceType: "generated", Bundle: c.Bundle, BundleDocument: raw, CreatedBy: userID, CreatedAt: now}
	steps, err := makeSteps(systemID, versionID, c.Bundle, now)
	if err != nil {
		return PromotionResult{}, err
	}
	d := Detail{Scenario: sc, Version: version, Steps: steps}
	r.scenarios[key(systemID, scenarioID)] = sc
	r.details[key(systemID, scenarioID)] = d
	c.PromotedScenarioID = scenarioID
	r.candidates[ck] = c
	return PromotionResult{Detail: d, Created: true}, nil
}
func (r *MemoryRepository) List(_ context.Context, systemID string) ([]Scenario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Scenario{}
	for _, s := range r.scenarios {
		if s.SystemID == systemID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func (r *MemoryRepository) Get(_ context.Context, systemID, scenarioID string) (Detail, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.details[key(systemID, scenarioID)]
	return d, ok, nil
}
func makeSteps(systemID, versionID string, b discovery.CandidateBundle, now time.Time) ([]Step, error) {
	out := make([]Step, 0, len(b.Steps))
	for i, s := range b.Steps {
		id, err := scenarioUUID()
		if err != nil {
			return nil, err
		}
		depends := []string{}
		if i > 0 {
			depends = []string{b.Steps[i-1].Key}
		}
		request, _ := json.Marshal(map[string]string{"method": s.Method, "path": s.Path})
		out = append(out, Step{ID: id, SystemID: systemID, VersionID: versionID, Key: s.Key, Name: s.Name, Position: i + 1, Type: "http", OperationID: s.OperationID, DependsOn: depends, RequestConfig: request, CreatedAt: now})
	}
	return out, nil
}
func scenarioUUID() (string, error) {
	var v [16]byte
	if _, err := rand.Read(v[:]); err != nil {
		return "", err
	}
	v[6] = (v[6] & 15) | 64
	v[8] = (v[8] & 63) | 128
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
}

var _ Repository = (*MemoryRepository)(nil)
