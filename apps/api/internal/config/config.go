package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName   string
	Environment   string
	HTTP          HTTP
	MySQL         MySQL
	Auth          Auth
	ConnectorHTTP ConnectorHTTP
}

type ConnectorHTTP struct {
	AllowedHosts         []string
	AllowPrivate         bool
	MaxRequestBodyBytes  int64
	MaxResponseBodyBytes int64
	DefaultTimeout       time.Duration
	MaxTimeout           time.Duration
}

type Auth struct {
	Mode          string
	JWTIssuer     string
	JWTAudience   string
	JWTSigningKey string
}

type HTTP struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type MySQL struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ServiceName: env("SERVICE_NAME", "bizdevops-api"),
		Environment: env("APP_ENV", "development"),
		HTTP: HTTP{
			Address:           env("HTTP_ADDRESS", ":8080"),
			ReadHeaderTimeout: duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		MySQL: MySQL{
			DSN:             os.Getenv("MYSQL_DSN"),
			MaxOpenConns:    integer("MYSQL_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    integer("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: duration("MYSQL_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Auth: Auth{
			Mode:          env("AUTH_MODE", "jwt"),
			JWTIssuer:     os.Getenv("AUTH_JWT_ISSUER"),
			JWTAudience:   os.Getenv("AUTH_JWT_AUDIENCE"),
			JWTSigningKey: os.Getenv("AUTH_JWT_SIGNING_KEY"),
		},
		ConnectorHTTP: ConnectorHTTP{
			AllowedHosts:         splitCSV(os.Getenv("CONNECTOR_HTTP_ALLOWED_HOSTS")),
			AllowPrivate:         boolean("CONNECTOR_HTTP_ALLOW_PRIVATE", false),
			MaxRequestBodyBytes:  int64(integer("CONNECTOR_HTTP_MAX_REQUEST_BODY_BYTES", 1<<20)),
			MaxResponseBodyBytes: int64(integer("CONNECTOR_HTTP_MAX_RESPONSE_BODY_BYTES", 1<<20)),
			DefaultTimeout:       duration("CONNECTOR_HTTP_DEFAULT_TIMEOUT", 30*time.Second),
			MaxTimeout:           duration("CONNECTOR_HTTP_MAX_TIMEOUT", 60*time.Second),
		},
	}

	if cfg.HTTP.Address == "" {
		return Config{}, fmt.Errorf("HTTP_ADDRESS cannot be empty")
	}
	if cfg.HTTP.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT must be positive")
	}
	if err := validateAuth(cfg.Auth); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func splitCSV(value string) []string {
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func boolean(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func validateAuth(auth Auth) error {
	switch auth.Mode {
	case "development":
		return nil
	case "jwt":
		if auth.JWTIssuer == "" {
			return fmt.Errorf("AUTH_JWT_ISSUER is required when AUTH_MODE=jwt")
		}
		if auth.JWTAudience == "" {
			return fmt.Errorf("AUTH_JWT_AUDIENCE is required when AUTH_MODE=jwt")
		}
		if auth.JWTSigningKey == "" {
			return fmt.Errorf("AUTH_JWT_SIGNING_KEY is required when AUTH_MODE=jwt")
		}
		return nil
	default:
		return fmt.Errorf("AUTH_MODE must be development or jwt")
	}
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func integer(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
