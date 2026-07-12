# Tencent Cloud trial deployment

The trial layout avoids Docker and coexists with the existing Nginx site:

- UI: `/bizdevops/`
- API upstream: `127.0.0.1:18080`
- API service: `bizdevops-api.service`
- Worker template: `bizdevops-worker@<system-id>.service`
- Files: `/opt/bizdevops`, `/var/www/bizdevops`, `/etc/bizdevops/api.env`

The trial uses development identity only behind Nginx Basic Auth. Do not remove
Basic Auth until a production login/token flow is configured. Database, JWT and
Basic Auth secrets are generated on the server and are not committed.

Build the web app with `VITE_BASE_PATH=/bizdevops/`. Apply every `*.up.sql`
migration in lexical order. Validate with `nginx -t`, local API health, the
proxied health endpoint, and an authorized management overview request.
