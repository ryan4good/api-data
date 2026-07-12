"""Executable structural contract for the MySQL 8.0 initial migration."""
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]
SQL = (ROOT / "db/migrations/000001_initial.up.sql").read_text(encoding="utf-8")

REQUIRED_TABLES = {
    "users", "business_systems", "system_members", "code_sources", "scan_runs",
    "api_operations", "scenario_discoveries", "scenario_candidates",
    "scenario_imports", "scenarios", "scenario_versions",
    "scenario_steps", "environments", "connectors", "scenario_runs",
    "scenario_step_runs", "scenario_assertion_results", "audit_logs",
}
SYSTEM_SCOPED = REQUIRED_TABLES - {"users", "business_systems"}


def table_body(name: str) -> str:
    match = re.search(
        rf"CREATE TABLE {re.escape(name)}\s*\((.*?)\) ENGINE=InnoDB",
        SQL,
        flags=re.IGNORECASE | re.DOTALL,
    )
    assert match, f"missing table: {name}"
    return match.group(1)


def main() -> None:
    assert "MySQL 8.0+" in SQL
    assert "DEFAULT CHARSET=utf8mb4" in SQL
    for table in REQUIRED_TABLES:
        body = table_body(table)
        assert re.search(r"\bPRIMARY KEY\s*\(", body, re.IGNORECASE), f"{table}: missing PK"
        if table in SYSTEM_SCOPED:
            assert re.search(r"\bsystem_id CHAR\(36\) NOT NULL", body, re.IGNORECASE), (
                f"{table}: missing non-null system_id"
            )
            assert re.search(r"(?:KEY|PRIMARY KEY).*\bsystem_id\b", body, re.IGNORECASE), (
                f"{table}: missing system-scoped index"
            )

    # In MySQL, ON DELETE SET NULL applies to every child column. A composite
    # tenant FK such as (system_id, optional_id) cannot use it because system_id
    # is deliberately NOT NULL.
    invalid = re.findall(
        r"FOREIGN KEY\s*\(\s*system_id\s*,[^)]*\).*?ON DELETE SET NULL",
        SQL,
        flags=re.IGNORECASE,
    )
    assert not invalid, "composite tenant foreign keys must not use ON DELETE SET NULL"

    member_body = table_body("system_members")
    role_enum = re.search(r"role\s+ENUM\(([^)]+)\)", member_body, re.IGNORECASE)
    assert role_enum, "system_members: missing role enum"
    roles = {value.strip(" '\"").lower() for value in role_enum.group(1).split(",")}
    expected_roles = {"owner", "maintainer", "reviewer", "runner", "viewer"}
    assert roles == expected_roles, (
        f"system_members: expected roles {sorted(expected_roles)}, got {sorted(roles)}"
    )

    # Every tenant-scoped relation must include system_id on both sides.
    scoped_relations = re.findall(
        r"FOREIGN KEY\s*\(\s*system_id\s*,\s*([^)]+)\)\s*"
        r"REFERENCES\s+\w+\s*\(\s*system_id\s*,\s*([^)]+)\)",
        SQL,
        flags=re.IGNORECASE,
    )
    assert scoped_relations, "missing composite tenant foreign keys"

    discovery_body = table_body("scenario_discoveries")
    assert re.search(
        r"FOREIGN KEY\s*\(\s*system_id\s*,\s*code_source_id\s*\)\s*"
        r"REFERENCES\s+code_sources\s*\(\s*system_id\s*,\s*id\s*\)",
        discovery_body,
        flags=re.IGNORECASE,
    ), "scenario_discoveries: code source relation must be tenant scoped"

    candidate_body = table_body("scenario_candidates")
    assert re.search(
        r"FOREIGN KEY\s*\(\s*system_id\s*,\s*discovery_id\s*\)\s*"
        r"REFERENCES\s+scenario_discoveries\s*\(\s*system_id\s*,\s*id\s*\)",
        candidate_body,
        flags=re.IGNORECASE,
    ), "scenario_candidates: discovery relation must be tenant scoped"
    print(f"PASS: {len(REQUIRED_TABLES)} tables; {len(scoped_relations)} scoped foreign keys")


if __name__ == "__main__":
    main()
