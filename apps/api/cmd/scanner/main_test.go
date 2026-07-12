package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bizdevops/apps/api/internal/modules/scanner"
)

func TestRunWritesScanResultAsJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "routes.go"), []byte("package api\nfunc register(r Router) { r.GET(\"/health\", health) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := run([]string{root}, &stdout); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	var got output
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got.Status != scanner.StatusSucceeded || len(got.Operations) != 1 || got.Operations[0].Path != "/health" {
		t.Fatalf("run() output = %#v", got)
	}
}

func TestRunRequiresRootArgument(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("run(nil) error = nil, want usage error")
	}
}
