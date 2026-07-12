package runrecord

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
	"bizdevops/apps/api/internal/modules/execution"
)

const (
	queryMemberID   = "62000000-0000-4000-8000-000000000001"
	queryOutsiderID = "62000000-0000-4000-8000-000000000002"
)

type queryRoles struct{}

func (queryRoles) RoleForUser(_ context.Context, systemID, userID string) (string, bool, error) {
	if systemID == querySystemID && userID == queryMemberID {
		return string(access.RoleViewer), true, nil
	}
	return "", false, nil
}

func TestRunRecordHTTPMembersReadAndOutsidersSee404(t *testing.T) {
	repository := execution.NewMemoryRepository()
	_ = repository.CreateRun(context.Background(), execution.Run{ID: "run-1", SystemID: querySystemID, Status: execution.RunStatusPassed, Attempts: []execution.StepAttempt{}})
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, NewQueryService(repository), queryRoles{})
	handler := access.DevelopmentIdentity(mux)

	list := runRecordRequest(handler, "/api/v1/systems/"+querySystemID+"/scenario-runs", queryMemberID)
	if list.Code != http.StatusOK || len(runRecordEnvelope(t, list)["data"].([]any)) != 1 {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	detail := runRecordRequest(handler, "/api/v1/systems/"+querySystemID+"/scenario-runs/run-1", queryMemberID)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	outside := runRecordRequest(handler, "/api/v1/systems/"+querySystemID+"/scenario-runs/run-1", queryOutsiderID)
	if outside.Code != http.StatusNotFound || runRecordEnvelope(t, outside)["error"].(map[string]any)["code"] != "system_not_found" {
		t.Fatalf("outside status=%d body=%s", outside.Code, outside.Body.String())
	}
}

func runRecordRequest(handler http.Handler, path, userID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, bytes.NewReader(nil))
	request.Header.Set(access.DevelopmentUserHeader, userID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func runRecordEnvelope(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}
