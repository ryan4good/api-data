package main

import (
	"context"
	"testing"
	"time"

	"bizdevops/apps/api/internal/config"
	"bizdevops/apps/api/internal/modules/connector"
	"bizdevops/apps/api/internal/modules/discovery"
	"bizdevops/apps/api/internal/modules/execution"
	"bizdevops/apps/api/internal/modules/importer"
	"bizdevops/apps/api/internal/modules/scanner"
	"bizdevops/apps/api/internal/modules/scenario"
	"bizdevops/apps/api/internal/modules/system"
)

func TestSelectSystemRepositoryKeepsMemoryWithoutDSN(t *testing.T) {
	repository, closeRepository, err := selectSystemRepository(context.Background(), config.MySQL{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repository.(*system.MemoryRepository); !ok {
		t.Fatalf("repository type = %T, want *system.MemoryRepository", repository)
	}
	if err := closeRepository(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectWorkflowRepositoriesUsesMemoryWithoutDSN(t *testing.T) {
	dependencies, closeRepositories, err := selectWorkflowRepositories(context.Background(), config.MySQL{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := dependencies.scans.(*scanner.MemoryRepository); !ok {
		t.Fatalf("scanner repository type = %T", dependencies.scans)
	}
	if _, ok := dependencies.imports.(*importer.MemoryRepository); !ok {
		t.Fatalf("import repository type = %T", dependencies.imports)
	}
	if _, ok := dependencies.discoveries.(*discovery.MemoryRepository); !ok {
		t.Fatalf("discovery repository type = %T", dependencies.discoveries)
	}
	if _, ok := dependencies.executions.(*execution.MemoryRepository); !ok {
		t.Fatalf("execution repository type = %T", dependencies.executions)
	}
	if _, ok := dependencies.scenarios.(*scenario.MemoryRepository); !ok {
		t.Fatalf("scenario repository type = %T", dependencies.scenarios)
	}
	if err := closeRepositories(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectWorkflowRepositoriesOpensConfiguredMySQL(t *testing.T) {
	original := openWorkflowMySQL
	t.Cleanup(func() { openWorkflowMySQL = original })
	wantScans := scanner.NewMemoryRepository()
	wantImports := importer.NewMemoryRepository()
	wantDiscoveries := discovery.NewMemoryRepository()
	wantExecutions := execution.NewMemoryRepository()
	wantScenarios := scenario.NewMemoryRepository(nil)
	closed := false
	openWorkflowMySQL = func(ctx context.Context, dsn string, pool system.MySQLPoolConfig) (workflowDependencies, func() error, error) {
		if dsn != "configured-dsn" || pool.MaxOpenConns != 7 || pool.MaxIdleConns != 3 {
			t.Fatalf("dsn=%q pool=%#v", dsn, pool)
		}
		return workflowDependencies{scans: wantScans, imports: wantImports, discoveries: wantDiscoveries, scenarios: wantScenarios, executions: wantExecutions}, func() error { closed = true; return nil }, nil
	}

	dependencies, closeRepositories, err := selectWorkflowRepositories(context.Background(), config.MySQL{
		DSN: "configured-dsn", MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dependencies.scans != wantScans || dependencies.imports != wantImports || dependencies.discoveries != wantDiscoveries || dependencies.scenarios != wantScenarios || dependencies.executions != wantExecutions {
		t.Fatalf("unexpected repositories: %#v", dependencies)
	}
	if err := closeRepositories(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("workflow database was not closed")
	}
}

func TestSelectStepExecutorUsesSafeDisabledDefaultAndExplicitHTTPConfig(t *testing.T) {
	disabled, err := selectStepExecutor(config.ConnectorHTTP{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := disabled.(execution.DisabledExecutor); !ok {
		t.Fatalf("default executor type = %T", disabled)
	}

	httpExecutor, err := selectStepExecutor(config.ConnectorHTTP{AllowedHosts: []string{"api.example.test"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := httpExecutor.(*connector.HTTPExecutor); !ok {
		t.Fatalf("configured executor type = %T", httpExecutor)
	}
}

func TestSelectSystemRepositoryOpensConfiguredMySQLAndReturnsCloser(t *testing.T) {
	original := openSystemMySQL
	t.Cleanup(func() { openSystemMySQL = original })

	wantRepository := system.NewMemoryRepository(nil, nil)
	closed := false
	openSystemMySQL = func(ctx context.Context, dsn string, pool system.MySQLPoolConfig) (system.Repository, func() error, error) {
		if dsn != "configured-dsn" {
			t.Fatalf("dsn=%q", dsn)
		}
		if pool.MaxOpenConns != 7 || pool.MaxIdleConns != 3 || pool.ConnMaxLifetime != 5*time.Minute {
			t.Fatalf("pool=%#v", pool)
		}
		return wantRepository, func() error { closed = true; return nil }, nil
	}

	repository, closeRepository, err := selectSystemRepository(context.Background(), config.MySQL{
		DSN: "configured-dsn", MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository != wantRepository {
		t.Fatalf("repository=%T, want injected repository", repository)
	}
	if err := closeRepository(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("mysql repository was not closed")
	}
}
