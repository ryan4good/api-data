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

    def test_nginx_protects_both_trial_locations_with_basic_auth(self) -> None:
        nginx = (DEPLOY / "nginx-bizdevops.conf").read_text(encoding="utf-8")
        blocks = re.findall(r"location\s+\^~\s+(/bizdevops/(?:api/)?)\s*\{(.*?)\n\}", nginx, re.DOTALL)
        self.assertEqual({path for path, _ in blocks}, {"/bizdevops/", "/bizdevops/api/"})
        for path, body in blocks:
            with self.subTest(path=path):
                self.assertIn('auth_basic "BizDevOps trial";', body)
                self.assertIn("auth_basic_user_file /etc/nginx/.htpasswd-bizdevops;", body)

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
        client = (ROOT / "apps/web/src/api/client.test.ts").read_text(encoding="utf-8")
        nginx = (DEPLOY / "nginx-bizdevops.conf").read_text(encoding="utf-8")
        self.assertIn("VITE_BASE_PATH", vite)
        self.assertIn("resolveApiBaseUrl('/bizdevops/')", client)
        self.assertIn("location ^~ /bizdevops/api/", nginx)
        self.assertIn("location ^~ /bizdevops/", nginx)
        self.assertIn("root /var/www;", nginx)
        self.assertIn("try_files $uri $uri/ /bizdevops/index.html;", nginx)
        self.assertNotIn("alias /var/www/bizdevops/;", nginx)


if __name__ == "__main__":
    unittest.main()
