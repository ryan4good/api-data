package execution

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ServiceOptions struct {
	NewID func() string
	Now   func() time.Time
}

type Service struct {
	repository Repository
	steps      StepProvider
	executor   StepExecutor
	newID      func() string
	now        func() time.Time
}

func NewService(repository Repository, steps StepProvider, executor StepExecutor, options ServiceOptions) *Service {
	if options.NewID == nil {
		options.NewID = uuid.NewString
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{repository: repository, steps: steps, executor: executor, newID: options.NewID, now: options.Now}
}

func (service *Service) Execute(ctx context.Context, command ExecuteCommand) (Run, error) {
	if command.SystemID == "" || command.ScenarioID == "" || command.ScenarioVersionID == "" || command.EnvironmentID == "" || command.RequestedBy == "" {
		return Run{}, ErrInvalidCommand
	}
	steps, err := service.steps.Steps(ctx, command.SystemID, command.ScenarioVersionID)
	if err != nil {
		return Run{}, err
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Position < steps[j].Position })
	if len(steps) == 0 || command.StopAfterStepID != "" && !hasStep(steps, command.StopAfterStepID) {
		return Run{}, ErrStepNotFound
	}
	now := service.now().UTC()
	run := Run{
		ID: service.newID(), SystemID: command.SystemID, ScenarioID: command.ScenarioID, ScenarioVersionID: command.ScenarioVersionID,
		EnvironmentID: command.EnvironmentID, Status: RunStatusRunning, TriggerType: "manual", InputVariables: redactMap(command.InputVariables),
		Summary: RunSummary{TotalSteps: len(steps), StopAfterStepID: command.StopAfterStepID}, RequestedBy: command.RequestedBy,
		StartedAt: now, CreatedAt: now, Attempts: []StepAttempt{},
	}
	if err := service.repository.CreateRun(ctx, run); err != nil {
		return Run{}, err
	}
	variables := cloneMap(command.InputVariables)
	var infrastructureErr error
	for _, step := range steps {
		attempt, extracted, stepErr := service.executeStep(ctx, run, step, 1, variables)
		if err := service.repository.SaveAttempt(ctx, attempt); err != nil {
			return Run{}, err
		}
		run.Attempts = append(run.Attempts, attempt)
		run.Summary.ExecutedSteps++
		for key, value := range extracted {
			variables[key] = value
		}
		if attempt.Status == StepStatusFailed {
			run.Status, run.Outcome, run.Summary.FailedStepID = RunStatusFailed, OutcomeFailed, step.ID
			if errors.Is(stepErr, ErrExecutorNotConfigured) {
				infrastructureErr = ErrExecutorNotConfigured
			}
			break
		}
		if command.StopAfterStepID == step.ID {
			break
		}
	}
	if run.Status != RunStatusFailed {
		run.Status = RunStatusPassed
		if run.Summary.ExecutedSteps == run.Summary.TotalSteps {
			run.Outcome = OutcomeSucceeded
		} else {
			run.Outcome = OutcomePartial
		}
	}
	run.OutputVariables = redactMap(variables)
	run.FinishedAt = service.now().UTC()
	if err := service.repository.UpdateRun(ctx, run); err != nil {
		return Run{}, err
	}
	if infrastructureErr != nil {
		return run, infrastructureErr
	}
	return run, nil
}

func (service *Service) RetryStep(ctx context.Context, command RetryCommand) (RetryResult, error) {
	if command.SystemID == "" || command.RunID == "" || command.StepID == "" || command.RequestedBy == "" {
		return RetryResult{}, ErrInvalidCommand
	}
	run, err := service.repository.GetRun(ctx, command.SystemID, command.RunID)
	if err != nil {
		return RetryResult{}, err
	}
	steps, err := service.steps.Steps(ctx, command.SystemID, run.ScenarioVersionID)
	if err != nil {
		return RetryResult{}, err
	}
	step, found := findStep(steps, command.StepID)
	if !found {
		return RetryResult{}, ErrStepNotFound
	}
	latestNo := 0
	for _, attempt := range run.Attempts {
		if attempt.StepID == command.StepID && attempt.AttemptNo > latestNo {
			latestNo = attempt.AttemptNo
		}
	}
	if latestNo == 0 {
		return RetryResult{}, ErrStepNotExecuted
	}
	attempt, _, stepErr := service.executeStep(ctx, run, step, latestNo+1, cloneMap(run.OutputVariables))
	if err := service.repository.SaveAttempt(ctx, attempt); err != nil {
		return RetryResult{}, err
	}
	run.Attempts = append(run.Attempts, attempt)
	run.FinishedAt = service.now().UTC()
	if attempt.Status == StepStatusFailed {
		run.Status, run.Outcome, run.Summary.FailedStepID = RunStatusFailed, OutcomeFailed, step.ID
	} else {
		run.Status, run.Summary.FailedStepID = RunStatusPassed, ""
		if run.Summary.ExecutedSteps == run.Summary.TotalSteps {
			run.Outcome = OutcomeSucceeded
		} else {
			run.Outcome = OutcomePartial
		}
	}
	if err := service.repository.UpdateRun(ctx, run); err != nil {
		return RetryResult{}, err
	}
	result := RetryResult{Run: run, Attempt: attempt}
	if errors.Is(stepErr, ErrExecutorNotConfigured) {
		return result, ErrExecutorNotConfigured
	}
	return result, nil
}

func (service *Service) executeStep(ctx context.Context, run Run, step Step, attemptNo int, variables map[string]any) (StepAttempt, map[string]any, error) {
	started := service.now().UTC()
	result, executeErr := service.executor.Execute(ctx, step, variables)
	status := StepStatusPassed
	for _, assertion := range result.Assertions {
		if assertion.Status == AssertionFailed || assertion.Status == AssertionError {
			status = StepStatusFailed
		}
	}
	errorMessage := ""
	if executeErr != nil {
		status = StepStatusFailed
		errorMessage = sanitizeText(executeErr.Error(), variables, step.Config)
	}
	assertions := append([]AssertionResult(nil), result.Assertions...)
	for index := range assertions {
		assertions[index].ID = service.newID()
		assertions[index].Message = sanitizeText(assertions[index].Message, variables, step.Config)
		if sensitiveKey(assertions[index].Key) {
			assertions[index].Expected, assertions[index].Actual = "[REDACTED]", "[REDACTED]"
		}
	}
	finished := service.now().UTC()
	attempt := StepAttempt{
		ID: service.newID(), SystemID: run.SystemID, RunID: run.ID, StepID: step.ID, AttemptNo: attemptNo, Position: step.Position,
		Status: status, RequestSnapshot: redactMap(result.RequestSnapshot), ResponseSnapshot: redactMap(result.ResponseSnapshot),
		ExtractedVariables: redactMap(result.ExtractedVariables), ErrorMessage: errorMessage, DurationMS: result.DurationMS,
		Assertions: assertions, StartedAt: started, FinishedAt: finished, CreatedAt: started,
	}
	return attempt, result.ExtractedVariables, executeErr
}

func hasStep(steps []Step, id string) bool { _, found := findStep(steps, id); return found }

func findStep(steps []Step, id string) (Step, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
	}
	return Step{}, false
}

func sanitizeText(message string, sources ...map[string]any) string {
	for _, source := range sources {
		for _, secret := range sensitiveValues(source) {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	if len(message) > 2048 {
		message = message[:2048]
	}
	return message
}

func redactMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	redacted := make(map[string]any, len(source))
	for key, value := range source {
		if sensitiveKey(key) {
			redacted[key] = "[REDACTED]"
		} else {
			redacted[key] = redactValue(value)
		}
	}
	return redacted
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return redactMap(typed)
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = item
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactValue(typed[index])
		}
		return result
	default:
		return typed
	}
}

func sensitiveValues(source map[string]any) []string {
	var values []string
	for key, value := range source {
		if sensitiveKey(key) {
			if text, ok := value.(string); ok && text != "" {
				values = append(values, text)
			}
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			values = append(values, sensitiveValues(typed)...)
		case map[string]string:
			for nestedKey, nestedValue := range typed {
				if sensitiveKey(nestedKey) && nestedValue != "" {
					values = append(values, nestedValue)
				}
			}
		}
	}
	return values
}

func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, marker := range []string{"authorization", "cookie", "password", "passwd", "secret", "token", "api_key", "apikey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
