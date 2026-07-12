package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAnalyzeDiscoversGinRoutes(t *testing.T) {
	root := t.TempDir()
	source := `package api

import "github.com/gin-gonic/gin"

func Register(r *gin.Engine) {
	r.GET("/orders/:id", getOrder)
	r.POST("/orders", handlers.CreateOrder)
	r.PUT("/orders/:id", updateOrder)
	r.PATCH("/orders/:id/status", patchStatus)
	r.DELETE("/orders/:id", deleteOrder)
	r.OPTIONS("/orders", optionsOrders)
	r.Get("/wrong-case", accidental)
}
`
	if err := os.WriteFile(filepath.Join(root, "routes.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := []Operation{
		{Method: "GET", Path: "/orders/:id", Handler: "getOrder", Source: SourceLocation{File: "routes.go", Line: 6}},
		{Method: "POST", Path: "/orders", Handler: "handlers.CreateOrder", Source: SourceLocation{File: "routes.go", Line: 7}},
		{Method: "PUT", Path: "/orders/:id", Handler: "updateOrder", Source: SourceLocation{File: "routes.go", Line: 8}},
		{Method: "PATCH", Path: "/orders/:id/status", Handler: "patchStatus", Source: SourceLocation{File: "routes.go", Line: 9}},
		{Method: "DELETE", Path: "/orders/:id", Handler: "deleteOrder", Source: SourceLocation{File: "routes.go", Line: 10}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestAnalyzeWalksPackagesAndSkipsGeneratedNoise(t *testing.T) {
	root := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/users/routes.go", "package users\nfunc routes(r Router) { r.GET(`/users`, listUsers) }\n")
	write("internal/users/routes_test.go", "package users\nfunc testRoutes(r Router) { r.GET(`/test-only`, fake) }\n")
	write("vendor/example/routes.go", "package example\nfunc routes(r Router) { r.GET(`/vendored`, fake) }\n")

	got, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Operation{{Method: "GET", Path: "/users", Handler: "listUsers", Source: SourceLocation{File: "internal/users/routes.go", Line: 2}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestScanStateTransitions(t *testing.T) {
	tests := []struct {
		from, to Status
		want     bool
	}{
		{StatusQueued, StatusRunning, true},
		{StatusRunning, StatusSucceeded, true},
		{StatusRunning, StatusFailed, true},
		{StatusQueued, StatusSucceeded, false},
		{StatusSucceeded, StatusRunning, false},
		{StatusFailed, StatusRunning, false},
		{StatusRunning, StatusRunning, false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}

	job := NewJob()
	if job.Status != StatusQueued {
		t.Fatalf("NewJob().Status = %q, want %q", job.Status, StatusQueued)
	}
	if err := job.Transition(StatusSucceeded); err == nil {
		t.Fatal("Transition(queued -> succeeded) error = nil, want error")
	}
	if job.Status != StatusQueued {
		t.Fatalf("invalid transition changed status to %q", job.Status)
	}
	if err := job.Transition(StatusRunning); err != nil {
		t.Fatalf("Transition(queued -> running) error = %v", err)
	}
}
