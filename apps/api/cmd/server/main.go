package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"bizdevops/apps/api/internal/config"
	"bizdevops/apps/api/internal/httpapi"
	"bizdevops/apps/api/internal/modules/connector"
	"bizdevops/apps/api/internal/modules/discovery"
	"bizdevops/apps/api/internal/modules/environment"
	"bizdevops/apps/api/internal/modules/execution"
	"bizdevops/apps/api/internal/modules/importer"
	"bizdevops/apps/api/internal/modules/management"
	"bizdevops/apps/api/internal/modules/scanner"
	"bizdevops/apps/api/internal/modules/scenario"
	"bizdevops/apps/api/internal/modules/system"

	_ "github.com/go-sql-driver/mysql"
)

type workflowDependencies struct {
	scans        scanner.Repository
	imports      importer.Repository
	management   management.Repository
	environments environment.Repository
	discoveries  discovery.Repository
	scenarios    scenario.Repository
	executions   execution.Repository
	steps        execution.StepProvider
}

var openSystemMySQL = func(ctx context.Context, dsn string, pool system.MySQLPoolConfig) (system.Repository, func() error, error) {
	repository, err := system.OpenMySQLRepository(ctx, dsn, pool)
	if err != nil {
		return nil, nil, err
	}
	return repository, repository.Close, nil
}

var openWorkflowMySQL = func(ctx context.Context, dsn string, pool system.MySQLPoolConfig) (workflowDependencies, func() error, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return workflowDependencies{}, nil, err
	}
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return workflowDependencies{}, nil, err
	}
	return workflowDependencies{
		scans: scanner.NewMySQLRepository(db), imports: importer.NewMySQLRepository(db),
		management:   management.NewMySQLRepository(db),
		environments: environment.NewMySQLRepository(db),
		discoveries:  discovery.NewMySQLRepository(db), executions: execution.NewMySQLRepository(db),
		scenarios: scenario.NewMySQLRepository(db),
		steps:     execution.NewMySQLStepProvider(db),
	}, db.Close, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	repository, closeRepository, err := selectSystemRepository(startupCtx, cfg.MySQL)
	if err != nil {
		cancelStartup()
		logger.Error("initialize system repository", "error", err)
		os.Exit(1)
	}
	workflows, closeWorkflowRepositories, err := selectWorkflowRepositories(startupCtx, cfg.MySQL)
	executor, executorErr := selectStepExecutor(cfg.ConnectorHTTP)
	cancelStartup()
	if err != nil {
		_ = closeRepository()
		logger.Error("initialize workflow repositories", "error", err)
		os.Exit(1)
	}
	if executorErr != nil {
		_ = closeRepository()
		_ = closeWorkflowRepositories()
		logger.Error("initialize step executor", "error", executorErr)
		os.Exit(1)
	}
	defer func() {
		if err := closeRepository(); err != nil {
			logger.Error("close system repository", "error", err)
		}
	}()
	defer func() {
		if err := closeWorkflowRepositories(); err != nil {
			logger.Error("close workflow repositories", "error", err)
		}
	}()

	server := &http.Server{
		Addr: cfg.HTTP.Address,
		Handler: httpapi.NewWithDependencies(cfg, logger, httpapi.Dependencies{
			Systems: repository, Scans: workflows.scans, Imports: workflows.imports, Management: workflows.management, Environments: workflows.environments,
			Discoveries: workflows.discoveries, Scenarios: workflows.scenarios, Executions: workflows.executions, Steps: workflows.steps, Executor: executor,
		}),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api server started", "address", cfg.HTTP.Address, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = server.Close()
	}
	logger.Info("api server stopped")
}

func selectWorkflowRepositories(ctx context.Context, cfg config.MySQL) (workflowDependencies, func() error, error) {
	if cfg.DSN == "" {
		return workflowDependencies{
			scans: scanner.NewMemoryRepository(), imports: importer.NewMemoryRepository(),
			management:   management.NewMemoryRepository(nil, nil, nil),
			environments: environment.NewMemoryRepository(),
			discoveries:  discovery.NewMemoryRepository(), executions: execution.NewMemoryRepository(),
			scenarios: scenario.NewMemoryRepository(nil),
		}, func() error { return nil }, nil
	}
	return openWorkflowMySQL(ctx, cfg.DSN, system.MySQLPoolConfig{
		MaxOpenConns: cfg.MaxOpenConns, MaxIdleConns: cfg.MaxIdleConns, ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
}

func selectStepExecutor(cfg config.ConnectorHTTP) (execution.StepExecutor, error) {
	if len(cfg.AllowedHosts) == 0 {
		return execution.DisabledExecutor{}, nil
	}
	return connector.NewHTTPExecutor(connector.HTTPExecutorConfig{
		AllowedHosts: cfg.AllowedHosts, AllowPrivate: cfg.AllowPrivate,
		MaxRequestBodyBytes: cfg.MaxRequestBodyBytes, MaxResponseBodyBytes: cfg.MaxResponseBodyBytes,
		DefaultTimeout: cfg.DefaultTimeout, MaxTimeout: cfg.MaxTimeout,
	})
}

func selectSystemRepository(ctx context.Context, cfg config.MySQL) (system.Repository, func() error, error) {
	if cfg.DSN == "" {
		return system.NewMemoryRepository(nil, nil), func() error { return nil }, nil
	}
	return openSystemMySQL(ctx, cfg.DSN, system.MySQLPoolConfig{
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
}
