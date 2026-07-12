"""Static safety contracts for the Tencent Cloud trial deployment."""
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DEPLOY = ROOT / "deploy/tencent"


class TencentDeploymentContractTest(unittest.TestCase):
    def test_local_credential_file_is_ignored(self) -> None:
        patterns = (ROOT / ".gitignore").read_text(encoding="utf-8").splitlines()
        self.assertIn("/docs/tencent.txt", patterns)

    def test_api_is_bound_only_to_the_loopback_trial_port(self) -> None:
        environment = (DEPLOY / "api.env.example").read_text(encoding="utf-8")
        self.assertRegex(environment, r"(?m)^HTTP_ADDRESS=127\.0\.0\.1:18080$")
        self.assertNotRegex(environment, r"(?m)^HTTP_ADDRESS=(?:0\.0\.0\.0|\[?::\]?):18080$")

    def test_scanner_workspace_allowlist_defaults_to_deny_all(self) -> None:
        environment = (DEPLOY / "api.env.example").read_text(encoding="utf-8")
        self.assertRegex(environment, r"(?m)^SCANNER_ALLOWED_ROOTS=$")
        self.assertNotRegex(environment, r"(?m)^SCANNER_ALLOWED_ROOTS=(?:/|[A-Za-z]:[\\/])$")

    def test_nginx_protects_both_trial_locations_with_basic_auth(self) -> None:
        nginx = (DEPLOY / "nginx-bizdevops.conf").read_text(encoding="utf-8")
        blocks = re.findall(r"location\s+\^~\s+(/bizdevops/(?:api/)?)\s*\{(.*?)\n\}", nginx, re.DOTALL)
        self.assertEqual({path for path, _ in blocks}, {"/bizdevops/", "/bizdevops/api/"})
        for path, body in blocks:
            with self.subTest(path=path):
                self.assertIn('auth_basic "BizDevOps trial";', body)
                self.assertIn("auth_basic_user_file /etc/nginx/.htpasswd-bizdevops;", body)

    def test_trial_runtime_requires_jwt_configuration(self) -> None:
        environment = (DEPLOY / "api.env.example").read_text(encoding="utf-8")
        self.assertRegex(environment, r"(?m)^AUTH_MODE=jwt$")
        self.assertNotRegex(environment, r"(?m)^AUTH_MODE=development$")
        self.assertRegex(environment, r"(?m)^AUTH_JWT_ISSUER=replace-me$")
        self.assertRegex(environment, r"(?m)^AUTH_JWT_AUDIENCE=replace-me$")
        self.assertRegex(environment, r"(?m)^AUTH_JWT_SIGNING_KEY=replace-me$")
        self.assertRegex(environment, r"(?m)^AUTH_JWT_TOKEN_TTL=15m$")
        self.assertRegex(environment, r"(?m)^AUTH_COOKIE_NAME=bizdevops_session$")
        self.assertRegex(environment, r"(?m)^AUTH_COOKIE_SECURE=true$")
        self.assertRegex(environment, r"(?m)^AUTH_COOKIE_PATH=/bizdevops/$")

    def test_deployment_has_no_development_user_dependency(self) -> None:
        combined = "\n".join(path.read_text(encoding="utf-8") for path in DEPLOY.iterdir() if path.is_file())
        self.assertNotIn("VITE_DEV_USER_ID", combined)
        self.assertNotIn("X-Dev-User-ID", combined)

    def test_login_rate_limit_is_declared_in_http_context_and_applied_exactly(self) -> None:
        http_config = (DEPLOY / "nginx-bizdevops-http.conf").read_text(encoding="utf-8")
        nginx = (DEPLOY / "nginx-bizdevops.conf").read_text(encoding="utf-8")
        self.assertRegex(
            http_config,
            r"(?m)^limit_req_zone \$binary_remote_addr zone=bizdevops_login:10m rate=5r/m;$",
        )
        login = re.search(
            r"location\s+=\s+/bizdevops/api/v1/auth/login\s*\{(.*?)\n\}",
            nginx,
            re.DOTALL,
        )
        self.assertIsNotNone(login)
        assert login is not None
        self.assertIn("limit_req zone=bizdevops_login burst=5 nodelay;", login.group(1))
        self.assertIn('auth_basic "BizDevOps trial";', login.group(1))

    def test_basic_auth_removal_is_gated_by_tls_rate_limit_and_jwt_acceptance(self) -> None:
        readme = (DEPLOY / "README.md").read_text(encoding="utf-8")
        for condition in ("HTTPS/TLS", "login rate limit", "JWT acceptance"):
            with self.subTest(condition=condition):
                self.assertIn(condition, readme)
        self.assertIn("keep Basic Auth", readme)
        self.assertIn("rollback", readme.lower())

    def test_secure_cookie_avoids_basic_bearer_collision_but_http_blocks_switch(self) -> None:
        readme = (DEPLOY / "README.md").read_text(encoding="utf-8")
        self.assertIn("HttpOnly", readme)
        self.assertIn("SameSite=Strict", readme)
        self.assertIn("Secure", readme)
        self.assertRegex(readme, r"must not switch\s+the\s+public trial to JWT")
        self.assertIn("loopback", readme)

    def test_frontend_does_not_persist_tokens_in_web_storage(self) -> None:
        source = "\n".join(
            path.read_text(encoding="utf-8")
            for path in (ROOT / "apps/web/src").rglob("*")
            if path.suffix in {".ts", ".tsx"} and not path.name.endswith(".test.ts") and not path.name.endswith(".test.tsx")
        )
        self.assertNotRegex(source, r"(?i)(?:localStorage|sessionStorage).{0,120}(?:token|jwt)")
        self.assertNotRegex(source, r"(?i)(?:token|jwt).{0,120}(?:localStorage|sessionStorage)")

    def test_systemd_units_run_as_the_dedicated_account(self) -> None:
        for name in ("bizdevops-api.service", "bizdevops-worker@.service"):
            unit = (DEPLOY / name).read_text(encoding="utf-8")
            with self.subTest(unit=name):
                self.assertRegex(unit, r"(?m)^User=bizdevops$")
                self.assertRegex(unit, r"(?m)^Group=bizdevops$")
                self.assertNotRegex(unit, r"(?m)^User=root$")

    def test_templates_do_not_contain_committed_secret_values(self) -> None:
        combined = "\n".join(path.read_text(encoding="utf-8") for path in DEPLOY.iterdir() if path.is_file())
        self.assertNotRegex(combined, r"(?i)(?:password|passwd|token|signing_key)\s*=\s*(?!replace-me(?:\s|$))\S+")
        self.assertNotIn("BEGIN OPENSSH PRIVATE KEY", combined)
        self.assertNotIn("docs/tencent.txt", combined)

    def test_frontend_and_proxy_share_the_trial_base_path(self) -> None:
        vite = (ROOT / "apps/web/vite.config.ts").read_text(encoding="utf-8")
        api_client = (ROOT / "apps/web/src/api/client.ts").read_text(encoding="utf-8")
        client = (ROOT / "apps/web/src/api/client.test.ts").read_text(encoding="utf-8")
        nginx = (DEPLOY / "nginx-bizdevops.conf").read_text(encoding="utf-8")
        self.assertIn("VITE_BASE_PATH", vite)
        self.assertNotIn("VITE_DEV_USER_ID", api_client)
        self.assertIn("resolveApiBaseUrl('/bizdevops/')", client)
        self.assertIn("location ^~ /bizdevops/api/", nginx)
        self.assertIn("location ^~ /bizdevops/", nginx)
        self.assertIn("root /var/www;", nginx)
        self.assertIn("try_files $uri $uri/ /bizdevops/index.html;", nginx)
        self.assertNotIn("alias /var/www/bizdevops/;", nginx)


if __name__ == "__main__":
    unittest.main()
