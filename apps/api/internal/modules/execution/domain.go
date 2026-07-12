package execution

import (
	"context"
	"errors"
	"time"
)

const (
	RunStatusQueued    = "queued"
	RunStatusRunning   = "running"
	RunStatusPassed    = "passed"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"
	RunStatusTimedOut  = "timed_out"

	OutcomeSucceeded = "succeeded"
	OutcomePartial   = "partial"
	OutcomeFailed    = "failed"

	StepStatusPassed = "passed"
	StepStatusFailed = "failed"

	AssertionPassed  = "passed"
	AssertionFailed  = "failed"
	AssertionSkipped = "skipped"
	AssertionError   = "error"
)

var (
	ErrRunNotFound           = errors.New("scenario run not found")
	ErrStepNotFound          = errors.New("scenario step not found")
	ErrStepNotExecuted       = errors.New("scenario step has not been executed")
	ErrInvalidCommand        = errors.New("invalid execution command")
	ErrExecutorNotConfigured = errors.New("step executor is not configured")
	ErrRunNotCancellable     = errors.New("scenario run cannot be cancelled")
	ErrLeaseLost             = errors.New("scenario run lease was lost")
)

type Step struct {
	ID       string         `json:"id"`
	Position int            `json:"position"`
	Name     string         `json:"name"`
	Type     string         `json:"type,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
}

type AssertionResult struct {
	ID         string `json:"id,omitempty"`
	Key        string `json:"key"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Expected   any    `json:"expected,omitempty"`
	Actual     any    `json:"actual,omitempty"`
	Message    string `json:"message,omitempty"`
	DurationMS int    `json:"durationMs,omitempty"`
}

type StepResult struct {
	RequestSnapshot    map[string]any
	ResponseSnapshot   map[string]any
	ExtractedVariables map[string]any
	Assertions         []AssertionResult
	DurationMS         int
}

type StepAttempt struct {
	ID                 string            `json:"id"`
	SystemID           string            `json:"systemId"`
	RunID              string            `json:"runId"`
	StepID             string            `json:"stepId"`
	AttemptNo          int               `json:"attemptNo"`
	Position           int               `json:"position"`
	Status             string            `json:"status"`
	RequestSnapshot    map[string]any    `json:"requestSnapshot,omitempty"`
	ResponseSnapshot   map[string]any    `json:"responseSnapshot,omitempty"`
	ExtractedVariables map[string]any    `json:"extractedVariables,omitempty"`
	ErrorMessage       string            `json:"errorMessage,omitempty"`
	DurationMS         int               `json:"durationMs"`
	Assertions         []AssertionResult `json:"assertions"`
	StartedAt          time.Time         `json:"startedAt"`
	FinishedAt         time.Time         `json:"finishedAt"`
	CreatedAt          time.Time         `json:"createdAt"`
}

type RunSummary struct {
	TotalSteps         int        `json:"totalSteps"`
	ExecutedSteps      int        `json:"executedSteps"`
	FailedStepID       string     `json:"failedStepId,omitempty"`
	StopAfterStepID    string     `json:"stopAfterStepId,omitempty"`
	RetryKey           string     `json:"retryKey,omitempty"`
	LeaseOwner         string     `json:"leaseOwner,omitempty"`
	LeaseExpiresAt     *time.Time `json:"leaseExpiresAt,omitempty"`
	ExecutionTimeoutMS int64      `json:"executionTimeoutMs,omitempty"`
}

type Run struct {
	ID                string         `json:"id"`
	SystemID          string         `json:"systemId"`
	ScenarioID        string         `json:"scenarioId"`
	ScenarioVersionID string         `json:"scenarioVersionId"`
	EnvironmentID     string         `json:"environmentId"`
	Status            string         `json:"status"`
	Outcome           string         `json:"outcome"`
	TriggerType       string         `json:"triggerType"`
	InputVariables    map[string]any `json:"inputVariables,omitempty"`
	OutputVariables   map[string]any `json:"outputVariables,omitempty"`
	Summary           RunSummary     `json:"summary"`
	RequestedBy       string         `json:"requestedBy"`
	StartedAt         time.Time      `json:"startedAt"`
	FinishedAt        time.Time      `json:"finishedAt"`
	CreatedAt         time.Time      `json:"createdAt"`
	Attempts          []StepAttempt  `json:"attempts"`
}

type ExecuteCommand struct {
	SystemID          string
	ScenarioID        string
	ScenarioVersionID string
	EnvironmentID     string
	RequestedBy       string
	StopAfterStepID   string
	InputVariables    map[string]any
}

type RetryCommand struct {
	SystemID    string
	RunID       string
	StepID      string
	RequestedBy string
}

type RetryResult struct {
	Run     Run         `json:"run"`
	Attempt StepAttempt `json:"attempt"`
}

type StepProvider interface {
	Steps(context.Context, string, string) ([]Step, error)
}

type StepExecutor interface {
	Execute(context.Context, Step, map[string]any) (StepResult, error)
}

type Repository interface {
	CreateRun(context.Context, Run) error
	SaveAttempt(context.Context, StepAttempt) error
	UpdateRun(context.Context, Run) error
	GetRun(context.Context, string, string) (Run, error)
	ListRuns(context.Context, string) ([]Run, error)
}

type AsyncRepository interface {
	Repository
	EnqueueRun(context.Context, Run) (Run, bool, error)
	LeaseNext(context.Context, string, string, time.Time, time.Duration) (Run, bool, error)
	Heartbeat(context.Context, string, string, string, time.Time) error
	CancelRun(context.Context, string, string, time.Time) (Run, error)
	RequeueExpired(context.Context, string, time.Time) (int64, error)
}
