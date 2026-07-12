"""Run migration and system-isolation checks with no Python dependencies.

The integration target can be an ephemeral MySQL 8.0 Docker container or an
existing server configured entirely through MYSQL_* environment variables.
"""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time
import uuid
from collections.abc import Callable


ROOT = Path(__file__).resolve().parents[2]
UPS = sorted((ROOT / "db/migrations").glob("*.up.sql"))
DOWNS = sorted((ROOT / "db/migrations").glob("*.down.sql"), reverse=True)
SEED = ROOT / "db/seeds/000001_development.sql"
SCOPED = ROOT / "db/queries/business_systems.scoped.sql"
IMAGE = "mysql:8.0.36"
DATABASE = "bizdevops_contract_test"


def run(
    command: list[str],
    *,
    input_text: str | None = None,
    check: bool = True,
    env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=ROOT,
        input=input_text,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=check,
        env=env,
    )


def static_contracts() -> None:
    run([sys.executable, str(ROOT / "db/tests/validate_migration.py")])
    run([sys.executable, "-m", "unittest", "db.tests.test_system_isolation_contract", "-v"])
    print("PASS: static migration and isolation contracts")


def docker_ready() -> bool:
    if not shutil.which("docker"):
        return False
    return run(["docker", "info"], check=False).returncode == 0


def docker_mysql(container: str, password: str, sql: str, *, database: bool = True) -> str:
    command = [
        "docker", "exec", "-i", "-e", f"MYSQL_PWD={password}", container,
        "mysql", "-uroot", "--batch", "--skip-column-names",
    ]
    if database:
        command.append(DATABASE)
    result = run(command, input_text=sql)
    return result.stdout.strip()


def wait_for_mysql(container: str, password: str) -> None:
    for _ in range(60):
        result = run(
            ["docker", "exec", "-e", f"MYSQL_PWD={password}", container,
             "mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "--silent"],
            check=False,
        )
        if result.returncode == 0:
            return
        time.sleep(1)
    logs = run(["docker", "logs", container], check=False).stdout
    raise RuntimeError(f"MySQL did not become healthy:\n{logs}")


def named_query(name: str) -> str:
    sql = SCOPED.read_text(encoding="utf-8")
    match = re.search(rf"-- name: {re.escape(name)}\s+(.*?;)", sql, re.DOTALL)
    if not match:
        raise AssertionError(f"missing query: {name}")
    return match.group(1)


def bind(query: str, *values: str) -> str:
    for value in values:
        escaped = value.replace("'", "''")
        query = query.replace("?", f"'{escaped}'", 1)
    if "?" in query:
        raise AssertionError("not all query parameters were bound")
    return query


def assert_equal(actual: str, expected: str, message: str) -> None:
    if actual != expected:
        raise AssertionError(f"{message}: expected {expected!r}, got {actual!r}")


def exercise_contracts(mysql: Callable[[str, bool], str], server_label: str) -> None:
    for migration in UPS:
        mysql(migration.read_text(encoding="utf-8"), True)
    mysql(SEED.read_text(encoding="utf-8"), True)

    list_query = named_query("ListAuthorizedBusinessSystems")
    detail_query = named_query("GetAuthorizedBusinessSystem")
    alice = "10000000-0000-4000-8000-000000000001"
    bob = "10000000-0000-4000-8000-000000000002"
    outsider = "10000000-0000-4000-8000-000000000004"
    oms = "20000000-0000-4000-8000-000000000001"
    wms = "20000000-0000-4000-8000-000000000002"

    assert_equal(mysql(f"SELECT system_key FROM ({bind(list_query, alice)[:-1]}) AS scoped;", True), "oms", "Alice must only see OMS")
    assert_equal(mysql(f"SELECT system_key FROM ({bind(list_query, bob)[:-1]}) AS scoped;", True), "wms", "Bob must only see WMS")
    assert_equal(mysql(f"SELECT COUNT(*) FROM ({bind(list_query, outsider)[:-1]}) AS scoped;", True), "0", "Outsider must see no systems")
    assert_equal(mysql(f"SELECT COUNT(*) FROM ({bind(detail_query, alice, wms)[:-1]}) AS scoped;", True), "0", "Alice must not read WMS by ID")
    assert_equal(mysql(f"SELECT COUNT(*) FROM ({bind(detail_query, alice, oms)[:-1]}) AS scoped;", True), "1", "Alice must read OMS by ID")

    enum = mysql("SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'system_members' AND COLUMN_NAME = 'role';", True)
    assert_equal(enum, "enum('owner','maintainer','reviewer','runner','viewer')", "member roles")

    for migration in DOWNS:
        mysql(migration.read_text(encoding="utf-8"), True)
    assert_equal(mysql("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE();", True), "0", "down migration must remove all tables")
    print(f"PASS: {server_label} up, seed, scoped reads, role enum, and down")


def docker_integration_contracts() -> None:
    container = f"bizdevops-mysql-test-{uuid.uuid4().hex[:8]}"
    password = secrets.token_urlsafe(24)
    run([
        "docker", "run", "--detach", "--name", container,
        "-e", f"MYSQL_ROOT_PASSWORD={password}",
        "-e", f"MYSQL_DATABASE={DATABASE}", IMAGE,
    ])
    try:
        wait_for_mysql(container, password)
        exercise_contracts(
            lambda sql, database=True: docker_mysql(container, password, sql, database=database),
            "MySQL 8.0",
        )
    finally:
        run(["docker", "rm", "--force", container], check=False)


def environment_integration_contracts() -> None:
    missing = [name for name in ("MYSQL_HOST", "MYSQL_PORT", "MYSQL_USER", "MYSQL_PASSWORD") if not os.environ.get(name)]
    if missing:
        raise SystemExit(f"Environment connection requires: {', '.join(missing)}")
    client = shutil.which("mysql")
    if not client:
        raise SystemExit("mysql client is required for environment connection")

    env = os.environ.copy()
    env["MYSQL_PWD"] = env.pop("MYSQL_PASSWORD")
    base = [
        client,
        "--host", env["MYSQL_HOST"],
        "--port", env["MYSQL_PORT"],
        "--user", env["MYSQL_USER"],
        "--batch", "--skip-column-names",
    ]

    def local_mysql(sql: str, database: bool = True) -> str:
        command = [*base]
        if database:
            command.append(DATABASE)
        return run(command, input_text=sql, env=env).stdout.strip()

    local_mysql(f"DROP DATABASE IF EXISTS `{DATABASE}`; CREATE DATABASE `{DATABASE}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", False)
    try:
        version = local_mysql("SELECT VERSION();", False).splitlines()[0]
        exercise_contracts(local_mysql, f"existing server {version}")
    finally:
        local_mysql(f"DROP DATABASE IF EXISTS `{DATABASE}`;", False)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--static-only", action="store_true")
    parser.add_argument("--require-docker", action="store_true")
    parser.add_argument("--environment", action="store_true", help="use MYSQL_HOST/PORT/USER/PASSWORD")
    args = parser.parse_args()

    static_contracts()
    if args.static_only:
        return
    if args.environment:
        environment_integration_contracts()
        return
    if not docker_ready():
        if args.require_docker:
            raise SystemExit("Docker is required but unavailable")
        print("SKIP: Docker unavailable; static contracts passed. Use --require-docker in CI.")
        return
    docker_integration_contracts()


if __name__ == "__main__":
    main()
