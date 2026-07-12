package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type workerConfig struct {
	MySQLDSN          string
	SystemID          string
	WorkerID          string
	PollInterval      time.Duration
	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
	AllowedHosts      []string
	AllowPrivate      bool
}

func loadConfig(getenv func(string) string) (workerConfig, error) {
	config := workerConfig{
		MySQLDSN: strings.TrimSpace(getenv("MYSQL_DSN")), SystemID: strings.TrimSpace(getenv("WORKER_SYSTEM_ID")),
		WorkerID: strings.TrimSpace(getenv("WORKER_ID")), PollInterval: time.Second, LeaseDuration: 30 * time.Second, HeartbeatInterval: 10 * time.Second,
	}
	for key, value := range map[string]string{"MYSQL_DSN": config.MySQLDSN, "WORKER_SYSTEM_ID": config.SystemID, "WORKER_ID": config.WorkerID} {
		if value == "" {
			return workerConfig{}, fmt.Errorf("%s is required", key)
		}
	}
	var err error
	if config.PollInterval, err = durationSetting(getenv("WORKER_POLL_INTERVAL"), config.PollInterval); err != nil {
		return workerConfig{}, fmt.Errorf("WORKER_POLL_INTERVAL: %w", err)
	}
	if config.LeaseDuration, err = durationSetting(getenv("WORKER_LEASE_DURATION"), config.LeaseDuration); err != nil {
		return workerConfig{}, fmt.Errorf("WORKER_LEASE_DURATION: %w", err)
	}
	if config.HeartbeatInterval, err = durationSetting(getenv("WORKER_HEARTBEAT_INTERVAL"), config.HeartbeatInterval); err != nil {
		return workerConfig{}, fmt.Errorf("WORKER_HEARTBEAT_INTERVAL: %w", err)
	}
	if config.HeartbeatInterval >= config.LeaseDuration {
		return workerConfig{}, errors.New("WORKER_HEARTBEAT_INTERVAL must be shorter than WORKER_LEASE_DURATION")
	}
	for _, host := range strings.Split(getenv("HTTP_EXECUTOR_ALLOWED_HOSTS"), ",") {
		if host = strings.TrimSpace(host); host != "" {
			config.AllowedHosts = append(config.AllowedHosts, host)
		}
	}
	if raw := strings.TrimSpace(getenv("HTTP_EXECUTOR_ALLOW_PRIVATE")); raw != "" {
		config.AllowPrivate, err = strconv.ParseBool(raw)
		if err != nil {
			return workerConfig{}, errors.New("HTTP_EXECUTOR_ALLOW_PRIVATE must be true or false")
		}
	}
	return config, nil
}

func durationSetting(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("must be a positive duration")
	}
	return value, nil
}
