package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bizdevops/apps/api/internal/config"
	"bizdevops/apps/api/internal/modules/access"
	"bizdevops/apps/api/internal/modules/discovery"
	"bizdevops/apps/api/internal/modules/execution"
	"bizdevops/apps/api/internal/modules/importer"
	"bizdevops/apps/api/internal/modules/scanner"
	"bizdevops/apps/api/internal/modules/scenario"
	"bizdevops/apps/api/internal/modules/system"
	"github.com/golang-jwt/jwt/v5"
)

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
			Systems:     systems,
			Scans:       scanner.NewMemoryRepository(),
			Imports:     importer.NewMemoryRepository(),
			Discoveries: discovery.NewMemoryRepository(),
			Scenarios:   scenario.NewMemoryRepository(nil),
			Executions:  execution.NewMemoryRepository(),
		},
	)

	for _, path := range []string{
		"/api/v1/systems/" + systemID + "/scans",
		"/api/v1/systems/" + systemID + "/api-operations",
		"/api/v1/systems/" + systemID + "/scenario-imports",
		"/api/v1/systems/" + systemID + "/discoveries",
		"/api/v1/systems/" + systemID + "/scenarios",
		"/api/v1/systems/" + systemID + "/scenario-runs",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(access.DevelopmentUserHeader, userID)
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
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
