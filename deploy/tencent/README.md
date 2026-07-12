# Tencent Cloud trial deployment

The trial layout avoids Docker and coexists with the existing Nginx site:

- UI: `/bizdevops/`
- API upstream: `127.0.0.1:18080`
- API service: `bizdevops-api.service`
- Worker template: `bizdevops-worker@<system-id>.service`
- Files: `/opt/bizdevops`, `/var/www/bizdevops`, `/etc/bizdevops/api.env`

## Authentication boundary

The runtime template uses `AUTH_MODE=jwt`; deployment must not build or run the
UI with a development user ID. Provision issuer and audience for this deployment
and generate `AUTH_JWT_SIGNING_KEY` on the server from at least 32 cryptographically
random bytes. Store the environment file as root-owned mode `0600`. Never place a
real database password, signing key, access token, or plaintext login password in
source control, shell history, service unit files, or deployment logs. Login
passwords must be stored only as the password hash format accepted by the API.
The browser session is an `HttpOnly`, `SameSite=Strict`, `Secure` cookie named
`bizdevops_session` with path `/bizdevops/`; browser code must never persist the JWT in
`localStorage` or `sessionStorage`. The API may retain Bearer support for non-browser
clients, but the browser uses the cookie. The scoped path also prevents the JWT
from being sent to unrelated applications sharing the Nginx `:8080` origin.

The current public listener is plain HTTP on port 8080, so keep Basic Auth on
both `/bizdevops/` and `/bizdevops/api/` as a second protection layer after the
JWT login flow is deployed. Basic Auth removal is forbidden until all of these
gates pass:

1. HTTPS/TLS is enabled end to end and HTTP redirects to HTTPS.
2. The login rate limit is installed and verified with rejected excess requests.
3. JWT acceptance covers valid login, invalid password, expired token, malformed
   token, unauthorized role, and cross-system denial.
4. Nginx and API access logs have been checked not to contain passwords or tokens.

The cookie design avoids the Authorization-header collision between Nginx Basic
Auth and API Bearer JWT: Nginx consumes the Basic header while the browser session
travels in `Cookie`. However, a Secure cookie is intentionally not sent over the
current HTTP listener, and HTTP would expose login credentials in transit. Operators
must not switch the public trial to JWT while it is still HTTP. First validate JWT
directly over the loopback upstream, then provision TLS and perform the guarded
switch described below. Never weaken `AUTH_COOKIE_SECURE` or put a token in a URL.

`nginx-bizdevops-http.conf` declares the login rate limit zone and must be loaded
directly in Nginx's `http` context (a top-level file in `/etc/nginx/conf.d/` on
the target host). `nginx-bizdevops.conf` belongs inside the existing trial
`server` and applies that zone to the exact login endpoint. Installing only one
of these snippets is invalid. Run `nginx -t` before every reload.

## Switch and rollback

1. Back up the current environment, Nginx configuration, binaries, and web root.
2. Generate the signing key on the server, create the initial account with a
   password hash (never plaintext), install both Nginx snippets, and set the
   environment file to mode `0600`.
3. In a non-public staging process, restart the JWT API and complete login and
   authorization acceptance directly against the loopback upstream. Keep the
   current Basic-protected public deployment serving its previous release.
4. On the current HTTP listener, stop here and keep Basic Auth. After HTTPS/TLS,
   the login rate limit, loopback JWT acceptance, and log review pass, deploy the
   Secure-cookie flow while retaining Basic Auth and repeat JWT acceptance through
   HTTPS. Only after that succeeds may a later atomic Nginx change remove all three
   `auth_basic` pairs. Run `nginx -t` before each reload.
5. If startup, login, authorization, rate limiting, or regression checks fail,
   rollback the previous binaries, web root, environment, and both Nginx files;
   reload only after `nginx -t`. Keep the database migration if it is backward
   compatible; otherwise use its reviewed down migration before restoring the
   old application.

Build the web app with `VITE_BASE_PATH=/bizdevops/`. Apply every `*.up.sql`
migration in lexical order. Validate local API health, proxied health, login,
authenticated `/api/v1/me`, role denial, cross-system denial, and the pre-existing
services. Do not expose port 18080 beyond loopback.
