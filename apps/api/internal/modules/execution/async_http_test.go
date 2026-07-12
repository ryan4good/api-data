package execution

import (
	"net/http"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
)

func TestAsyncHTTPEnqueueIsIdempotentAndSupportsCancellation(t *testing.T) {
	repository := NewMemoryRepository()
	queue := NewQueueService(repository, staticPlan{{ID: "step-1", Position: 1}}, QueueServiceOptions{NewID: sequentialIDs(), Now: fixedNow})
	mux := http.NewServeMux()
	RegisterAsyncRoutes(mux, queue, executionRoles{httpRunnerID: "runner", httpViewerID: "viewer"})
	handler := developmentHandler(mux)
	body := []byte(`{"scenarioId":"` + testScenarioID + `","scenarioVersionId":"` + testVersionID + `","environmentId":"` + testEnvID + `","retryKey":"deploy-42","executionTimeoutMs":30000}`)
	first := executionRequest(handler, http.MethodPost, asyncCollectionPath(), httpRunnerID, body)
	if first.Code != http.StatusAccepted {
		t.Fatalf("enqueue status=%d body=%s", first.Code, first.Body.String())
	}
	firstData := executionEnvelope(t, first)["data"].(map[string]any)
	runID := firstData["run"].(map[string]any)["id"].(string)
	second := executionRequest(handler, http.MethodPost, asyncCollectionPath(), httpRunnerID, body)
	if second.Code != http.StatusOK || executionEnvelope(t, second)["data"].(map[string]any)["existing"] != true {
		t.Fatalf("idempotent enqueue status=%d body=%s", second.Code, second.Body.String())
	}
	forbidden := executionRequest(handler, http.MethodPost, asyncCollectionPath(), httpViewerID, body)
	assertExecutionError(t, forbidden, http.StatusForbidden, "forbidden")
	cancelled := executionRequest(handler, http.MethodPost, executionCollectionPath()+"/"+runID+"/cancel", httpRunnerID, nil)
	if cancelled.Code != http.StatusOK || executionEnvelope(t, cancelled)["data"].(map[string]any)["status"] != RunStatusCancelled {
		t.Fatalf("cancel status=%d body=%s", cancelled.Code, cancelled.Body.String())
	}
}

func asyncCollectionPath() string { return "/api/v1/systems/" + testSystemID + "/scenario-run-jobs" }

func developmentHandler(handler http.Handler) http.Handler {
	return access.DevelopmentIdentity(handler)
}
