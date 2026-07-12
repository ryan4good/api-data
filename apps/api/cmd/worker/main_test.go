package main

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"
)

func TestLoadConfigRequiresDatabaseSystemAndWorkerIdentity(t *testing.T) {
	for _, missing := range []string{"MYSQL_DSN", "WORKER_SYSTEM_ID", "WORKER_ID"} {
		values := validWorkerEnvironment()
		delete(values, missing)
		_, err := loadConfig(func(key string) string { return values[key] })
		if err == nil {
			t.Fatalf("missing %s expected error", missing)
		}
	}
}

func TestLoadConfigParsesDurationsAndHTTPPolicy(t *testing.T) {
	values := validWorkerEnvironment()
	values["WORKER_POLL_INTERVAL"] = "250ms"
	values["WORKER_LEASE_DURATION"] = "45s"
	values["WORKER_HEARTBEAT_INTERVAL"] = "10s"
	values["HTTP_EXECUTOR_ALLOWED_HOSTS"] = "api.example.test, internal.example.test "
	values["HTTP_EXECUTOR_ALLOW_PRIVATE"] = "true"
	config, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if config.PollInterval != 250*time.Millisecond || config.LeaseDuration != 45*time.Second || config.HeartbeatInterval != 10*time.Second || len(config.AllowedHosts) != 2 || !config.AllowPrivate {
		t.Fatalf("config = %#v", config)
	}
}

type scriptedRunner struct {
	results []bool
	errors  []error
	calls   int
}

func (runner *scriptedRunner) RunOnce(context.Context) (bool, error) {
	index := runner.calls
	runner.calls++
	if index >= len(runner.results) {
		return false, nil
	}
	return runner.results[index], runner.errors[index]
}

func TestRunLoopWaitsOnEmptyOrErrorAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &scriptedRunner{results: []bool{false, true}, errors: []error{nil, errors.New("temporary")}}
	waits := 0
	wait := func(context.Context, time.Duration) error {
		waits++
		if waits == 2 {
			cancel()
		}
		return nil
	}
	err := runLoop(ctx, runner, 100*time.Millisecond, wait, log.New(io.Discard, "", 0))
	if !errors.Is(err, context.Canceled) || waits != 2 || runner.calls != 2 {
		t.Fatalf("runLoop() err=%v waits=%d calls=%d", err, waits, runner.calls)
	}
}

func validWorkerEnvironment() map[string]string {
	return map[string]string{
		"MYSQL_DSN":        "worker:secret@tcp(localhost:3306)/bizdevops",
		"WORKER_SYSTEM_ID": "51000000-0000-4000-8000-000000000001",
		"WORKER_ID":        "worker-a",
	}
}
