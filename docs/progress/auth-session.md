# Authentication Session Progress

## Scope

- Production Bearer JWT authentication.
- Development-only `X-Dev-User-ID` authentication.
- `GET /api/v1/me` identity endpoint.

## TDD log

### RED - authentication contract tests

Status: completed.

Added failing tests for:

- valid JWT actor propagation;
- signature, issuer, audience, expiration, and UUID subject validation;
- production rejection of `X-Dev-User-ID`;
- authentication configuration validation;
- `GET /api/v1/me`.

Command:

```text
go test ./internal/modules/access ./internal/config ./internal/httpapi
```

Observed failures:

```text
no required module provides package github.com/golang-jwt/jwt/v5
cfg.Auth undefined
unknown field Auth in struct literal of type config.Config
```

This is the expected Red state: neither the authentication dependency nor the configuration and middleware contracts existed.

### GREEN

Status: completed.

Implemented:

- HS256 Bearer verification through `github.com/golang-jwt/jwt/v5`;
- signature and algorithm enforcement;
- required issuer, audience, expiration, and UUID subject validation;
- actor propagation through request context;
- secure JWT mode by default, with no default issuer, audience, or signing key;
- development identity only when `AUTH_MODE=development`;
- rejection of `X-Dev-User-ID` in every non-development mode;
- `GET /api/v1/me` with the standard `{ "data": ... }` envelope;
- compatibility bridge so existing protected System handlers accept an actor already authenticated by the production middleware.

Verification:

```text
go test ./internal/modules/access ./internal/config ./internal/httpapi
ok

go test ./...
ok at the authentication Green checkpoint (including cmd/server and internal/modules/system)

go vet ./...
ok at the authentication Green checkpoint
```

Final focused regression after another parallel session entered its own importer Red phase:

```text
go test ./cmd/server ./internal/config ./internal/httpapi ./internal/modules/access ./internal/modules/system
ok

go vet ./cmd/server ./internal/config ./internal/httpapi ./internal/modules/access ./internal/modules/system
ok
```

The later repository-wide command was temporarily blocked only by undefined importer symbols in that parallel session's expected Red tests, not by authentication or System packages.

## Contract decisions

- `AUTH_MODE` supports exactly `jwt` and `development`; its secure default is `jwt`.
- JWT mode requires `AUTH_JWT_ISSUER`, `AUTH_JWT_AUDIENCE`, and `AUTH_JWT_SIGNING_KEY` at startup.
- The signing key has no checked-in or runtime fallback value.
- JWT mode accepts only HS256 and requires a valid signature, matching issuer and audience, an unexpired `exp`, and a UUID `sub`.
- `X-Dev-User-ID` is accepted only in explicit development mode and must itself be a UUID.
- Public health/status routes remain accessible without credentials. Invalid supplied credentials are rejected, while protected routes enforce an authenticated actor.
- Authentication errors are intentionally generic and neither configuration nor request logs include signing keys or tokens.

## Known limitations

- JWT key rotation and JWKS/asymmetric verification are not implemented yet.
- Token revocation and per-user session invalidation are not implemented yet.
- Authorization remains the responsibility of each domain handler after authentication.

## Next handoff

- Set `AUTH_MODE=development` explicitly for local System E2E tests that use `X-Dev-User-ID`.
- For production-like runs, provision all three `AUTH_JWT_*` values through the secret/configuration layer; never add them to source control.
- A later identity-provider session can replace HS256 with JWKS-backed asymmetric keys while preserving the Actor and `/api/v1/me` contracts.
