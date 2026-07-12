package importer

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
	httpOwnerID      = "41000000-0000-4000-8000-000000000001"
	httpMaintainerID = "41000000-0000-4000-8000-000000000002"
	httpReviewerID   = "41000000-0000-4000-8000-000000000003"
	httpRunnerID     = "41000000-0000-4000-8000-000000000004"
	httpViewerID     = "41000000-0000-4000-8000-000000000005"
	httpOutsiderID   = "41000000-0000-4000-8000-000000000006"
)

type staticRoleReader map[string]access.Role

func (reader staticRoleReader) RoleForUser(_ context.Context, systemID, userID string) (string, bool, error) {
	if systemID != testSystemID {
		return "", false, nil
	}
	role, found := reader[userID]
	return string(role), found, nil
}

func newImporterHTTP(t *testing.T) (http.Handler, *Service) {
	t.Helper()
	service := newTestService(NewMemoryRepository())
	roles := staticRoleReader{
		httpOwnerID: access.RoleOwner, httpMaintainerID: access.RoleMaintainer, httpReviewerID: access.RoleReviewer,
		httpRunnerID: access.RoleRunner, httpViewerID: access.RoleViewer,
	}
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, roles)
	return access.DevelopmentIdentity(mux), service
}

func importerRequest(handler http.Handler, method, path, userID string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if userID != "" {
		req.Header.Set(access.DevelopmentUserHeader, userID)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeImporterEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid response JSON: %v; body=%s", err, recorder.Body.String())
	}
	return envelope
}

func TestImporterHTTPUploadListAndDetailAreSystemScoped(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	upload := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOwnerID, uploadBody(postmanWithoutScript()))
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
	}
	data := decodeImporterEnvelope(t, upload)["data"].(map[string]any)
	importID := data["id"].(string)
	if _, leaked := data["rawDocument"]; leaked {
		t.Fatalf("response leaked raw upload: %#v", data)
	}

	list := importerRequest(handler, http.MethodGet, importCollectionPath(), httpViewerID, nil)
	if list.Code != http.StatusOK || len(decodeImporterEnvelope(t, list)["data"].([]any)) != 1 {
		t.Fatalf("member list status=%d body=%s", list.Code, list.Body.String())
	}
	detail := importerRequest(handler, http.MethodGet, importCollectionPath()+"/"+importID, httpRunnerID, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("member detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	outside := importerRequest(handler, http.MethodGet, importCollectionPath()+"/"+importID, httpOutsiderID, nil)
	assertHTTPError(t, outside, http.StatusNotFound, "system_not_found")
}

func TestImporterHTTPRoleMatrixForWrites(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	for _, userID := range []string{httpOwnerID, httpMaintainerID} {
		response := importerRequest(handler, http.MethodPost, importCollectionPath(), userID, uploadBody(postmanWithoutScript()))
		if response.Code != http.StatusCreated {
			t.Fatalf("upload role %s status=%d body=%s", userID, response.Code, response.Body.String())
		}
	}
	for _, userID := range []string{httpReviewerID, httpRunnerID, httpViewerID} {
		response := importerRequest(handler, http.MethodPost, importCollectionPath(), userID, uploadBody(postmanWithoutScript()))
		assertHTTPError(t, response, http.StatusForbidden, "forbidden")
	}
	outside := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOutsiderID, uploadBody(postmanWithoutScript()))
	assertHTTPError(t, outside, http.StatusNotFound, "system_not_found")
}

func TestImporterHTTPConfirmAndApplyEnforceSeparateRoles(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	upload := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOwnerID, uploadBody(postmanWithScript()))
	importID := decodeImporterEnvelope(t, upload)["data"].(map[string]any)["id"].(string)
	confirmPath := importCollectionPath() + "/" + importID + "/confirm-scripts"
	applyPath := importCollectionPath() + "/" + importID + "/apply"

	assertHTTPError(t, importerRequest(handler, http.MethodPost, confirmPath, httpRunnerID, nil), http.StatusForbidden, "forbidden")
	confirmed := importerRequest(handler, http.MethodPost, confirmPath, httpReviewerID, nil)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("reviewer confirm status=%d body=%s", confirmed.Code, confirmed.Body.String())
	}
	assertHTTPError(t, importerRequest(handler, http.MethodPost, applyPath, httpReviewerID, nil), http.StatusForbidden, "forbidden")
	applied := importerRequest(handler, http.MethodPost, applyPath, httpMaintainerID, nil)
	if applied.Code != http.StatusOK || decodeImporterEnvelope(t, applied)["data"].(map[string]any)["status"] != StatusApplied {
		t.Fatalf("maintainer apply status=%d body=%s", applied.Code, applied.Body.String())
	}
}

func TestImporterHTTPMapsReviewGateToStableConflict(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	upload := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOwnerID, uploadBody(postmanWithScript()))
	importID := decodeImporterEnvelope(t, upload)["data"].(map[string]any)["id"].(string)
	response := importerRequest(handler, http.MethodPost, importCollectionPath()+"/"+importID+"/apply", httpOwnerID, nil)
	assertHTTPError(t, response, http.StatusConflict, "scripts_unreviewed")
}

func TestImporterHTTPMapsFailedConversionToStableConflict(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	invalidBundle := []byte(`{"schemaVersion":"1.0","system":{"key":"oms","name":"OMS"},"scenario":{"key":"bad","name":"Bad","version":1,"variables":[],"steps":[]}}`)
	upload := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOwnerID, uploadBody(invalidBundle))
	if upload.Code != http.StatusCreated {
		t.Fatalf("failed draft status=%d body=%s", upload.Code, upload.Body.String())
	}
	data := decodeImporterEnvelope(t, upload)["data"].(map[string]any)
	if data["status"] != StatusFailed {
		t.Fatalf("failed draft body=%s", upload.Body.String())
	}
	response := importerRequest(handler, http.MethodPost, importCollectionPath()+"/"+data["id"].(string)+"/apply", httpOwnerID, nil)
	assertHTTPError(t, response, http.StatusConflict, "import_has_errors")
}

func TestImporterHTTPRejectsNonStrictAndOversizedUploadWithoutLeakingDocument(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	secret := "top-secret-marker"
	cases := [][]byte{
		[]byte(`{"fileName":"a.json","document":{"password":"` + secret + `"},"unknown":true}`),
		[]byte(`{"fileName":"a.json","document":{}} {}`),
		[]byte(`{"fileName":"a.json","document":{"password":"` + secret + `"}}`),
		[]byte(`{"fileName":"a.json","document":{"padding":"` + strings.Repeat("x", (1<<20)+1) + `"}}`),
	}
	for index, body := range cases {
		response := importerRequest(handler, http.MethodPost, importCollectionPath(), httpOwnerID, body)
		assertHTTPError(t, response, http.StatusBadRequest, "invalid_request")
		if strings.Contains(response.Body.String(), secret) {
			t.Fatalf("case %d leaked sensitive source: %s", index, response.Body.String())
		}
	}
}

func TestImporterHTTPReturns404ForMissingImportWithoutScopeDisclosure(t *testing.T) {
	handler, _ := newImporterHTTP(t)
	response := importerRequest(handler, http.MethodGet, importCollectionPath()+"/missing", httpViewerID, nil)
	assertHTTPError(t, response, http.StatusNotFound, "scenario_import_not_found")
}

func uploadBody(document []byte) []byte {
	return append(append([]byte(`{"fileName":"orders.json","document":`), document...), '}')
}

func importCollectionPath() string {
	return "/api/v1/systems/" + testSystemID + "/scenario-imports"
}

func assertHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	errorValue := decodeImporterEnvelope(t, response)["error"].(map[string]any)
	if errorValue["code"] != code {
		t.Fatalf("error code=%v, want %s; body=%s", errorValue["code"], code, response.Body.String())
	}
}
