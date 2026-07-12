package connector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"bizdevops/apps/api/internal/modules/execution"
)

func TestHTTPExecutorBuildsRequestExtractsAndAssertsWithRedactedSnapshots(t *testing.T) {
	type receivedRequest struct {
		Path, Query, Authorization, Body string
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		received <- receivedRequest{request.URL.Path, request.URL.Query().Get("trace"), request.Header.Get("Authorization"), string(body)}
		w.Header().Set("Set-Cookie", "session=response-secret")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"created-42","state":"ready"},"token":"response-secret"}`))
	}))
	defer server.Close()
	executor := newLocalExecutor(t, server.URL, HTTPExecutorConfig{})
	secret := "request-secret"
	step := execution.Step{ID: "create", Type: "http", Config: map[string]any{
		"baseURL": server.URL, "method": "POST", "path": "/orders/{{orderId}}",
		"headers":    map[string]any{"Authorization": "Bearer {{apiToken}}", "X-Trace": "safe"},
		"query":      map[string]any{"trace": "{{trace}}"},
		"body":       map[string]any{"name": "order-{{orderId}}", "password": "{{apiToken}}"},
		"extractors": []any{map[string]any{"key": "createdId", "expression": "$.data.id", "required": true}},
		"assertions": []any{
			map[string]any{"key": "status-created", "type": "status", "operator": "eq", "expected": 201},
			map[string]any{"key": "state-ready", "type": "body", "actual": "$.data.state", "operator": "eq", "expected": "ready"},
		},
	}}
	result, err := executor.Execute(context.Background(), step, map[string]any{"orderId": "42", "trace": "trace-1", "apiToken": secret})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	request := <-received
	if request.Path != "/orders/42" || request.Query != "trace-1" || request.Authorization != "Bearer "+secret || !strings.Contains(request.Body, `"password":"`+secret+`"`) {
		t.Fatalf("request substitution failed: %#v", request)
	}
	if result.ExtractedVariables["createdId"] != "created-42" || len(result.Assertions) != 2 || result.Assertions[0].Status != execution.AssertionPassed || result.Assertions[1].Status != execution.AssertionPassed {
		t.Fatalf("extract/assert result = %#v", result)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "response-secret") {
		t.Fatalf("snapshot leaked secret: %s", encoded)
	}
}

func TestHTTPExecutorRejectsSSRFAndNonAllowlistedHosts(t *testing.T) {
	privateExecutor, err := NewHTTPExecutor(HTTPExecutorConfig{AllowedHosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = privateExecutor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": "http://127.0.0.1:8080", "path": "/"}}, nil)
	if !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("private target error = %v", err)
	}

	allowlistExecutor, err := NewHTTPExecutor(HTTPExecutorConfig{AllowedHosts: []string{"api.example.test"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = allowlistExecutor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": "http://169.254.169.254", "path": "/latest/meta-data"}}, nil)
	if !errors.Is(err, ErrTargetNotAllowed) {
		t.Fatalf("allowlist error = %v", err)
	}
	_, err = allowlistExecutor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": "file:///etc/passwd"}}, nil)
	if !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("scheme error = %v", err)
	}
}

func TestHTTPExecutorBlocksRedirectToNonAllowedHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, "http://example.com/escaped", http.StatusFound)
	}))
	defer server.Close()
	executor := newLocalExecutor(t, server.URL, HTTPExecutorConfig{})
	_, err := executor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": server.URL, "path": "/redirect"}}, nil)
	if !errors.Is(err, ErrTargetNotAllowed) {
		t.Fatalf("redirect error = %v", err)
	}
}

func TestHTTPExecutorEnforcesTimeoutAndBodyLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/slow" {
			time.Sleep(40 * time.Millisecond)
		}
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	}))
	defer server.Close()
	executor := newLocalExecutor(t, server.URL, HTTPExecutorConfig{MaxRequestBodyBytes: 16, MaxResponseBodyBytes: 16, MaxTimeout: time.Second})
	_, err := executor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": server.URL, "path": "/", "method": "POST", "body": strings.Repeat("x", 32)}}, nil)
	if !errors.Is(err, ErrRequestBodyTooLarge) {
		t.Fatalf("request limit error = %v", err)
	}
	_, err = executor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": server.URL, "path": "/"}}, nil)
	if !errors.Is(err, ErrResponseBodyTooLarge) {
		t.Fatalf("response limit error = %v", err)
	}
	_, err = executor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": server.URL, "path": "/slow", "timeoutMs": 5}}, nil)
	if !errors.Is(err, ErrRequestTimeout) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestHTTPExecutorReportsUnboundVariablesAndFailedAssertions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"wrong"}`))
	}))
	defer server.Close()
	executor := newLocalExecutor(t, server.URL, HTTPExecutorConfig{})
	_, err := executor.Execute(context.Background(), execution.Step{Config: map[string]any{"baseURL": server.URL, "path": "/{{missing}}"}}, nil)
	if !errors.Is(err, ErrUnboundVariable) {
		t.Fatalf("unbound variable error = %v", err)
	}
	result, err := executor.Execute(context.Background(), execution.Step{Config: map[string]any{
		"baseURL": server.URL, "path": "/", "assertions": []any{map[string]any{"key": "state", "type": "body", "path": "state", "operator": "eq", "expected": "ready"}},
	}}, nil)
	if err != nil || result.Assertions[0].Status != execution.AssertionFailed {
		t.Fatalf("failed assertion result=%#v err=%v", result, err)
	}
}

func newLocalExecutor(t *testing.T, rawURL string, overrides HTTPExecutorConfig) *HTTPExecutor {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	overrides.AllowedHosts = []string{parsed.Hostname()}
	overrides.AllowPrivate = true
	executor, err := NewHTTPExecutor(overrides)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}
