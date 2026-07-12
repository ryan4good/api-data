package scenario

import (
	"bizdevops/apps/api/internal/modules/discovery"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrCandidateNotFound    = errors.New("candidate not found")
	ErrCandidateNotAccepted = errors.New("candidate is not accepted")
	ErrPromotionConflict    = errors.New("candidate promotion conflict")
	ErrInvalidRevision      = errors.New("invalid scenario revision")
	ErrScenarioNotFound     = errors.New("scenario not found")
	ErrRevisionConflict     = errors.New("scenario revision conflict")
)

type UpdateRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Steps       []UpdateStep `json:"steps"`
}

type UpdateStep struct {
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	OperationID   string          `json:"operationId,omitempty"`
	DependsOn     []string        `json:"dependsOn"`
	RequestConfig json.RawMessage `json:"requestConfig,omitempty"`
}

func ValidateUpdate(input UpdateRequest) error {
	if strings.TrimSpace(input.Name) == "" || utf8.RuneCountInString(input.Name) > 255 {
		return invalidRevision("name must be non-empty and at most 255 characters")
	}
	if input.Status != "draft" && input.Status != "active" && input.Status != "archived" {
		return invalidRevision("status must be draft, active, or archived")
	}
	keys := make(map[string]struct{}, len(input.Steps))
	for i, step := range input.Steps {
		if strings.TrimSpace(step.Key) == "" || utf8.RuneCountInString(step.Key) > 128 {
			return invalidRevision("steps[%d].key must be non-empty and at most 128 characters", i)
		}
		if strings.TrimSpace(step.Name) == "" || utf8.RuneCountInString(step.Name) > 255 {
			return invalidRevision("steps[%d].name must be non-empty and at most 255 characters", i)
		}
		if step.Type != "http" && step.Type != "script" && step.Type != "delay" {
			return invalidRevision("steps[%d].type must be http, script, or delay", i)
		}
		if step.OperationID != "" && !scenarioUUIDPattern.MatchString(step.OperationID) {
			return invalidRevision("steps[%d].operationId must be a UUID", i)
		}
		if len(step.RequestConfig) > 0 && string(step.RequestConfig) != "null" {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(step.RequestConfig, &object); err != nil || object == nil {
				return invalidRevision("steps[%d].requestConfig must be a JSON object", i)
			}
		}
		if _, exists := keys[step.Key]; exists {
			return invalidRevision("step key %q is duplicated", step.Key)
		}
		keys[step.Key] = struct{}{}
	}
	graph := make(map[string][]string, len(input.Steps))
	for i, step := range input.Steps {
		seenDependencies := map[string]struct{}{}
		for _, dependency := range step.DependsOn {
			if dependency == step.Key {
				return invalidRevision("steps[%d] cannot depend on itself", i)
			}
			if _, exists := keys[dependency]; !exists {
				return invalidRevision("steps[%d] depends on unknown step %q", i, dependency)
			}
			if _, duplicate := seenDependencies[dependency]; duplicate {
				return invalidRevision("steps[%d] repeats dependency %q", i, dependency)
			}
			seenDependencies[dependency] = struct{}{}
			graph[step.Key] = append(graph[step.Key], dependency)
		}
	}
	state := make(map[string]uint8, len(keys))
	var visit func(string) bool
	visit = func(key string) bool {
		if state[key] == 1 {
			return false
		}
		if state[key] == 2 {
			return true
		}
		state[key] = 1
		for _, dependency := range graph[key] {
			if !visit(dependency) {
				return false
			}
		}
		state[key] = 2
		return true
	}
	for key := range keys {
		if !visit(key) {
			return invalidRevision("step dependencies contain a cycle")
		}
	}
	return nil
}

func invalidRevision(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRevision, fmt.Sprintf(format, args...))
}

type PromotionCandidate struct {
	ID                 string
	SystemID           string
	DiscoveryID        string
	Key                string
	Name               string
	Description        string
	Bundle             discovery.CandidateBundle
	ReviewStatus       string
	PromotedScenarioID string
}
type Scenario struct {
	ID               string    `json:"id"`
	SystemID         string    `json:"systemId"`
	Key              string    `json:"key"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	Status           string    `json:"status"`
	CurrentVersionID string    `json:"currentVersionId"`
	CreatedBy        string    `json:"createdBy"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
type Version struct {
	ID             string                    `json:"id"`
	SystemID       string                    `json:"systemId"`
	ScenarioID     string                    `json:"scenarioId"`
	VersionNo      int                       `json:"versionNo"`
	SourceType     string                    `json:"sourceType"`
	Bundle         discovery.CandidateBundle `json:"bundle"`
	BundleDocument []byte                    `json:"bundleDocument"`
	CreatedBy      string                    `json:"createdBy"`
	CreatedAt      time.Time                 `json:"createdAt"`
}
type Step struct {
	ID            string          `json:"id"`
	SystemID      string          `json:"systemId"`
	VersionID     string          `json:"versionId"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Position      int             `json:"position"`
	Type          string          `json:"type"`
	OperationID   string          `json:"operationId,omitempty"`
	DependsOn     []string        `json:"dependsOn"`
	RequestConfig json.RawMessage `json:"requestConfig,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}
type Detail struct {
	Scenario Scenario `json:"scenario"`
	Version  Version  `json:"version"`
	Steps    []Step   `json:"steps"`
}
type PromotionResult struct {
	Detail
	Created bool `json:"created"`
}
