# Authentication: JWT login deployment contract

## Scope

This change moves the Tencent Cloud deployment template from development identity
to the formal JWT login boundary. It adds required issuer, audience, signing-key,
15-minute token-TTL, and Secure-cookie settings; removes deployment reliance on a
development user header; and adds a per-IP Nginx login limit.

Application login behavior and browser session handling are implemented and
tested in their respective API and Web changes. This record covers only the
deployment and static safety contract.

## TDD evidence

The deployment contract was extended first. Its Red run failed because the
template still used `AUTH_MODE=development`, JWT settings and the Nginx rate-limit
zone were absent, the frontend contract still allowed `VITE_DEV_USER_ID`, and the
deployment guide lacked gated switch/rollback instructions. The templates and
instructions were then changed to satisfy those observed failures.

## Security decisions

- JWT runtime values are required placeholders; the real signing key is generated
  on the server from at least 32 cryptographically random bytes and stored only in
  root-owned mode-`0600` environment configuration.
- Login passwords are persisted only as password hashes. Plaintext passwords,
  signing keys, database credentials, and tokens must not enter the repository or
  deployment logs.
- `/bizdevops/api/v1/auth/login` has an exact Nginx location with a five-requests-
  per-minute zone and a burst of five. The zone declaration is a separate file
  intentionally installed in the Nginx `http` context; the server snippet only
  consumes it, so the final layout remains valid under `nginx -t`.
- The trial endpoint is still HTTP on port 8080. Therefore Basic Auth remains on
  the UI, API prefix, and exact login location even after JWT acceptance. Removing
  it requires HTTPS/TLS, verified login rate limiting, the full JWT authorization
  acceptance suite, and a secret-free log review.
- The browser uses an `HttpOnly`, `SameSite=Strict`, `Secure` cookie named
  `bizdevops_session` with path `/bizdevops/` and never persists tokens in Web Storage. This
  avoids the `Authorization` collision with Nginx Basic Auth while retaining
  Bearer compatibility for non-browser clients, without sending the JWT to
  unrelated applications sharing the `:8080` origin.
- Public rollout remains blocked until JWT passes over loopback and TLS is active:
  the Secure cookie will not be sent over HTTP, and weakening it would expose
  credentials and tokens. Under HTTPS, JWT acceptance runs with Basic Auth still
  enabled; only after success may Basic Auth be removed in a later atomic change.

## Acceptance and rollback

Acceptance includes valid and invalid login, expired/malformed tokens, role and
cross-system denial over loopback; unchanged API/UI behavior for the old public
release behind Basic Auth; then, after TLS, public HTTPS JWT-cookie acceptance
behind Basic Auth and rate-limit rejection. It also includes `nginx -t`, loopback-only
API binding, and regression checks for existing services.
Rollback restores the prior application, web root, environment, and both Nginx
files, with `nginx -t` required before reload. Basic Auth is retained throughout
the current non-TLS rollout.

## Integrated application result

- API login uses bcrypt and a dummy comparison for unknown accounts, signs a
  short-lived HS256 JWT, and returns one fixed 401 contract for unknown email,
  wrong password, missing hash, or disabled user.
- The browser receives an HttpOnly/SameSite=Strict cookie; the middleware also
  accepts Bearer JWT for non-browser clients and rechecks that the database user
  remains active on every authenticated request.
- Login and logout explicitly remain reachable when an expired or key-rotation
  stale cookie is present. A new Red test reproduced the pre-handler 401; the
  identity boundary now bypasses stale-token validation only for the exact
  login/logout recovery paths. `/auth/me` and business routes remain protected.
- The Web app adds `/login`, an authentication route boundary, in-memory user
  state, `/auth/me` refresh verification, and server-side logout. It never stores
  or sends the token from browser JavaScript.

## Real MariaDB staging acceptance

The new Linux amd64 API was started once on remote loopback `127.0.0.1:18081`
without changing the public `18080` service. Against the real trial MariaDB it
verified wrong-password 401, successful login, cookie and Bearer `/auth/me`,
cookie-authenticated management overview, rejection of the development header,
stale-cookie logout recovery, and logout invalidation. The staging unit, env,
cookie jar, binary release, and temporary password hash were then removed or
restored. Public deployment remains intentionally blocked on trusted TLS.
