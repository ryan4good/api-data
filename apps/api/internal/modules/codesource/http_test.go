package codesource

import (
	"bizdevops/apps/api/internal/modules/access"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testSystem      = "11111111-1111-4111-8111-111111111111"
	testOtherSystem = "22222222-2222-4222-8222-222222222222"
	testSource      = "33333333-3333-4333-8333-333333333333"
	testUser        = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type testRoles struct {
	role  string
	found bool
}

func (r testRoles) RoleForUser(context.Context, string, string) (string, bool, error) {
	return r.role, r.found, nil
}

func testHandler(repo Repository, roles access.RoleReader) http.Handler {
	mux := http.NewServeMux()
	RegisterSystemRoutes(mux, repo, roles)
	return access.DevelopmentIdentity(mux)
}
func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(access.DevelopmentUserHeader, testUser)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestHTTPCreateListUpdateAndRBAC(t *testing.T) {
	repo := NewMemoryRepository()
	owner := testHandler(repo, testRoles{role: string(access.RoleOwner), found: true})
	path := "/api/v1/systems/" + testSystem + "/code-sources"
	body := `{"name":"Orders API","sourceType":"git","repositoryUrl":"https://git.example.test/orders.git","defaultRef":"main","includePaths":["src"],"excludePaths":["vendor"],"credentialRef":"vault://git/orders","status":"active"}`
	w := perform(owner, http.MethodPost, path, body)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"credentialRef":"vault://git/orders"`) {
		t.Fatalf("create=%d %s", w.Code, w.Body.String())
	}
	if w := perform(owner, http.MethodGet, path, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"Orders API"`) {
		t.Fatalf("list=%d %s", w.Code, w.Body.String())
	}
	var created []CodeSource
	created, _ = repo.List(context.Background(), testSystem)
	update := `{"name":"Orders Local","sourceType":"local","localPath":"/srv/orders","includePaths":[],"excludePaths":[],"status":"disabled"}`
	if w := perform(owner, http.MethodPut, path+"/"+created[0].ID, update); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"sourceType":"local"`) {
		t.Fatalf("update=%d %s", w.Code, w.Body.String())
	}

	viewer := testHandler(repo, testRoles{role: string(access.RoleViewer), found: true})
	if w := perform(viewer, http.MethodGet, path, ""); w.Code != http.StatusOK {
		t.Fatalf("viewer list=%d", w.Code)
	}
	if w := perform(viewer, http.MethodPost, path, body); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create=%d", w.Code)
	}
	if w := perform(testHandler(repo, testRoles{found: false}), http.MethodGet, path, ""); w.Code != http.StatusNotFound {
		t.Fatalf("nonmember=%d", w.Code)
	}
}

func TestHTTPRejectsInvalidPayloadsAndConflicts(t *testing.T) {
	repo := NewMemoryRepository()
	h := testHandler(repo, testRoles{role: string(access.RoleMaintainer), found: true})
	path := "/api/v1/systems/" + testSystem + "/code-sources"
	cases := []string{
		`{"name":"repo","sourceType":"git","repositoryUrl":"https://x","unknown":true}`,
		`{"name":"repo","sourceType":"git"}`,
		`{"name":"repo","sourceType":"local","localPath":"relative/path"}`,
		`{"name":"repo","sourceType":"git","repositoryUrl":"https://x","credentialRef":"plain-password"}`,
		`{"name":"repo","sourceType":"git","repositoryUrl":"https://x"} {}`,
	}
	for _, body := range cases {
		if w := perform(h, http.MethodPost, path, body); w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, w.Code, w.Body.String())
		}
	}
	valid := `{"name":"repo","sourceType":"git","repositoryUrl":"https://x"}`
	if w := perform(h, http.MethodPost, path, valid); w.Code != http.StatusCreated {
		t.Fatalf("first=%d %s", w.Code, w.Body.String())
	}
	if w := perform(h, http.MethodPost, path, valid); w.Code != http.StatusConflict {
		t.Fatalf("duplicate=%d %s", w.Code, w.Body.String())
	}
	if w := perform(h, http.MethodPut, path+"/not-a-uuid", valid); w.Code != http.StatusBadRequest {
		t.Fatalf("uuid=%d", w.Code)
	}
	if w := perform(h, http.MethodPut, path+"/"+testSource, valid); w.Code != http.StatusNotFound {
		t.Fatalf("missing=%d %s", w.Code, w.Body.String())
	}
}

func TestMemoryRepositoryKeepsSourcesSystemScoped(t *testing.T) {
	r := NewMemoryRepository()
	a := CodeSource{ID: testSource, SystemID: testSystem, Name: "repo", SourceType: TypeGit, RepositoryURL: "https://x", Status: StatusActive}
	if err := r.Create(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(context.Background(), CodeSource{ID: testSource, SystemID: testOtherSystem, Name: "other", SourceType: TypeGit, RepositoryURL: "https://y", Status: StatusActive}); err != ErrNotFound {
		t.Fatalf("cross update err=%v", err)
	}
	items, _ := r.List(context.Background(), testOtherSystem)
	if len(items) != 0 {
		t.Fatalf("cross-system leak: %#v", items)
	}
	if _, found, err := r.Get(context.Background(), testOtherSystem, testSource); err != nil || found {
		t.Fatalf("cross get found=%v err=%v", found, err)
	}
	if got, found, err := r.Get(context.Background(), testSystem, testSource); err != nil || !found || got.Name != "repo" {
		t.Fatalf("get=%#v found=%v err=%v", got, found, err)
	}
}
