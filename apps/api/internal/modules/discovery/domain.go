package discovery

import (
	"errors"
	"time"
)

type Type string

const (
	TypeCode   Type = "code"
	TypePRD    Type = "prd"
	TypePrompt Type = "prompt"
	TypeMixed  Type = "mixed"
)

func (t Type) Valid() bool { return t == TypeCode || t == TypePRD || t == TypePrompt || t == TypeMixed }

type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusReady   Status = "ready"
	StatusFailed  Status = "failed"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewAccepted ReviewStatus = "accepted"
	ReviewRejected ReviewStatus = "rejected"
)

const PriorityP0 = "P0"

var (
	ErrCandidateNotFound = errors.New("scenario candidate not found")
	ErrAlreadyReviewed   = errors.New("scenario candidate already reviewed")
	ErrInvalidInput      = errors.New("invalid discovery input")
)

type Discovery struct {
	ID            string     `json:"id"`
	SystemID      string     `json:"systemId"`
	CodeSourceID  string     `json:"codeSourceId,omitempty"`
	Type          Type       `json:"type"`
	Name          string     `json:"name"`
	InputDocument []byte     `json:"inputDocument"`
	Config        []byte     `json:"config,omitempty"`
	Status        Status     `json:"status"`
	Summary       []byte     `json:"summary,omitempty"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
	RequestedBy   string     `json:"requestedBy"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type CandidateStep struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	OperationID string `json:"operationId,omitempty"`
	Method      string `json:"method,omitempty"`
	Path        string `json:"path,omitempty"`
}

type CandidateBundle struct {
	Priority       string          `json:"priority"`
	RequiresReview bool            `json:"requiresReview"`
	SourceRefs     []string        `json:"sourceRefs"`
	Steps          []CandidateStep `json:"steps"`
}

type Candidate struct {
	ID             string          `json:"id"`
	SystemID       string          `json:"systemId"`
	DiscoveryID    string          `json:"discoveryId"`
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Priority       string          `json:"priority"`
	Confidence     float64         `json:"confidence"`
	SourceRefs     []string        `json:"sourceRefs"`
	RequiresReview bool            `json:"requiresReview"`
	Steps          []CandidateStep `json:"steps"`
	Evidence       []byte          `json:"evidence"`
	Bundle         []byte          `json:"candidateBundle"`
	ReviewStatus   ReviewStatus    `json:"reviewStatus"`
	ReviewNote     string          `json:"reviewNote,omitempty"`
	ReviewedBy     string          `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time      `json:"reviewedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}
