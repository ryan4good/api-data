package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bizdevops/apps/api/internal/modules/access"
)

const (
	testSystemID = "11111111-1111-4111-8111-111111111111"
	testOwnerID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testViewerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

func newTestHandler() http.Handler {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository(
		[]BusinessSystem{{ID: testSystemID, Code: "billing", Name: "Billing", Status: StatusActive, CreatedAt: now, UpdatedAt: now}},
		[]Member{
			{SystemID: testSystemID, UserID: testOwnerID, DisplayName: "Owner", Role: access.RoleOwner, Status: MemberActive},
			{SystemID: testSystemID, UserID: testViewerID, DisplayName: "Viewer", Role: access.RoleViewer, Status: MemberActive},
		},
	)
	mux := http.NewServeMux()
	Register(mux, repo)
	return mux
}

func request(t *testing.T, handler http.Handler, method, path, userID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if userID != "" {
		req.Header.Set(access.DevelopmentUserHeader, userID)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, recorder.Body.String())
	}
	return result
}

func TestSystemHTTPListAndDetailAreMembershipScoped(t *testing.T) {
	handler := newTestHandler()

	missing := request(t, handler, http.MethodGet, "/api/v1/systems", "", nil)
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing identity status=%d, want 401", missing.Code)
	}

	list := request(t, handler, http.MethodGet, "/api/v1/systems", testViewerID, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	items := decodeEnvelope(t, list)["data"].([]any)
	item := items[0].(map[string]any)
	if item["id"] != testSystemID || item["myRole"] != "viewer" {
		t.Fatalf("unexpected system: %#v", item)
	}

	detail := request(t, handler, http.MethodGet, "/api/v1/systems/"+testSystemID, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", nil)
	if detail.Code != http.StatusNotFound {
		t.Fatalf("outside scope status=%d, want 404; body=%s", detail.Code, detail.Body.String())
	}
}

func TestSystemHTTPMemberManagementRequiresOwner(t *testing.T) {
	handler := newTestHandler()
	path := "/api/v1/systems/" + testSystemID + "/members"

	forbidden := request(t, handler, http.MethodGet, path, testViewerID, nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("viewer status=%d, want 403", forbidden.Code)
	}

	list := request(t, handler, http.MethodGet, path, testOwnerID, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("owner list status=%d body=%s", list.Code, list.Body.String())
	}

	newUserID := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	payload := []byte(`{"userId":"` + newUserID + `","role":"runner"}`)
	updated := request(t, handler, http.MethodPost, path, testOwnerID, payload)
	if updated.Code != http.StatusOK {
		t.Fatalf("upsert status=%d body=%s", updated.Code, updated.Body.String())
	}
	member := decodeEnvelope(t, updated)["data"].(map[string]any)
	if member["userId"] != newUserID || member["displayName"] != newUserID || member["role"] != "runner" || member["status"] != "active" {
		t.Fatalf("unexpected member: %#v", member)
	}
}

func TestSystemHTTPRejectsInvalidMemberInput(t *testing.T) {
	handler := newTestHandler()
	path := "/api/v1/systems/" + testSystemID + "/members"
	response := request(t, handler, http.MethodPost, path, testOwnerID, []byte(`{"userId":"not-a-uuid","role":"admin"}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", response.Code, response.Body.String())
	}
}
