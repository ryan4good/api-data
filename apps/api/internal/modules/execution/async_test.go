package execution

import (
	"context"
	"errors"
	"testing"
	"time"
)

type blockingExecutor struct{}

func (blockingExecutor) Execute(ctx context.Context, _ Step, _ map[string]any) (StepResult, error) {
	<-ctx.Done()
	return StepResult{}, ctx.Err()
}

type controlledExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (executor controlledExecutor) Execute(context.Context, Step, map[string]any) (StepResult, error) {
	close(executor.started)
	<-executor.release
	return StepResult{}, nil
}

func TestQueueServiceEnqueueIsRetryKeyIdempotentAndSystemScoped(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	command := EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "deploy-42"}
	first, err := queue.Enqueue(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := queue.Enqueue(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if first.Run.ID != second.Run.ID || !second.Existing || first.Run.Status != RunStatusQueued {
		t.Fatalf("idempotency = first %#v second %#v", first, second)
	}
	command.SystemID = "51000000-0000-4000-8000-000000000002"
	third, err := queue.Enqueue(context.Background(), command)
	if err != nil || third.Run.ID == first.Run.ID || third.Existing {
		t.Fatalf("cross-system retry key leaked: %#v %v", third, err)
	}
}

func TestMemoryLeaseIsExclusiveAndExpiredRunCanBeRequeued(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	outcome, _ := queue.Enqueue(context.Background(), EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "one"})
	now := fixedNow()
	leased, found, err := repository.LeaseNext(context.Background(), testSystemID, "worker-a", now, time.Minute)
	if err != nil || !found || leased.ID != outcome.Run.ID || leased.Summary.LeaseOwner != "worker-a" || leased.Status != RunStatusRunning {
		t.Fatalf("lease = %#v %v %v", leased, found, err)
	}
	_, found, err = repository.LeaseNext(context.Background(), testSystemID, "worker-b", now, time.Minute)
	if err != nil || found {
		t.Fatalf("duplicate lease found=%v err=%v", found, err)
	}
	count, err := repository.RequeueExpired(context.Background(), testSystemID, now.Add(2*time.Minute))
	if err != nil || count != 1 {
		t.Fatalf("RequeueExpired() = %d, %v", count, err)
	}
	leased, found, err = repository.LeaseNext(context.Background(), testSystemID, "worker-b", now.Add(2*time.Minute), time.Minute)
	if err != nil || !found || leased.Summary.LeaseOwner != "worker-b" {
		t.Fatalf("re-lease = %#v %v %v", leased, found, err)
	}
}

func TestQueueCancellationHandlesQueuedAndRunningRuns(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	queued, _ := queue.Enqueue(context.Background(), EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "queued"})
	cancelled, err := queue.Cancel(context.Background(), testSystemID, queued.Run.ID)
	if err != nil || cancelled.Status != RunStatusCancelled {
		t.Fatalf("queued cancel = %#v %v", cancelled, err)
	}
	running, _ := queue.Enqueue(context.Background(), EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "running"})
	_, _, _ = repository.LeaseNext(context.Background(), testSystemID, "worker", fixedNow(), time.Minute)
	cancelled, err = queue.Cancel(context.Background(), testSystemID, running.Run.ID)
	if err != nil || cancelled.Status != RunStatusCancelled {
		t.Fatalf("running cancel = %#v %v", cancelled, err)
	}
}

func TestWorkerTimesOutLeasedExecution(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: time.Now})
	outcome, err := queue.Enqueue(context.Background(), EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "timeout", ExecutionTimeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	executionService := NewService(repository, staticPlan{{ID: "step-1", Position: 1}}, blockingExecutor{}, ServiceOptions{NewID: sequentialIDs(), Now: time.Now})
	worker := NewWorker(repository, executionService, WorkerOptions{SystemID: testSystemID, WorkerID: "worker-a", LeaseDuration: time.Minute, Now: time.Now})
	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("RunOnce() = %v, %v", processed, err)
	}
	run, err := repository.GetRun(context.Background(), testSystemID, outcome.Run.ID)
	if err != nil || run.Status != RunStatusTimedOut || !errors.Is(worker.LastExecutionError(), context.DeadlineExceeded) {
		t.Fatalf("timed out run = %#v err=%v workerErr=%v", run, err, worker.LastExecutionError())
	}
}

func TestRunningCancellationIsNotOverwrittenByWorkerCompletion(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: time.Now})
	outcome, _ := queue.Enqueue(context.Background(), EnqueueCommand{SystemID: testSystemID, ScenarioID: testScenarioID, ScenarioVersionID: testVersionID, EnvironmentID: testEnvID, RequestedBy: testUserID, RetryKey: "cancel-running"})
	executor := controlledExecutor{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(repository, staticPlan{{ID: "step-1", Position: 1}}, executor, ServiceOptions{NewID: sequentialIDs(), Now: time.Now})
	worker := NewWorker(repository, service, WorkerOptions{SystemID: testSystemID, WorkerID: "worker-a", LeaseDuration: time.Minute, HeartbeatInterval: time.Minute, Now: time.Now})
	done := make(chan error, 1)
	go func() { _, err := worker.RunOnce(context.Background()); done <- err }()
	<-executor.started
	if _, err := queue.Cancel(context.Background(), testSystemID, outcome.Run.ID); err != nil {
		t.Fatal(err)
	}
	close(executor.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	run, _ := repository.GetRun(context.Background(), testSystemID, outcome.Run.ID)
	if run.Status != RunStatusCancelled {
		t.Fatalf("worker overwrote cancellation: %#v", run)
	}
}
