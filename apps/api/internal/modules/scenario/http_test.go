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

type conflictUpdateService struct{ *Service }

func (s conflictUpdateService) Update(context.Context, string, string, string, UpdateRequest) (Detail, error) {
	return Detail{}, ErrRevisionConflict
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

func TestScenarioHTTPUpdateRBACValidationAndNotFound(t *testing.T) {
	repo := NewMemoryRepository([]PromotionCandidate{acceptedCandidate()})
	service := NewService(repo)
	created, err := service.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/systems/" + systemA + "/scenarios/" + created.Scenario.ID
	body := `{"name":"人工场景","description":"修订","status":"active","steps":[{"key":"a","name":"A","type":"http","operationId":"55555555-5555-4555-8555-555555555555","dependsOn":[],"requestConfig":{"method":"GET","path":"/a"}}]}`
	for _, role := range []string{"owner", "maintainer"} {
		w := request(t, handler(service, roles{role: role, found: true}), http.MethodPut, path, body)
		if w.Code != http.StatusOK {
			t.Fatalf("role=%s status=%d body=%s", role, w.Code, w.Body.String())
		}
		var response struct {
			Data struct {
				Steps []struct {
					RequestConfig json.RawMessage `json:"requestConfig"`
				} `json:"steps"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Data.Steps) != 1 || string(response.Data.Steps[0].RequestConfig) != `{"method":"GET","path":"/a"}` {
			t.Fatalf("requestConfig must round-trip as object: %s", w.Body.String())
		}
	}
	if w := request(t, handler(service, roles{role: "viewer", found: true}), http.MethodPut, path, body); w.Code != http.StatusForbidden {
		t.Fatalf("viewer=%d body=%s", w.Code, w.Body.String())
	}
	owner := handler(service, roles{role: "owner", found: true})
	if w := request(t, owner, http.MethodPut, path, `{"name":"x","status":"bad","steps":[]}`); w.Code != http.StatusBadRequest || errCode(t, w) != "invalid_request" {
		t.Fatalf("validation=%d body=%s", w.Code, w.Body.String())
	}
	missing := "/api/v1/systems/" + systemA + "/scenarios/77777777-7777-4777-8777-777777777777"
	if w := request(t, owner, http.MethodPut, missing, body); w.Code != http.StatusNotFound || errCode(t, w) != "scenario_not_found" {
		t.Fatalf("missing=%d body=%s", w.Code, w.Body.String())
	}
	conflict := handler(conflictUpdateService{Service: service}, roles{role: "owner", found: true})
	if w := request(t, conflict, http.MethodPut, path, body); w.Code != http.StatusConflict || errCode(t, w) != "revision_conflict" {
		t.Fatalf("conflict=%d body=%s", w.Code, w.Body.String())
	}
}
