package scenario

import (
	"bizdevops/apps/api/internal/modules/discovery"
	"errors"
	"time"
)

var (
	ErrCandidateNotFound    = errors.New("candidate not found")
	ErrCandidateNotAccepted = errors.New("candidate is not accepted")
	ErrPromotionConflict    = errors.New("candidate promotion conflict")
)

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
	ID            string    `json:"id"`
	SystemID      string    `json:"systemId"`
	VersionID     string    `json:"versionId"`
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Position      int       `json:"position"`
	Type          string    `json:"type"`
	OperationID   string    `json:"operationId,omitempty"`
	DependsOn     []string  `json:"dependsOn"`
	RequestConfig []byte    `json:"requestConfig,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
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
