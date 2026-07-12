package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
)

const (
	httpRunnerID   = "56000000-0000-4000-8000-000000000001"
	httpViewerID   = "56000000-0000-4000-8000-000000000002"
	httpOwnerID    = "56000000-0000-4000-8000-000000000003"
	httpOutsiderID = "56000000-0000-4000-8000-000000000004"
)

type executionRoles map[string]access.Role

func (roles executionRoles) RoleForUser(_ context.Context, systemID, userID string) (string, bool, error) {
	if systemID != testSystemID {
		return "", false, nil
	}
	role, found := roles[userID]
	return string(role), found, nil
}

func newExecutionHTTP() http.Handler {
	repository := NewMemoryRepository()
	service := newExecutionService(repository, &fakeStepExecutor{results: successResults()})
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, executionRoles{httpRunnerID: access.RoleRunner, httpViewerID: access.RoleViewer, httpOwnerID: access.RoleOwner})
	return access.DevelopmentIdentity(mux)
}

func TestExecutionHTTPRunnerCanRunToStepAndRetry(t *testing.T) {
	handler := newExecutionHTTP()
	body := []byte(`{"scenarioId":"` + testScenarioID + `","scenarioVersionId":"` + testVersionID + `","environmentId":"` + testEnvID + `","stopAfterStepId":"step-2"}`)
	response := executionRequest(handler, http.MethodPost, executionCollectionPath(), httpRunnerID, body)
	if response.Code != http.StatusCreated {
		t.Fatalf("execute status=%d body=%s", response.Code, response.Body.String())
	}
	run := executionEnvelope(t, response)["data"].(map[string]any)
	if run["outcome"] != OutcomePartial {
		t.Fatalf("run=%#v", run)
	}
	runID := run["id"].(string)
	retry := executionRequest(handler, http.MethodPost, executionCollectionPath()+"/"+runID+"/steps/step-2/retry", httpOwnerID, nil)
	if retry.Code != http.StatusOK || executionEnvelope(t, retry)["data"].(map[string]any)["attempt"].(map[string]any)["attemptNo"] != float64(2) {
		t.Fatalf("retry status=%d body=%s", retry.Code, retry.Body.String())
	}
}

func TestExecutionHTTPWriteRoleAndMembershipMatrix(t *testing.T) {
	handler := newExecutionHTTP()
	body := []byte(`{"scenarioId":"` + testScenarioID + `","scenarioVersionId":"` + testVersionID + `","environmentId":"` + testEnvID + `"}`)
	assertExecutionError(t, executionRequest(handler, http.MethodPost, executionCollectionPath(), httpViewerID, body), http.StatusForbidden, "forbidden")
	assertExecutionError(t, executionRequest(handler, http.MethodPost, executionCollectionPath(), httpOutsiderID, body), http.StatusNotFound, "system_not_found")
}

func TestExecutionHTTPRejectsStrictOrOversizedJSON(t *testing.T) {
	handler := newExecutionHTTP()
	bodies := [][]byte{
		[]byte(`{"scenarioId":"x","scenarioVersionId":"x","environmentId":"x","unknown":true}`),
		[]byte(`{} {}`),
		[]byte(`{"scenarioId":"` + strings.Repeat("x", (1<<20)+1) + `"}`),
	}
	for _, body := range bodies {
		assertExecutionError(t, executionRequest(handler, http.MethodPost, executionCollectionPath(), httpRunnerID, body), http.StatusBadRequest, "invalid_request")
	}
}

func executionRequest(handler http.Handler, method, path, userID string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set(access.DevelopmentUserHeader, userID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func executionEnvelope(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, response.Body.String())
	}
	return envelope
}

func executionCollectionPath() string { return "/api/v1/systems/" + testSystemID + "/scenario-runs" }

func assertExecutionError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
	}
	got := executionEnvelope(t, response)["error"].(map[string]any)["code"]
	if got != code {
		t.Fatalf("code=%v want=%s", got, code)
	}
}
