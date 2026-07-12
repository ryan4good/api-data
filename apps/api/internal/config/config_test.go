package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AUTH_MODE", "development")
	t.Setenv("HTTP_ADDRESS", "")
	// LookupEnv distinguishes an explicitly empty value, so remove it by using a
	// fresh key path for defaults below.
	t.Setenv("HTTP_ADDRESS", ":9090")
	t.Setenv("MYSQL_MAX_OPEN_CONNS", "42")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "3s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Address != ":9090" {
		t.Fatalf("address = %q", cfg.HTTP.Address)
	}
	if cfg.MySQL.MaxOpenConns != 42 {
		t.Fatalf("max open connections = %d", cfg.MySQL.MaxOpenConns)
	}
	if cfg.HTTP.ShutdownTimeout != 3*time.Second {
		t.Fatalf("shutdown timeout = %s", cfg.HTTP.ShutdownTimeout)
	}
}

func TestLoadRejectsEmptyAddress(t *testing.T) {
	t.Setenv("AUTH_MODE", "development")
	t.Setenv("HTTP_ADDRESS", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadJWTAuthentication(t *testing.T) {
	t.Setenv("AUTH_MODE", "jwt")
	t.Setenv("AUTH_JWT_ISSUER", "https://identity.example.test")
	t.Setenv("AUTH_JWT_AUDIENCE", "bizdevops-api")
	t.Setenv("AUTH_JWT_SIGNING_KEY", "runtime-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.Mode != "jwt" || cfg.Auth.JWTSigningKey != "runtime-secret" {
		t.Fatalf("auth=%#v", cfg.Auth)
	}
}

func TestLoadHTTPExecutorConfiguration(t *testing.T) {
	t.Setenv("AUTH_MODE", "development")
	t.Setenv("CONNECTOR_HTTP_ALLOWED_HOSTS", "api.example.test, internal.example.test ")
	t.Setenv("CONNECTOR_HTTP_ALLOW_PRIVATE", "true")
	t.Setenv("CONNECTOR_HTTP_DEFAULT_TIMEOUT", "7s")
	t.Setenv("CONNECTOR_HTTP_MAX_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ConnectorHTTP.AllowedHosts) != 2 || !cfg.ConnectorHTTP.AllowPrivate {
		t.Fatalf("connector HTTP=%#v", cfg.ConnectorHTTP)
	}
	if cfg.ConnectorHTTP.DefaultTimeout != 7*time.Second || cfg.ConnectorHTTP.MaxTimeout != 15*time.Second {
		t.Fatalf("connector timeouts=%#v", cfg.ConnectorHTTP)
	}
}

func TestLoadRejectsIncompleteOrUnknownAuthentication(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		issuer   string
		audience string
		key      string
	}{
		{name: "unknown mode", mode: "test"},
		{name: "missing issuer", mode: "jwt", audience: "api", key: "secret"},
		{name: "missing audience", mode: "jwt", issuer: "issuer", key: "secret"},
		{name: "missing signing key", mode: "jwt", issuer: "issuer", audience: "api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTH_MODE", tt.mode)
			t.Setenv("AUTH_JWT_ISSUER", tt.issuer)
			t.Setenv("AUTH_JWT_AUDIENCE", tt.audience)
			t.Setenv("AUTH_JWT_SIGNING_KEY", tt.key)
			if _, err := Load(); err == nil {
				t.Fatal("Load() expected authentication error")
			}
		})
	}
}
