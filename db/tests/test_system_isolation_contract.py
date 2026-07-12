"""Static contract tests for the MySQL system-isolation development slice."""
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


class SystemIsolationContractTest(unittest.TestCase):
    def test_member_role_enum_is_the_platform_contract(self) -> None:
        migration = (ROOT / "db/migrations/000001_initial.up.sql").read_text(encoding="utf-8")
        body = re.search(
            r"CREATE TABLE system_members\s*\((.*?)\) ENGINE=InnoDB",
            migration,
            re.IGNORECASE | re.DOTALL,
        )
        self.assertIsNotNone(body)
        role = re.search(r"\brole\s+ENUM\(([^)]+)\)", body.group(1), re.IGNORECASE)
        self.assertIsNotNone(role)
        self.assertEqual(
            [item.strip(" '\"") for item in role.group(1).split(",")],
            ["owner", "maintainer", "reviewer", "runner", "viewer"],
        )

    def test_seed_has_oms_wms_users_and_distinct_memberships(self) -> None:
        seed = (ROOT / "db/seeds/000001_development.sql").read_text(encoding="utf-8")
        for value in ("alice@example.test", "bob@example.test", "outsider@example.test", "'oms'", "'wms'"):
            self.assertIn(value, seed)
        self.assertRegex(seed, r"(?is)INSERT INTO system_members.*?'owner'.*?'viewer'")

    def test_application_system_queries_always_join_membership(self) -> None:
        query = (ROOT / "db/queries/business_systems.scoped.sql").read_text(encoding="utf-8")
        normalized = re.sub(r"\s+", " ", query.lower())
        self.assertIn("join system_members", normalized)
        self.assertRegex(normalized, r"system_members\s+as\s+sm")
        self.assertIn("sm.system_id = bs.id", normalized)
        self.assertIn("sm.user_id = ?", normalized)
        self.assertIn("bs.id = ?", normalized)
        self.assertNotRegex(normalized, r"select\s+.*\s+from\s+business_systems\s+as\s+bs\s*;")

    def test_compose_pins_mysql_8_and_has_healthcheck(self) -> None:
        compose = (ROOT / "deploy/dev/compose.yaml").read_text(encoding="utf-8")
        self.assertRegex(compose, r"image:\s*mysql:8\.0(?:\.\d+)?")
        self.assertIn("healthcheck:", compose)
        self.assertIn("000001_initial.up.sql", compose)
        self.assertIn("000001_development.sql", compose)

    def test_ci_runner_exercises_up_seed_scope_and_down(self) -> None:
        runner = (ROOT / "db/tests/run_mysql_integration.py").read_text(encoding="utf-8")
        for marker in ("000001_initial.up.sql", "000001_development.sql", "business_systems.scoped.sql", "000001_initial.down.sql"):
            self.assertIn(marker, runner)

    def test_runner_supports_secret_safe_environment_connection(self) -> None:
        runner = (ROOT / "db/tests/run_mysql_integration.py").read_text(encoding="utf-8")
        for variable in ("MYSQL_HOST", "MYSQL_PORT", "MYSQL_USER", "MYSQL_PASSWORD"):
            self.assertIn(variable, runner)
        self.assertIn('DATABASE = "bizdevops_contract_test"', runner)
        self.assertNotRegex(runner, r"(?m)^ROOT_PASSWORD\s*=")
        self.assertNotIn('f"-p{', runner)
        self.assertIn('env["MYSQL_PWD"]', runner)

    def test_runner_keeps_client_warnings_out_of_query_results(self) -> None:
        runner = (ROOT / "db/tests/run_mysql_integration.py").read_text(encoding="utf-8")
        self.assertIn("stderr=subprocess.PIPE", runner)
        self.assertNotIn("stderr=subprocess.STDOUT", runner)

    def test_local_server_runner_is_documented_without_a_password_value(self) -> None:
        readme = (ROOT / "deploy/dev/README.md").read_text(encoding="utf-8")
        self.assertIn("--environment", readme)
        for variable in ("MYSQL_HOST", "MYSQL_PORT", "MYSQL_USER", "MYSQL_PASSWORD"):
            self.assertIn(variable, readme)
        self.assertNotRegex(readme, r"MYSQL_PASSWORD\s*=\s*\S+")


if __name__ == "__main__":
    unittest.main()
