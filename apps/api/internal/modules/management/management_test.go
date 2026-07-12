package management

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"bizdevops/apps/api/internal/modules/access"
)

const (
	managementUser    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	managementAdmin   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	managementSystemA = "11111111-1111-4111-8111-111111111111"
	managementSystemB = "22222222-2222-4222-8222-222222222222"
)

func managementFixtures() []SystemOverview {
	return []SystemOverview{
		{SystemID: managementSystemA, SystemKey: "oms", SystemName: "OMS", APICount: 12, P0PendingCount: 2, ScenarioCount: 4, Runs24h: RunCounts{Passed: 7, Failed: 1}},
		{SystemID: managementSystemB, SystemKey: "wms", SystemName: "WMS", APICount: 8, ScenarioCount: 3, Runs24h: RunCounts{Running: 1}},
	}
}

func TestMemoryRepositoryScopesMembersAndLetsPlatformAdminSeeAll(t *testing.T) {
	repository := NewMemoryRepository(managementFixtures(), map[string]map[string]string{
		managementUser: {managementSystemA: "owner"},
	}, map[string]bool{managementAdmin: true})

	member, err := repository.Overview(context.Background(), managementUser)
	if err != nil || member.Scope != ScopeMember || len(member.Systems) != 1 || member.Systems[0].SystemID != managementSystemA {
		t.Fatalf("member overview=%#v err=%v", member, err)
	}
	admin, err := repository.Overview(context.Background(), managementAdmin)
	if err != nil || admin.Scope != ScopePlatform || len(admin.Systems) != 2 {
		t.Fatalf("admin overview=%#v err=%v", admin, err)
	}
	if admin.APICount != 20 || admin.Runs24h.Failed != 1 {
		t.Fatalf("admin totals=%#v", admin)
	}
	if _, found, err := repository.SystemOverview(context.Background(), managementUser, managementSystemB); err != nil || found {
		t.Fatalf("cross-system detail found=%v err=%v", found, err)
	}
}

func TestManagementHTTPRequiresIdentityAndHidesUnauthorizedSystems(t *testing.T) {
	repository := NewMemoryRepository(managementFixtures(), map[string]map[string]string{
		managementUser: {managementSystemA: "viewer"},
	}, nil)
	mux := http.NewServeMux()
	Register(mux, repository, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := access.DevelopmentIdentity(mux)
	overviewRecorder := httptest.NewRecorder()
	overviewRequest := httptest.NewRequest(http.MethodGet, "/api/v1/management/overview", nil)
	overviewRequest.Header.Set(access.DevelopmentUserHeader, managementUser)
	handler.ServeHTTP(overviewRecorder, overviewRequest)
	var overviewEnvelope map[string]any
	if err := json.Unmarshal(overviewRecorder.Body.Bytes(), &overviewEnvelope); err != nil {
		t.Fatal(err)
	}
	overviewData := overviewEnvelope["data"].(map[string]any)
	if overviewData["accessScope"] != "authorized" || overviewData["apiAssetCount"] != float64(12) {
		t.Fatalf("frontend management contract=%#v", overviewData)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/management/overview", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/management/systems/"+managementSystemB+"/overview", nil)
	request.Header.Set(access.DevelopmentUserHeader, managementUser)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-system=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["error"].(map[string]any)["code"] != "system_not_found" {
		t.Fatalf("envelope=%#v", envelope)
	}
}
