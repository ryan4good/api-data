package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bizdevops/apps/api/internal/config"
	"bizdevops/apps/api/internal/modules/access"
	"bizdevops/apps/api/internal/modules/codesource"
	"bizdevops/apps/api/internal/modules/discovery"
	"bizdevops/apps/api/internal/modules/environment"
	"bizdevops/apps/api/internal/modules/execution"
	"bizdevops/apps/api/internal/modules/importer"
	"bizdevops/apps/api/internal/modules/management"
	"bizdevops/apps/api/internal/modules/scanner"
	"bizdevops/apps/api/internal/modules/scenario"
	"bizdevops/apps/api/internal/modules/system"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthenticationRoutesUseInjectedUsersAndCookieAlongsideNginxBasicAuthorization(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password-123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := access.NewMemoryUserRepository([]access.User{{
		ID: "10000000-0000-4000-8000-000000000001", Email: "admin@example.test", DisplayName: "Admin",
		PasswordHash: string(hash), PlatformRole: "admin", Status: "active",
	}})
	handler := NewWithDependencies(config.Config{ServiceName: "test-api", Environment: "test", Auth: config.Auth{
		Mode: "jwt", JWTIssuer: "issuer", JWTAudience: "audience", JWTSigningKey: "signing-key", JWTTokenTTL: time.Hour,
		CookieName: "bizdevops_session", CookiePath: "/", CookieSecure: true,
	}}, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{AuthUsers: users})

	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.test","password":"password-123"}`)))
	if login.Code != http.StatusOK || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login status=%d body=%s cookies=%#v", login.Code, login.Body.String(), login.Result().Cookies())
	}

	me := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Basic nginx-credential-placeholder")
	request.AddCookie(login.Result().Cookies()[0])
	handler.ServeHTTP(me, request)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"platformRole":"admin"`) {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
}

func TestSystemWorkflowRoutesAreRegisteredWithInjectedRepositories(t *testing.T) {
	const systemID = "11111111-1111-4111-8111-111111111111"
	const userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	systems := system.NewMemoryRepository(
		[]system.BusinessSystem{{ID: systemID, Code: "orders", Name: "Orders", Status: system.StatusActive}},
		[]system.Member{{SystemID: systemID, UserID: userID, Role: access.RoleOwner, Status: system.MemberActive}},
	)
	handler := NewWithDependencies(
		config.Config{ServiceName: "test-api", Environment: "test", Auth: config.Auth{Mode: "development"}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencies{
			Systems:         systems,
			CodeSources:     codesource.NewMemoryRepository(),
			Scans:           scanner.NewMemoryRepository(),
			Imports:         importer.NewMemoryRepository(),
			Management:      management.NewMemoryRepository(nil, nil, nil),
			Discoveries:     discovery.NewMemoryRepository(),
			Scenarios:       scenario.NewMemoryRepository(nil),
			Executions:      execution.NewMemoryRepository(),
			AsyncExecutions: execution.NewMemoryRepository(),
			Environments:    environment.NewMemoryRepository(),
		},
	)

	for _, path := range []string{
		"/api/v1/systems/" + systemID + "/scans",
		"/api/v1/systems/" + systemID + "/api-operations",
		"/api/v1/systems/" + systemID + "/scenario-imports",
		"/api/v1/systems/" + systemID + "/discoveries",
		"/api/v1/systems/" + systemID + "/scenarios",
		"/api/v1/systems/" + systemID + "/scenario-runs",
		"/api/v1/management/overview",
		"/api/v1/systems/" + systemID + "/environments",
		"/api/v1/systems/" + systemID + "/code-sources",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(access.DevelopmentUserHeader, userID)
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
	asyncRecorder := httptest.NewRecorder()
	asyncRequest := httptest.NewRequest(http.MethodGet, "/api/v1/systems/"+systemID+"/scenario-run-jobs", nil)
	asyncRequest.Header.Set(access.DevelopmentUserHeader, userID)
	handler.ServeHTTP(asyncRecorder, asyncRequest)
	if asyncRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("async route status=%d body=%s", asyncRecorder.Code, asyncRecorder.Body.String())
	}
}

func TestScannerUsesConfiguredAllowedRootFromComposition(t *testing.T) {
	const systemID = "11111111-1111-4111-8111-111111111111"
	const sourceID = "22222222-2222-4222-8222-222222222222"
	const userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	root := t.TempDir()
	systems := system.NewMemoryRepository([]system.BusinessSystem{{ID: systemID, Code: "orders", Name: "Orders", Status: system.StatusActive}}, []system.Member{{SystemID: systemID, UserID: userID, Role: access.RoleOwner, Status: system.MemberActive}})
	sources := codesource.NewMemoryRepository()
	if err := sources.Create(context.Background(), codesource.CodeSource{ID: sourceID, SystemID: systemID, Name: "orders", SourceType: codesource.TypeLocal, LocalPath: root, Status: codesource.StatusActive, CreatedBy: userID}); err != nil {
		t.Fatal(err)
	}
	handler := NewWithDependencies(config.Config{ServiceName: "test-api", Environment: "test", Auth: config.Auth{Mode: "development"}, Scanner: config.Scanner{AllowedRoots: []string{root}}}, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{Systems: systems, CodeSources: sources, Scans: scanner.NewMemoryRepository()})
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/systems/"+systemID+"/scans", strings.NewReader(`{"codeSourceId":"`+sourceID+`"}`))
	createRequest.Header.Set(access.DevelopmentUserHeader, userID)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, createRequest)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", created.Code, created.Body.String())
	}
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &envelope); err != nil || envelope.Data.ID == "" {
		t.Fatalf("decode=%v body=%s", err, created.Body.String())
	}
	runRequest := httptest.NewRequest(http.MethodPost, "/api/v1/systems/"+systemID+"/scans/"+envelope.Data.ID+"/run", strings.NewReader(`{}`))
	runRequest.Header.Set(access.DevelopmentUserHeader, userID)
	ran := httptest.NewRecorder()
	handler.ServeHTTP(ran, runRequest)
	if ran.Code != http.StatusOK {
		t.Fatalf("run=%d body=%s", ran.Code, ran.Body.String())
	}
}

func TestHealthAndModuleRoutes(t *testing.T) {
	handler := New(config.Config{ServiceName: "test-api", Environment: "test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	tests := []struct {
		path string
		code int
	}{
		{"/healthz", http.StatusOK},
		{"/api/v1/status", http.StatusOK},
		{"/api/v1/systems", http.StatusUnauthorized},
		{"/api/v1/access", http.StatusNotImplemented},
		{"/api/v1/scans", http.StatusNotImplemented},
		{"/api/v1/apis", http.StatusNotImplemented},
		{"/api/v1/discoveries", http.StatusNotImplemented},
		{"/api/v1/scenarios", http.StatusNotImplemented},
		{"/api/v1/executions", http.StatusNotImplemented},
		{"/api/v1/runs", http.StatusNotImplemented},
		{"/api/v1/connectors", http.StatusNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if recorder.Code != tt.code {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.code, recorder.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
		})
	}
}

func TestReadinessReflectsMySQLConfiguration(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		code int
	}{
		{name: "missing DSN", code: http.StatusServiceUnavailable},
		{name: "configured DSN", dsn: "user:password@tcp(mysql:3306)/bizdevops", code: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(config.Config{
				ServiceName: "test-api",
				Environment: "test",
				MySQL:       config.MySQL{DSN: tt.dsn},
			}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if recorder.Code != tt.code {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.code, recorder.Body.String())
			}
		})
	}
}

func TestResponseEnvelopeContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name       string
		method     string
		path       string
		dsn        string
		wantStatus int
		wantKey    string
	}{
		{name: "health success", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantKey: "data"},
		{name: "status success", method: http.MethodGet, path: "/api/v1/status", wantStatus: http.StatusOK, wantKey: "data"},
		{name: "readiness success", method: http.MethodGet, path: "/readyz", dsn: "user:password@tcp(mysql:3306)/bizdevops", wantStatus: http.StatusOK, wantKey: "data"},
		{name: "readiness failure", method: http.MethodGet, path: "/readyz", wantStatus: http.StatusServiceUnavailable, wantKey: "error"},
		{name: "placeholder failure", method: http.MethodGet, path: "/api/v1/scenarios", wantStatus: http.StatusNotImplemented, wantKey: "error"},
		{name: "method failure", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed, wantKey: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(config.Config{
				ServiceName: "test-api",
				Environment: "test",
				MySQL:       config.MySQL{DSN: tt.dsn},
			}, logger)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}

			var envelope map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if _, ok := envelope[tt.wantKey]; !ok {
				t.Fatalf("response lacks %q envelope: %v", tt.wantKey, envelope)
			}
			otherKey := "error"
			if tt.wantKey == "error" {
				otherKey = "data"
			}
			if _, ok := envelope[otherKey]; ok {
				t.Fatalf("response unexpectedly contains %q: %v", otherKey, envelope)
			}
			if tt.wantKey == "error" {
				errorBody, ok := envelope["error"].(map[string]any)
				if !ok {
					t.Fatalf("error is not an object: %v", envelope["error"])
				}
				if errorBody["code"] == "" || errorBody["message"] == "" {
					t.Fatalf("error requires code and message: %v", errorBody)
				}
			}
		})
	}
}

func TestDevelopmentAuthenticationAndMeRoute(t *testing.T) {
	handler := New(config.Config{
		ServiceName: "test-api",
		Environment: "test",
		Auth:        config.Auth{Mode: "development"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Dev-User-ID", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestProductionJWTAuthenticationAndDevelopmentHeaderRejection(t *testing.T) {
	const signingKey = "test-signing-key-that-is-not-used-in-production"
	handler := New(config.Config{
		ServiceName: "test-api",
		Environment: "test",
		Auth: config.Auth{
			Mode: "jwt", JWTIssuer: "issuer", JWTAudience: "audience", JWTSigningKey: signingKey,
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "issuer", "aud": "audience", "sub": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(signingKey))
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+signed)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("JWT status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Dev-User-ID", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("development header status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
