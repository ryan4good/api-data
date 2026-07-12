package execution

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLStepProviderLoadsEnabledVersionStepsWithSystemScope(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	provider := NewMySQLStepProvider(db)
	mock.ExpectQuery(regexp.QuoteMeta(selectScenarioStepsSQL)).
		WithArgs(testSystemID, testVersionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "position", "name", "step_type", "request_config", "extractors", "assertions", "timeout_ms", "failure_policy"}).
			AddRow("step-1", 1, "Create order", "http", []byte(`{"method":"POST","url":"/orders"}`), []byte(`[]`), []byte(`[]`), 30000, "stop").
			AddRow("step-2", 2, "Check order", "http", nil, nil, nil, 30000, "stop"))

	steps, err := provider.Steps(context.Background(), testSystemID, testVersionID)
	if err != nil {
		t.Fatalf("Steps() error = %v", err)
	}
	if len(steps) != 2 || steps[0].ID != "step-1" || steps[0].Config["method"] != "POST" || steps[1].Position != 2 {
		t.Fatalf("steps = %#v", steps)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledExecutorFailsExplicitlyWithoutNetwork(t *testing.T) {
	result, err := (DisabledExecutor{}).Execute(context.Background(), Step{ID: "step-1"}, nil)
	if !errors.Is(err, ErrExecutorNotConfigured) || result.RequestSnapshot["executor"] != "disabled" {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	repository := NewMemoryRepository()
	service := NewService(repository, staticPlan{{ID: "step-1", Position: 1}}, DisabledExecutor{}, ServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	run, serviceErr := service.Execute(context.Background(), baseCommand())
	if !errors.Is(serviceErr, ErrExecutorNotConfigured) {
		t.Fatalf("service error = %v, want ErrExecutorNotConfigured", serviceErr)
	}
	if run.Status != RunStatusFailed || run.Attempts[0].ErrorMessage != ErrExecutorNotConfigured.Error() {
		t.Fatalf("disabled executor did not fail safely: %#v", run)
	}
}

func TestExecutionHTTPMapsDisabledExecutorToServiceUnavailable(t *testing.T) {
	repository := NewMemoryRepository()
	service := NewService(repository, staticPlan{{ID: "step-1", Position: 1}}, DisabledExecutor{}, ServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, executionRoles{httpRunnerID: access.RoleRunner})
	handler := access.DevelopmentIdentity(mux)
	body := []byte(`{"scenarioId":"` + testScenarioID + `","scenarioVersionId":"` + testVersionID + `","environmentId":"` + testEnvID + `"}`)
	response := executionRequest(handler, http.MethodPost, executionCollectionPath(), httpRunnerID, body)
	assertExecutionError(t, response, http.StatusServiceUnavailable, "executor_not_configured")
	items, err := repository.ListRuns(context.Background(), testSystemID)
	if err != nil || len(items) != 1 || items[0].Status != RunStatusFailed {
		t.Fatalf("failed audit run missing: %#v, %v", items, err)
	}
}
