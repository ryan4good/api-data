package environment

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
func h(repo Repository, rr access.RoleReader) http.Handler {
	m := http.NewServeMux()
	RegisterSystemRoutes(m, repo, rr)
	return access.DevelopmentIdentity(m)
}
func req(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set(access.DevelopmentUserHeader, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestEnvironmentHTTPWritesAndViewerRedaction(t *testing.T) {
	r, _ := fixture()
	path := "/api/v1/systems/" + sysA + "/environments/" + envID + "/secret-references"
	owner := h(r, roles{role: "owner", found: true})
	body := `{"variableKey":"password","secretRef":"vault://test/password"}`
	if w := req(t, owner, http.MethodPost, path, body); w.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", w.Code, w.Body.String())
	}
	viewer := h(r, roles{role: "viewer", found: true})
	w := req(t, viewer, http.MethodGet, path, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "vault://") {
		t.Fatalf("viewer leaked reference: %s", w.Body.String())
	}
	if w := req(t, viewer, http.MethodPost, path, body); w.Code != http.StatusForbidden {
		t.Fatalf("viewer write=%d", w.Code)
	}
	runner := h(r, roles{role: "runner", found: true})
	if w := req(t, runner, http.MethodGet, path, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "vault://") {
		t.Fatalf("runner=%d body=%s", w.Code, w.Body.String())
	}
}
func TestEnvironmentHTTPNonMember404AndStrictJSON(t *testing.T) {
	r, _ := fixture()
	path := "/api/v1/systems/" + sysA + "/environments"
	if w := req(t, h(r, roles{found: false}), http.MethodGet, path, ""); w.Code != http.StatusNotFound {
		t.Fatalf("nonmember=%d", w.Code)
	}
	w := req(t, h(r, roles{role: "owner", found: true}), http.MethodPost, path, `{"key":"dev","name":"Dev","unknown":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("strict=%d body=%s", w.Code, w.Body.String())
	}
	var v any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
}
