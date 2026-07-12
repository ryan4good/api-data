package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
)

type discoveryRoles struct {
	role  string
	found bool
}

func (s discoveryRoles) RoleForUser(context.Context, string, string) (string, bool, error) {
	return s.role, s.found, nil
}

func discoveryHandler(service DiscoveryHTTPService, roles access.RoleReader) http.Handler {
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, roles)
	return access.DevelopmentIdentity(mux)
}

func discoveryHTTP(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(access.DevelopmentUserHeader, testUser)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func discoveryError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.Error == nil {
		return ""
	}
	return v.Error.Code
}

func TestDiscoveryHTTPRBACAndCodeOnlyCreate(t *testing.T) {
	service := NewService(NewMemoryRepository())
	body := `{"type":"code","name":"订单发现","operations":[{"id":"op-create","systemId":"` + testSystemA + `","method":"POST","path":"/orders","operationKey":"POST /orders"}]}`
	for _, role := range []string{"owner", "maintainer"} {
		rec := discoveryHTTP(t, discoveryHandler(service, discoveryRoles{role: role, found: true}), http.MethodPost, "/api/v1/systems/"+testSystemA+"/discoveries", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("role=%s status=%d body=%s", role, rec.Code, rec.Body.String())
		}
	}
	rec := discoveryHTTP(t, discoveryHandler(service, discoveryRoles{role: "viewer", found: true}), http.MethodPost, "/api/v1/systems/"+testSystemA+"/discoveries", body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer status=%d", rec.Code)
	}
	rec = discoveryHTTP(t, discoveryHandler(service, discoveryRoles{found: false}), http.MethodGet, "/api/v1/systems/"+testSystemA+"/discoveries", "")
	if rec.Code != http.StatusNotFound || discoveryError(t, rec) != "system_not_found" {
		t.Fatalf("nonmember status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHTTPReviewerCanAcceptAndReject(t *testing.T) {
	service := NewService(NewMemoryRepository())
	discovery, candidates, err := service.Discover(context.Background(), DiscoverInput{SystemID: testSystemA, Type: TypeCode, Name: "Orders", RequestedBy: testUser, Operations: orderOperations(testSystemA)})
	if err != nil {
		t.Fatal(err)
	}
	h := discoveryHandler(service, discoveryRoles{role: "reviewer", found: true})
	path := "/api/v1/systems/" + testSystemA + "/discoveries/" + discovery.ID + "/candidates/" + candidates[0].ID + "/accept"
	if rec := discoveryHTTP(t, h, http.MethodPost, path, `{"note":"verified"}`); rec.Code != http.StatusOK {
		t.Fatalf("accept status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHTTPStrictInputAndReviewRole(t *testing.T) {
	service := NewService(NewMemoryRepository())
	owner := discoveryHandler(service, discoveryRoles{role: "owner", found: true})
	for _, body := range []string{`{"type":"code","name":"x","unknown":true}`, `{"type":"bad","name":"x"}`, `{"type":"prompt","name":"x","prompt":" "}`} {
		rec := discoveryHTTP(t, owner, http.MethodPost, "/api/v1/systems/"+testSystemA+"/discoveries", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, rec.Code, rec.Body.String())
		}
	}
	viewer := discoveryHandler(service, discoveryRoles{role: "viewer", found: true})
	rec := discoveryHTTP(t, viewer, http.MethodPost, "/api/v1/systems/"+testSystemA+"/discoveries/"+testDiscovery+"/candidates/"+testCandidate+"/reject", `{}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHTTPRejectsInvalidUUIDAndBodiesOverOneMiB(t *testing.T) {
	service := NewService(NewMemoryRepository())
	h := discoveryHandler(service, discoveryRoles{role: "owner", found: true})
	if rec := discoveryHTTP(t, h, http.MethodGet, "/api/v1/systems/not-a-uuid/discoveries", ""); rec.Code != http.StatusBadRequest || discoveryError(t, rec) != "invalid_request" {
		t.Fatalf("invalid UUID status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := `{"type":"prompt","name":"large","prompt":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	if rec := discoveryHTTP(t, h, http.MethodPost, "/api/v1/systems/"+testSystemA+"/discoveries", body); rec.Code != http.StatusBadRequest || discoveryError(t, rec) != "invalid_request" {
		t.Fatalf("oversized status=%d body=%s", rec.Code, rec.Body.String())
	}
}
