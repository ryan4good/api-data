package scanner

import (
	"context"
	"errors"
	"time"
)

var (
	ErrScanNotFound      = errors.New("scan run not found")
	ErrInvalidTransition = errors.New("invalid scan status transition")
)

type ScanRun struct {
	ID           string     `json:"id"`
	SystemID     string     `json:"systemId"`
	CodeSourceID string     `json:"codeSourceId"`
	SourceRef    string     `json:"sourceRef,omitempty"`
	SourceCommit string     `json:"sourceCommit,omitempty"`
	Language     string     `json:"language,omitempty"`
	Framework    string     `json:"framework,omitempty"`
	Status       Status     `json:"status"`
	Config       []byte     `json:"config,omitempty"`
	Summary      []byte     `json:"summary,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	RequestedBy  string     `json:"requestedBy"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type APIOperation struct {
	ID                 string    `json:"id"`
	SystemID           string    `json:"systemId"`
	ScanRunID          string    `json:"scanRunId"`
	OperationKey       string    `json:"operationKey"`
	OperationID        string    `json:"operationId,omitempty"`
	Method             string    `json:"method"`
	Path               string    `json:"path"`
	Summary            string    `json:"summary,omitempty"`
	Description        string    `json:"description,omitempty"`
	CodeEvidence       []byte    `json:"codeEvidence,omitempty"`
	Confidence         *float64  `json:"confidence,omitempty"`
	VerificationStatus string    `json:"verificationStatus"`
	LifecycleStatus    string    `json:"lifecycleStatus"`
	ContentHash        string    `json:"contentHash"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ScanUpdate struct {
	StartedAt    *time.Time
	FinishedAt   *time.Time
	Summary      []byte
	ErrorMessage string
}

type Repository interface {
	CreateScan(ctx context.Context, scan ScanRun) error
	GetScan(ctx context.Context, systemID, scanID string) (ScanRun, bool, error)
	ListScans(ctx context.Context, systemID string) ([]ScanRun, error)
	TransitionScan(ctx context.Context, systemID, scanID string, from, to Status, update ScanUpdate) error
	UpsertOperations(ctx context.Context, systemID, scanID string, operations []APIOperation) error
	ListOperations(ctx context.Context, systemID string) ([]APIOperation, error)
}
