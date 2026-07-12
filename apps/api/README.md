# BizDevOps API

Go HTTP API for the API data platform. It uses the standard library HTTP
server and keeps each bounded context under `internal/modules`.

## Run

```sh
go run ./cmd/server
```

Configuration is supplied through environment variables documented in
`.env.example`. Production mode uses JWT by default and requires issuer,
audience and signing key values. Local development must explicitly set
`AUTH_MODE=development` before using `X-Dev-User-ID`.

JWT mode exposes `POST /api/v1/auth/login`, `GET /api/v1/auth/me`, and
`POST /api/v1/auth/logout`. Login returns a Bearer access token and also sets
an HttpOnly, SameSite=Strict session cookie for browser clients. Keep
`AUTH_COOKIE_SECURE=true` outside local HTTP development and configure token
lifetime with `AUTH_JWT_TOKEN_TTL` (default `1h`).

## Initial endpoints

- `GET /healthz`
- `GET /readyz` (requires `MYSQL_DSN` to be configured)
- `GET /api/v1/status`
- `GET /api/v1/me`
- System list/detail and member management under `/api/v1/systems`.

## Code scanner

```sh
go run ./cmd/scanner <repository-root>
```

The command emits stable JSON operations with method, path, handler and source
location. The importer domain package accepts Scenario Bundle 1.0 and Postman
Collection 2.1 JSON; its HTTP upload workflow is the next integration step.
