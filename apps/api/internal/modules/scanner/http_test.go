package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
)

const (
	httpSystemID = "11111111-1111-4111-8111-111111111111"
	httpSourceID = "22222222-2222-4222-8222-222222222222"
	httpScanID   = "33333333-3333-4333-8333-333333333333"
	httpUserID   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type stubScannerRoles struct {
	role  string
	found bool
	err   error
}

func (s stubScannerRoles) RoleForUser(context.Context, string, string) (string, bool, error) {
	return s.role, s.found, s.err
}

type stubScannerService struct {
	created    CreateScanInput
	runSystem  string
	runScan    string
	runRoot    string
	scans      []ScanRun
	operations []APIOperation
	err        error
}

func (s *stubScannerService) Create(_ context.Context, input CreateScanInput) (ScanRun, error) {
	s.created = input
	return ScanRun{ID: httpScanID, SystemID: input.SystemID, CodeSourceID: input.CodeSourceID, Status: StatusQueued}, s.err
}

func (s *stubScannerService) Run(_ context.Context, systemID, scanID, root string) error {
	s.runSystem, s.runScan, s.runRoot = systemID, scanID, root
	return s.err
}

func (s *stubScannerService) ListScans(context.Context, string) ([]ScanRun, error) {
	return s.scans, s.err
}

func (s *stubScannerService) ListOperations(context.Context, string) ([]APIOperation, error) {
	return s.operations, s.err
}

func newScannerHTTPHandler(service ScannerHTTPService, roles access.RoleReader) http.Handler {
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, roles)
	return access.DevelopmentIdentity(mux)
}

func scannerRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set(access.DevelopmentUserHeader, httpUserID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func responseErrorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil {
		return ""
	}
	return envelope.Error.Code
}

func TestScannerHTTPMembersCanListScansAndOperations(t *testing.T) {
	service := &stubScannerService{
		scans:      []ScanRun{{ID: httpScanID, SystemID: httpSystemID}},
		operations: []APIOperation{{ID: "op-1", SystemID: httpSystemID, Method: "GET", Path: "/orders"}},
	}
	handler := newScannerHTTPHandler(service, stubScannerRoles{role: "viewer", found: true})
	for _, path := range []string{
		"/api/v1/systems/" + httpSystemID + "/scans",
		"/api/v1/systems/" + httpSystemID + "/api-operations",
	} {
		if recorder := scannerRequest(t, handler, http.MethodGet, path, ""); recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestScannerHTTPCreateMapsStrictBodyAndActor(t *testing.T) {
	for _, role := range []string{"owner", "maintainer"} {
		t.Run(role, func(t *testing.T) {
			service := &stubScannerService{}
			handler := newScannerHTTPHandler(service, stubScannerRoles{role: role, found: true})
			body := `{"codeSourceId":"` + httpSourceID + `","sourceRef":"main","sourceCommit":"abc123","language":"go","framework":"gin"}`
			recorder := scannerRequest(t, handler, http.MethodPost, "/api/v1/systems/"+httpSystemID+"/scans", body)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if service.created.SystemID != httpSystemID || service.created.CodeSourceID != httpSourceID || service.created.RequestedBy != httpUserID || service.created.Language != "go" || service.created.Framework != "gin" {
				t.Fatalf("created input=%#v", service.created)
			}
		})
	}
}

func TestScannerHTTPRunPassesScopedIDsAndRoot(t *testing.T) {
	service := &stubScannerService{}
	handler := newScannerHTTPHandler(service, stubScannerRoles{role: "maintainer", found: true})
	recorder := scannerRequest(t, handler, http.MethodPost,
		"/api/v1/systems/"+httpSystemID+"/scans/"+httpScanID+"/run", `{"repositoryRoot":"D:/work/service"}`)
	if recorder.Code != http.StatusOK || service.runSystem != httpSystemID || service.runScan != httpScanID || service.runRoot != "D:/work/service" {
		t.Fatalf("status=%d run=(%q,%q,%q) body=%s", recorder.Code, service.runSystem, service.runScan, service.runRoot, recorder.Body.String())
	}
}

func TestScannerHTTPHidesSystemsFromNonMembers(t *testing.T) {
	handler := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{found: false})
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/systems/" + httpSystemID + "/scans", ""},
		{http.MethodPost, "/api/v1/systems/" + httpSystemID + "/scans", `{"codeSourceId":"` + httpSourceID + `"}`},
		{http.MethodPost, "/api/v1/systems/" + httpSystemID + "/scans/" + httpScanID + "/run", `{"repositoryRoot":"."}`},
		{http.MethodGet, "/api/v1/systems/" + httpSystemID + "/api-operations", ""},
	} {
		recorder := scannerRequest(t, handler, request.method, request.path, request.body)
		if recorder.Code != http.StatusNotFound || responseErrorCode(t, recorder) != "system_not_found" {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestScannerHTTPReadOnlyMembersCannotCreateOrRun(t *testing.T) {
	for _, role := range []string{"reviewer", "runner", "viewer"} {
		handler := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{role: role, found: true})
		for _, request := range []struct{ path, body string }{
			{"/api/v1/systems/" + httpSystemID + "/scans", `{"codeSourceId":"` + httpSourceID + `"}`},
			{"/api/v1/systems/" + httpSystemID + "/scans/" + httpScanID + "/run", `{"repositoryRoot":"."}`},
		} {
			recorder := scannerRequest(t, handler, http.MethodPost, request.path, request.body)
			if recorder.Code != http.StatusForbidden || responseErrorCode(t, recorder) != "forbidden" {
				t.Fatalf("role=%s status=%d body=%s", role, recorder.Code, recorder.Body.String())
			}
		}
	}
}

func TestScannerHTTPRejectsInvalidUUIDAndStrictJSON(t *testing.T) {
	handler := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{role: "owner", found: true})
	tests := []struct{ path, body string }{
		{"/api/v1/systems/not-a-uuid/scans", `{"codeSourceId":"` + httpSourceID + `"}`},
		{"/api/v1/systems/" + httpSystemID + "/scans", `{"codeSourceId":"bad"}`},
		{"/api/v1/systems/" + httpSystemID + "/scans", `{"codeSourceId":"` + httpSourceID + `","unknown":true}`},
		{"/api/v1/systems/" + httpSystemID + "/scans", `{"codeSourceId":"` + httpSourceID + `"}{}`},
		{"/api/v1/systems/" + httpSystemID + "/scans/not-a-uuid/run", `{"repositoryRoot":"."}`},
		{"/api/v1/systems/" + httpSystemID + "/scans/" + httpScanID + "/run", `{"repositoryRoot":" "}`},
	}
	for _, test := range tests {
		recorder := scannerRequest(t, handler, http.MethodPost, test.path, test.body)
		if recorder.Code != http.StatusBadRequest || responseErrorCode(t, recorder) != "invalid_request" {
			t.Fatalf("POST %s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestScannerHTTPRejectsBodiesOverOneMiB(t *testing.T) {
	handler := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{role: "owner", found: true})
	body := `{"codeSourceId":"` + httpSourceID + `","sourceRef":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	recorder := scannerRequest(t, handler, http.MethodPost, "/api/v1/systems/"+httpSystemID+"/scans", body)
	if recorder.Code != http.StatusBadRequest || responseErrorCode(t, recorder) != "invalid_request" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestScannerHTTPMapsRoleReaderAndServiceErrors(t *testing.T) {
	roleFailure := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{err: errors.New("db down")})
	if recorder := scannerRequest(t, roleFailure, http.MethodGet, "/api/v1/systems/"+httpSystemID+"/scans", ""); recorder.Code != http.StatusInternalServerError {
		t.Fatalf("role failure status=%d", recorder.Code)
	}
	serviceFailure := newScannerHTTPHandler(&stubScannerService{err: ErrScanNotFound}, stubScannerRoles{role: "owner", found: true})
	if recorder := scannerRequest(t, serviceFailure, http.MethodPost, "/api/v1/systems/"+httpSystemID+"/scans/"+httpScanID+"/run", `{"repositoryRoot":"."}`); recorder.Code != http.StatusNotFound {
		t.Fatalf("service not found status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestScannerHTTPMethodNotAllowedUsesAPIEnvelope(t *testing.T) {
	handler := newScannerHTTPHandler(&stubScannerService{}, stubScannerRoles{role: "viewer", found: true})
	recorder := scannerRequest(t, handler, http.MethodDelete, "/api/v1/systems/"+httpSystemID+"/scans", "")
	if recorder.Code != http.StatusMethodNotAllowed || responseErrorCode(t, recorder) != "method_not_allowed" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
