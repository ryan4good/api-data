package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bizdevops/apps/api/internal/modules/connector"
	"bizdevops/apps/api/internal/modules/execution"
	mysql "github.com/go-sql-driver/mysql"
)

type runOnce interface {
	RunOnce(context.Context) (bool, error)
}

type waitFunction func(context.Context, time.Duration) error

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("worker stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	driverConfig, err := mysql.ParseDSN(config.MySQLDSN)
	if err != nil {
		return errors.New("MYSQL_DSN is invalid")
	}
	driverConfig.ParseTime = true
	database, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return err
	}
	defer database.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := database.PingContext(ctx); err != nil {
		return err
	}
	repository := execution.NewMySQLRepository(database)
	stepProvider := execution.NewMySQLStepProvider(database)
	var stepExecutor execution.StepExecutor = execution.DisabledExecutor{}
	if len(config.AllowedHosts) > 0 {
		httpExecutor, err := connector.NewHTTPExecutor(connector.HTTPExecutorConfig{AllowedHosts: config.AllowedHosts, AllowPrivate: config.AllowPrivate})
		if err != nil {
			return err
		}
		stepExecutor = httpExecutor
	}
	executionService := execution.NewService(repository, stepProvider, stepExecutor, execution.ServiceOptions{})
	worker := execution.NewWorker(repository, executionService, execution.WorkerOptions{
		SystemID: config.SystemID, WorkerID: config.WorkerID, LeaseDuration: config.LeaseDuration, HeartbeatInterval: config.HeartbeatInterval,
	})
	return runLoop(ctx, worker, config.PollInterval, waitFor, log.Default())
}

func runLoop(ctx context.Context, runner runOnce, pollInterval time.Duration, wait waitFunction, logger *log.Logger) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		processed, err := runner.RunOnce(ctx)
		if err != nil {
			logger.Printf("worker iteration failed: %v", err)
		}
		if !processed || err != nil {
			if waitErr := wait(ctx, pollInterval); waitErr != nil {
				return waitErr
			}
		}
	}
}

func waitFor(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
