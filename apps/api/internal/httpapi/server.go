package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"bizdevops/apps/api/internal/config"
	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
	"bizdevops/apps/api/internal/modules/catalog"
	"bizdevops/apps/api/internal/modules/codesource"
	"bizdevops/apps/api/internal/modules/connector"
	"bizdevops/apps/api/internal/modules/discovery"
	"bizdevops/apps/api/internal/modules/environment"
	"bizdevops/apps/api/internal/modules/execution"
	"bizdevops/apps/api/internal/modules/importer"
	"bizdevops/apps/api/internal/modules/management"
	"bizdevops/apps/api/internal/modules/runrecord"
	"bizdevops/apps/api/internal/modules/scanner"
	"bizdevops/apps/api/internal/modules/scenario"
	"bizdevops/apps/api/internal/modules/system"
)

type Dependencies struct {
	AuthUsers       access.UserRepository
	Systems         system.Repository
	Scans           scanner.Repository
	Imports         importer.Repository
	Management      management.Repository
	Discoveries     discovery.Repository
	Scenarios       scenario.Repository
	Executions      execution.Repository
	AsyncExecutions execution.AsyncRepository
	Environments    environment.Repository
	CodeSources     codesource.Repository
	Steps           execution.StepProvider
	Executor        execution.StepExecutor
}

type emptyStepProvider struct{}

func (emptyStepProvider) Steps(context.Context, string, string) ([]execution.Step, error) {
	return []execution.Step{}, nil
}

type statusResponse struct {
	Service         string   `json:"service"`
	Environment     string   `json:"environment"`
	Status          string   `json:"status"`
	MySQLConfigured bool     `json:"mysqlConfigured"`
	Modules         []string `json:"modules"`
	Time            string   `json:"time"`
}

func New(cfg config.Config, logger *slog.Logger) http.Handler {
	return NewWithDependencies(cfg, logger, Dependencies{
		Systems: system.NewMemoryRepository(nil, nil), Scans: scanner.NewMemoryRepository(), Imports: importer.NewMemoryRepository(),
		Management:  management.NewMemoryRepository(nil, nil, nil),
		Discoveries: discovery.NewMemoryRepository(), Executions: execution.NewMemoryRepository(),
		Scenarios:    scenario.NewMemoryRepository(nil),
		Environments: environment.NewMemoryRepository(),
		CodeSources:  codesource.NewMemoryRepository(),
	})
}

func NewWithSystemRepository(cfg config.Config, logger *slog.Logger, systemRepository system.Repository) http.Handler {
	return NewWithRepositories(cfg, logger, systemRepository, scanner.NewMemoryRepository(), importer.NewMemoryRepository())
}

func NewWithRepositories(
	cfg config.Config,
	logger *slog.Logger,
	systemRepository system.Repository,
	scannerRepository scanner.Repository,
	importRepository importer.Repository,
) http.Handler {
	return NewWithDependencies(cfg, logger, Dependencies{
		Systems: systemRepository, Scans: scannerRepository, Imports: importRepository,
		Management:  management.NewMemoryRepository(nil, nil, nil),
		Discoveries: discovery.NewMemoryRepository(), Executions: execution.NewMemoryRepository(),
		Scenarios:    scenario.NewMemoryRepository(nil),
		Environments: environment.NewMemoryRepository(),
		CodeSources:  codesource.NewMemoryRepository(),
	})
}

func NewWithDependencies(cfg config.Config, logger *slog.Logger, dependencies Dependencies) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()
	identityUsers := dependencies.AuthUsers
	if dependencies.AuthUsers == nil {
		dependencies.AuthUsers = access.NewMemoryUserRepository(nil)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		httpresponse.Success(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		if cfg.MySQL.DSN == "" {
			httpresponse.Failure(w, http.StatusServiceUnavailable, "mysql_not_configured", "MySQL is not configured", nil)
			return
		}
		httpresponse.Success(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		httpresponse.Success(w, http.StatusOK, statusResponse{
			Service: cfg.ServiceName, Environment: cfg.Environment, Status: "ready",
			MySQLConfigured: cfg.MySQL.DSN != "", Modules: registeredModules(), Time: time.Now().UTC().Format(time.RFC3339),
		})
	})

	if dependencies.Steps == nil {
		dependencies.Steps = emptyStepProvider{}
	}
	if dependencies.Executor == nil {
		dependencies.Executor = execution.DisabledExecutor{}
	}
	if dependencies.CodeSources == nil {
		dependencies.CodeSources = codesource.NewMemoryRepository()
	}

	system.Register(mux, dependencies.Systems)
	access.Register(mux)
	access.RegisterAuthentication(mux, access.NewAuthenticationService(dependencies.AuthUsers, access.TokenOptions{
		Issuer: cfg.Auth.JWTIssuer, Audience: cfg.Auth.JWTAudience, SigningKey: cfg.Auth.JWTSigningKey, TTL: cfg.Auth.JWTTokenTTL,
	}), dependencies.AuthUsers, access.AuthenticationHTTPOptions{
		CookieName: cfg.Auth.CookieName, CookieSecure: cfg.Auth.CookieSecure, CookiePath: cfg.Auth.CookiePath,
	})
	scanner.Register(mux)
	scanner.RegisterSystemRoutes(mux, scanner.NewService(dependencies.Scans, dependencies.CodeSources, scanner.AnalyzerFunc(scanner.Analyze), cfg.Scanner.AllowedRoots...), dependencies.Systems)
	importer.RegisterSystemRoutes(mux, importer.NewService(dependencies.Imports, importer.ServiceOptions{}), dependencies.Systems)
	management.Register(mux, dependencies.Management, logger)
	discovery.RegisterSystemRoutes(mux, discovery.NewService(dependencies.Discoveries), dependencies.Systems)
	scenario.RegisterSystemRoutes(mux, scenario.NewService(dependencies.Scenarios), dependencies.Systems)
	executionService := execution.NewService(dependencies.Executions, dependencies.Steps, dependencies.Executor, execution.ServiceOptions{})
	execution.RegisterSystemRoutes(mux, executionService, dependencies.Systems)
	asyncRepository := dependencies.AsyncExecutions
	if asyncRepository == nil {
		asyncRepository, _ = dependencies.Executions.(execution.AsyncRepository)
	}
	if asyncRepository != nil {
		execution.RegisterAsyncRoutes(mux, execution.NewQueueService(asyncRepository, dependencies.Steps, execution.QueueServiceOptions{}), dependencies.Systems)
	}
	environment.RegisterSystemRoutes(mux, dependencies.Environments, dependencies.Systems)
	codesource.RegisterSystemRoutes(mux, dependencies.CodeSources, dependencies.Systems)
	runrecord.RegisterSystemRoutes(mux, runrecord.NewQueryService(dependencies.Executions), dependencies.Systems)
	catalog.Register(mux)
	discovery.Register(mux)
	scenario.Register(mux)
	execution.Register(mux)
	runrecord.Register(mux)
	connector.Register(mux)

	identity := access.RequestIdentity(cfg.Auth.Mode, access.JWTIdentityOptions{
		Issuer: cfg.Auth.JWTIssuer, Audience: cfg.Auth.JWTAudience, SigningKey: cfg.Auth.JWTSigningKey, CookieName: cfg.Auth.CookieName, Users: identityUsers,
	})
	return requestLogger(logger, identity(mux))
}

func registeredModules() []string {
	return []string{"system", "identity/access", "code-source", "scanner", "catalog", "discovery", "scenario", "execution", "runrecord", "connector"}
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
