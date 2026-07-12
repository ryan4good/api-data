package runrecord

import (
	"context"
	"errors"
	"testing"

	"bizdevops/apps/api/internal/modules/execution"
)

const querySystemID = "61000000-0000-4000-8000-000000000001"

func TestQueryServiceKeepsSystemScopeForListAndDetail(t *testing.T) {
	repository := execution.NewMemoryRepository()
	run := execution.Run{ID: "run-1", SystemID: querySystemID, Status: execution.RunStatusPassed, Attempts: []execution.StepAttempt{}}
	if err := repository.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	service := NewQueryService(repository)
	items, err := service.List(context.Background(), querySystemID)
	if err != nil || len(items) != 1 {
		t.Fatalf("List() = %#v, %v", items, err)
	}
	_, err = service.Get(context.Background(), "61000000-0000-4000-8000-000000000002", "run-1")
	if !errors.Is(err, execution.ErrRunNotFound) {
		t.Fatalf("cross-system Get() error = %v", err)
	}
}
