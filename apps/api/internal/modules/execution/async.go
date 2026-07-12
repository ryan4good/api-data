package execution

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EnqueueCommand struct {
	SystemID          string
	ScenarioID        string
	ScenarioVersionID string
	EnvironmentID     string
	RequestedBy       string
	RetryKey          string
	InputVariables    map[string]any
	StopAfterStepID   string
	ExecutionTimeout  time.Duration
}

type EnqueueOutcome struct {
	Run      Run  `json:"run"`
	Existing bool `json:"existing"`
}

type QueueServiceOptions struct {
	NewID func() string
	Now   func() time.Time
}

type QueueService struct {
	repository AsyncRepository
	steps      StepProvider
	newID      func() string
	now        func() time.Time
}

func NewQueueService(repository AsyncRepository, steps StepProvider, options QueueServiceOptions) *QueueService {
	if options.NewID == nil {
		options.NewID = uuid.NewString
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &QueueService{repository: repository, steps: steps, newID: options.NewID, now: options.Now}
}

func (service *QueueService) Enqueue(ctx context.Context, command EnqueueCommand) (EnqueueOutcome, error) {
	if command.SystemID == "" || command.ScenarioID == "" || command.ScenarioVersionID == "" || command.EnvironmentID == "" || command.RequestedBy == "" || command.RetryKey == "" {
		return EnqueueOutcome{}, ErrInvalidCommand
	}
	steps, err := service.steps.Steps(ctx, command.SystemID, command.ScenarioVersionID)
	if err != nil {
		return EnqueueOutcome{}, err
	}
	if len(steps) == 0 || command.StopAfterStepID != "" && !hasStep(steps, command.StopAfterStepID) {
		return EnqueueOutcome{}, ErrStepNotFound
	}
	now := service.now().UTC()
	run := Run{
		ID: service.newID(), SystemID: command.SystemID, ScenarioID: command.ScenarioID, ScenarioVersionID: command.ScenarioVersionID,
		EnvironmentID: command.EnvironmentID, Status: RunStatusQueued, TriggerType: "api", InputVariables: redactMap(command.InputVariables),
		Summary:     RunSummary{TotalSteps: len(steps), StopAfterStepID: command.StopAfterStepID, RetryKey: command.RetryKey, ExecutionTimeoutMS: command.ExecutionTimeout.Milliseconds()},
		RequestedBy: command.RequestedBy, CreatedAt: now, Attempts: []StepAttempt{},
	}
	stored, existing, err := service.repository.EnqueueRun(ctx, run)
	return EnqueueOutcome{Run: stored, Existing: existing}, err
}

func (service *QueueService) Cancel(ctx context.Context, systemID, runID string) (Run, error) {
	return service.repository.CancelRun(ctx, systemID, runID, service.now().UTC())
}

type WorkerOptions struct {
	SystemID          string
	WorkerID          string
	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
	Now               func() time.Time
}

type Worker struct {
	repository AsyncRepository
	execution  *Service
	options    WorkerOptions
	mu         sync.RWMutex
	lastErr    error
}

func NewWorker(repository AsyncRepository, execution *Service, options WorkerOptions) *Worker {
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 30 * time.Second
	}
	if options.HeartbeatInterval <= 0 {
		options.HeartbeatInterval = options.LeaseDuration / 3
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Worker{repository: repository, execution: execution, options: options}
}

func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	now := worker.options.Now().UTC()
	if _, err := worker.repository.RequeueExpired(ctx, worker.options.SystemID, now); err != nil {
		return false, err
	}
	run, found, err := worker.repository.LeaseNext(ctx, worker.options.SystemID, worker.options.WorkerID, now, worker.options.LeaseDuration)
	if err != nil || !found {
		return false, err
	}
	executionCtx := ctx
	cancel := func() {}
	if run.Summary.ExecutionTimeoutMS > 0 {
		executionCtx, cancel = context.WithTimeout(ctx, time.Duration(run.Summary.ExecutionTimeoutMS)*time.Millisecond)
	}
	defer cancel()
	stopHeartbeat := make(chan struct{})
	go worker.heartbeatLoop(executionCtx, run.ID, stopHeartbeat)
	err = worker.execution.ExecuteLeased(executionCtx, run)
	close(stopHeartbeat)
	worker.mu.Lock()
	worker.lastErr = err
	worker.mu.Unlock()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(executionCtx.Err(), context.DeadlineExceeded) {
		return true, nil
	}
	return true, err
}

func (worker *Worker) heartbeatLoop(ctx context.Context, runID string, stop <-chan struct{}) {
	ticker := time.NewTicker(worker.options.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := worker.options.Now().UTC()
			_ = worker.repository.Heartbeat(ctx, worker.options.SystemID, runID, worker.options.WorkerID, now.Add(worker.options.LeaseDuration))
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (worker *Worker) LastExecutionError() error {
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	return worker.lastErr
}

func (service *Service) ExecuteLeased(ctx context.Context, run Run) error {
	steps, err := service.steps.Steps(ctx, run.SystemID, run.ScenarioVersionID)
	if err != nil {
		return err
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Position < steps[j].Position })
	run.Status = RunStatusRunning
	run.StartedAt = service.now().UTC()
	variables := cloneMap(run.InputVariables)
	for _, step := range steps {
		current, getErr := service.repository.GetRun(ctx, run.SystemID, run.ID)
		if getErr == nil && current.Status == RunStatusCancelled {
			return nil
		}
		latestAttempt := 0
		for _, attempt := range run.Attempts {
			if attempt.StepID == step.ID && attempt.AttemptNo > latestAttempt {
				latestAttempt = attempt.AttemptNo
			}
		}
		attempt, extracted, stepErr := service.executeStep(ctx, run, step, latestAttempt+1, variables)
		if saveErr := service.repository.SaveAttempt(ctx, attempt); saveErr != nil {
			return saveErr
		}
		if current, getErr := service.repository.GetRun(context.WithoutCancel(ctx), run.SystemID, run.ID); getErr == nil && current.Status == RunStatusCancelled {
			return nil
		}
		run.Attempts = append(run.Attempts, attempt)
		run.Summary.ExecutedSteps++
		for key, value := range extracted {
			variables[key] = value
		}
		if attempt.Status == StepStatusFailed {
			run.Status, run.Outcome, run.Summary.FailedStepID = RunStatusFailed, OutcomeFailed, step.ID
			if errors.Is(stepErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				run.Status = RunStatusTimedOut
			}
			break
		}
		if run.Summary.StopAfterStepID == step.ID {
			break
		}
	}
	if run.Status == RunStatusRunning {
		run.Status = RunStatusPassed
		if run.Summary.ExecutedSteps >= run.Summary.TotalSteps {
			run.Outcome = OutcomeSucceeded
		} else {
			run.Outcome = OutcomePartial
		}
	}
	run.OutputVariables = redactMap(variables)
	run.FinishedAt = service.now().UTC()
	run.Summary.LeaseOwner, run.Summary.LeaseExpiresAt = "", nil
	if err := service.repository.UpdateRun(context.WithoutCancel(ctx), run); err != nil {
		return err
	}
	if run.Status == RunStatusTimedOut {
		return context.DeadlineExceeded
	}
	return nil
}
