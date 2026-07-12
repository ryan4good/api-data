package scenario

import (
	"bizdevops/apps/api/internal/modules/access"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roles struct {
	role  string
	found bool
}

func (r roles) RoleForUser(context.Context, string, string) (string, bool, error) {
	return r.role, r.found, nil
}
func handler(service ScenarioHTTPService, rr access.RoleReader) http.Handler {
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, service, rr)
	return access.DevelopmentIdentity(mux)
}
func request(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set(access.DevelopmentUserHeader, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.Error == nil {
		return ""
	}
	return v.Error.Code
}
func TestScenarioHTTPPromotionRBACAndReads(t *testing.T) {
	service := NewService(NewMemoryRepository([]PromotionCandidate{acceptedCandidate()}))
	path := "/api/v1/systems/" + systemA + "/discoveries/" + discoveryID + "/candidates/" + candidateID + "/promote"
	for _, role := range []string{"owner", "maintainer"} {
		w := request(t, handler(service, roles{role: role, found: true}), http.MethodPost, path, `{}`)
		if w.Code != http.StatusOK && w.Code != http.StatusCreated {
			t.Fatalf("role=%s status=%d body=%s", role, w.Code, w.Body.String())
		}
	}
	viewer := handler(service, roles{role: "viewer", found: true})
	if w := request(t, viewer, http.MethodGet, "/api/v1/systems/"+systemA+"/scenarios", ""); w.Code != http.StatusOK {
		t.Fatalf("list status=%d", w.Code)
	}
	if w := request(t, viewer, http.MethodPost, path, `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("viewer promote=%d", w.Code)
	}
	if w := request(t, handler(service, roles{found: false}), http.MethodGet, "/api/v1/systems/"+systemA+"/scenarios", ""); w.Code != http.StatusNotFound || errCode(t, w) != "system_not_found" {
		t.Fatalf("nonmember=%d body=%s", w.Code, w.Body.String())
	}
}
func TestScenarioHTTPStrictUUIDAndJSON(t *testing.T) {
	service := NewService(NewMemoryRepository([]PromotionCandidate{acceptedCandidate()}))
	h := handler(service, roles{role: "owner", found: true})
	if w := request(t, h, http.MethodGet, "/api/v1/systems/not-uuid/scenarios", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("uuid=%d", w.Code)
	}
	path := "/api/v1/systems/" + systemA + "/discoveries/" + discoveryID + "/candidates/" + candidateID + "/promote"
	if w := request(t, h, http.MethodPost, path, `{"unknown":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("json=%d body=%s", w.Code, w.Body.String())
	}
}
