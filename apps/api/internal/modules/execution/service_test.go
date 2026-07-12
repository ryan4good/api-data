package execution

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	testSystemID   = "51000000-0000-4000-8000-000000000001"
	testScenarioID = "52000000-0000-4000-8000-000000000001"
	testVersionID  = "53000000-0000-4000-8000-000000000001"
	testEnvID      = "54000000-0000-4000-8000-000000000001"
	testUserID     = "55000000-0000-4000-8000-000000000001"
)

type staticPlan []Step

func (plan staticPlan) Steps(context.Context, string, string) ([]Step, error) {
	return append([]Step(nil), plan...), nil
}

type fakeStepExecutor struct {
	calls   []string
	results map[string]StepResult
	errors  map[string]error
}

func (executor *fakeStepExecutor) Execute(_ context.Context, step Step, _ map[string]any) (StepResult, error) {
	executor.calls = append(executor.calls, step.ID)
	return executor.results[step.ID], executor.errors[step.ID]
}

func TestServiceExecutesSequentiallyAndStopsAfterRequestedStep(t *testing.T) {
	repository := NewMemoryRepository()
	executor := &fakeStepExecutor{results: successResults()}
	service := newExecutionService(repository, executor)

	run, err := service.Execute(context.Background(), ExecuteCommand{
		SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID,
		EnvironmentID: testEnvID, RequestedBy: testUserID, StopAfterStepID: "step-2",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.calls, []string{"step-1", "step-2"}) {
		t.Fatalf("executor calls = %#v", executor.calls)
	}
	if run.Status != RunStatusPassed || run.Outcome != OutcomePartial || run.Summary.ExecutedSteps != 2 || run.Summary.TotalSteps != 3 {
		t.Fatalf("unexpected partial run: %#v", run)
	}
	if len(run.Attempts) != 2 {
		t.Fatalf("attempts = %#v", run.Attempts)
	}
}

func TestServiceMarksFullSuccessfulRunAsSucceeded(t *testing.T) {
	repository := NewMemoryRepository()
	executor := &fakeStepExecutor{results: successResults()}
	run, err := newExecutionService(repository, executor).Execute(context.Background(), baseCommand())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Status != RunStatusPassed || run.Outcome != OutcomeSucceeded || run.Summary.ExecutedSteps != 3 {
		t.Fatalf("unexpected successful run: %#v", run)
	}
}

func TestServiceStopsOnFailureAndRecordsAssertions(t *testing.T) {
	repository := NewMemoryRepository()
	results := successResults()
	results["step-2"] = StepResult{
		RequestSnapshot: map[string]any{"method": "POST"}, ResponseSnapshot: map[string]any{"status": 500},
		Assertions: []AssertionResult{{Key: "status-200", Type: "status", Status: AssertionFailed, Expected: 200, Actual: 500, Message: "unexpected status"}},
		DurationMS: 12,
	}
	executor := &fakeStepExecutor{results: results}
	service := newExecutionService(repository, executor)

	run, err := service.Execute(context.Background(), baseCommand())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.calls, []string{"step-1", "step-2"}) || run.Status != RunStatusFailed || run.Outcome != OutcomeFailed {
		t.Fatalf("failure did not stop run: calls=%#v run=%#v", executor.calls, run)
	}
	if run.Attempts[1].Status != StepStatusFailed || len(run.Attempts[1].Assertions) != 1 || run.Attempts[1].DurationMS != 12 {
		t.Fatalf("failure details missing: %#v", run.Attempts[1])
	}
}

func TestServiceRetryCreatesAttemptWithoutRerunningOtherSteps(t *testing.T) {
	repository := NewMemoryRepository()
	results := successResults()
	executor := &fakeStepExecutor{results: results, errors: map[string]error{"step-2": errors.New("first failure")}}
	service := newExecutionService(repository, executor)
	run, err := service.Execute(context.Background(), baseCommand())
	if err != nil {
		t.Fatal(err)
	}
	executor.calls = nil
	delete(executor.errors, "step-2")

	retried, err := service.RetryStep(context.Background(), RetryCommand{SystemID: testSystemID, RunID: run.ID, StepID: "step-2", RequestedBy: testUserID})
	if err != nil {
		t.Fatalf("RetryStep() error = %v", err)
	}
	if !reflect.DeepEqual(executor.calls, []string{"step-2"}) || retried.Attempt.AttemptNo != 2 {
		t.Fatalf("retry reran other steps: calls=%#v result=%#v", executor.calls, retried)
	}
	stored, err := repository.GetRun(context.Background(), testSystemID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Attempts) != 3 || stored.Outcome != OutcomePartial || stored.Status != RunStatusPassed {
		t.Fatalf("attempt history/outcome = %#v", stored)
	}
	_, err = service.RetryStep(context.Background(), RetryCommand{SystemID: testSystemID, RunID: run.ID, StepID: "step-3", RequestedBy: testUserID})
	if !errors.Is(err, ErrStepNotExecuted) {
		t.Fatalf("retry unexecuted error = %v", err)
	}
}

func TestServiceRedactsSensitiveSnapshotsAndErrors(t *testing.T) {
	repository := NewMemoryRepository()
	secret := "sensitive-value"
	executor := &fakeStepExecutor{
		results: map[string]StepResult{"step-1": {
			RequestSnapshot:  map[string]any{"headers": map[string]string{"Authorization": "Bearer " + secret, "X-Trace": "ok"}, "body": map[string]any{"password": secret, "name": "safe"}},
			ResponseSnapshot: map[string]any{"headers": map[string]any{"Set-Cookie": secret}, "body": map[string]any{"token": secret}},
		}},
		errors: map[string]error{"step-1": errors.New("request failed with " + secret)},
	}
	service := NewService(repository, staticPlan{{ID: "step-1", Position: 1}}, executor, ServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	run, err := service.Execute(context.Background(), ExecuteCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, InputVariables: map[string]any{"apiToken": secret}})
	if err != nil {
		t.Fatal(err)
	}
	attempt := run.Attempts[0]
	if containsSensitive(attempt.RequestSnapshot, secret) || containsSensitive(attempt.ResponseSnapshot, secret) || attempt.ErrorMessage == "request failed with "+secret {
		t.Fatalf("sensitive value leaked: %#v", attempt)
	}
}

func newExecutionService(repository Repository, executor StepExecutor) *Service {
	return NewService(repository, staticPlan{
		{ID: "step-1", Position: 1, Name: "one"}, {ID: "step-2", Position: 2, Name: "two"}, {ID: "step-3", Position: 3, Name: "three"},
	}, executor, ServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
}

func baseCommand() ExecuteCommand {
	return ExecuteCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID}
}

func successResults() map[string]StepResult {
	return map[string]StepResult{
		"step-1": {RequestSnapshot: map[string]any{"path": "/one"}, ResponseSnapshot: map[string]any{"status": 200}},
		"step-2": {RequestSnapshot: map[string]any{"path": "/two"}, ResponseSnapshot: map[string]any{"status": 200}},
		"step-3": {RequestSnapshot: map[string]any{"path": "/three"}, ResponseSnapshot: map[string]any{"status": 200}},
	}
}

func sequentialIDs() func() string {
	n := 0
	return func() string {
		n++
		return "id-" + string(rune('a'+n-1))
	}
}

func fixedNow() time.Time { return time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC) }

func containsSensitive(value map[string]any, secret string) bool {
	encoded, _ := json.Marshal(value)
	return bytesContains(encoded, []byte(secret))
}

func bytesContains(value, part []byte) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if reflect.DeepEqual(value[index:index+len(part)], part) {
			return true
		}
	}
	return false
}
