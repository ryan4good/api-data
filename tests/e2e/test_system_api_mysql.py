"""End-to-end contract test for the System API backed by local MySQL.

The runner deliberately owns a dedicated ``bizdevops_e2e`` database. It always
stops the API process and drops that database, including when an assertion or
startup step fails. The MySQL password is accepted only through MYSQL_PASSWORD.
"""
from __future__ import annotations

import contextlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request


ROOT = Path(__file__).resolve().parents[2]
DATABASE = "bizdevops_e2e"
MYSQL_HOST = os.environ.get("MYSQL_HOST", "127.0.0.1")
MYSQL_PORT = os.environ.get("MYSQL_PORT", "3306")
MYSQL_USER = os.environ.get("MYSQL_USER", "root")

ALICE = "10000000-0000-4000-8000-000000000001"
BOB = "10000000-0000-4000-8000-000000000002"
VIEWER = "10000000-0000-4000-8000-000000000003"
OUTSIDER = "10000000-0000-4000-8000-000000000004"
MEMBER_TARGET = "40000000-0000-4000-8000-000000000005"
OMS = "20000000-0000-4000-8000-000000000001"
WMS = "20000000-0000-4000-8000-000000000002"
SCENARIO = "60000000-0000-4000-8000-000000000001"
SCENARIO_VERSION = "61000000-0000-4000-8000-000000000001"
SCENARIO_STEP = "62000000-0000-4000-8000-000000000001"
ENVIRONMENT = "63000000-0000-4000-8000-000000000001"
GET_ORDER_OPERATION = "71000000-0000-4000-8000-000000000001"
CREATE_ORDER_OPERATION = "71000000-0000-4000-8000-000000000002"
GET_ORDER_STATUS_OPERATION = "71000000-0000-4000-8000-000000000003"


def required_password() -> str:
    password = os.environ.get("MYSQL_PASSWORD")
    if not password:
        raise RuntimeError("MYSQL_PASSWORD must be set in the process environment")
    return password


def executable(name: str, fallback: Path | None = None) -> str:
    found = shutil.which(name)
    if found:
        return found
    if fallback and fallback.is_file():
        return str(fallback)
    raise RuntimeError(f"required executable is unavailable: {name}")


def mysql(sql: str, password: str, *, database: bool = False) -> str:
    command = [
        executable("mysql"),
        f"--host={MYSQL_HOST}",
        f"--port={MYSQL_PORT}",
        f"--user={MYSQL_USER}",
        "--batch",
        "--skip-column-names",
    ]
    if database:
        command.append(DATABASE)
    environment = os.environ.copy()
    environment["MYSQL_PWD"] = password
    result = subprocess.run(
        command,
        cwd=ROOT,
        env=environment,
        input=sql,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    if result.returncode != 0:
        # mysql output does not contain the password because it is never an argv.
        raise RuntimeError(f"mysql command failed (exit {result.returncode}): {result.stdout.strip()}")
    return result.stdout.strip()


def reserve_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(("127.0.0.1", 0))
        return int(listener.getsockname()[1])


def api_request(base_url: str, path: str, user_id: str, *, body: dict[str, object] | None = None) -> tuple[int, dict]:
    payload = None if body is None else json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        base_url + path,
        data=payload,
        method="GET" if body is None else "POST",
        headers={"X-Dev-User-ID": user_id, "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


class MockBusinessAPIHandler(BaseHTTPRequestHandler):
    def do_POST(self) -> None:  # noqa: N802 - stdlib handler contract
        length = int(self.headers.get("Content-Length", "0"))
        if length:
            self.rfile.read(length)
        payload = json.dumps({"data": {"id": "order-e2e", "state": "created"}}).encode("utf-8")
        self.send_response(201)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, _format: str, *_args: object) -> None:
        return


class SystemAPIHarness:
    def __init__(self) -> None:
        self.password = required_password()
        self.process: subprocess.Popen[str] | None = None
        self.base_url = ""
        self._temporary_directory: tempfile.TemporaryDirectory[str] | None = None
        self._log = None
        self.mock_server: ThreadingHTTPServer | None = None
        self.mock_thread: threading.Thread | None = None
        self.mock_base_url = ""

    def __enter__(self) -> "SystemAPIHarness":
        try:
            self._start_mock_api()
            mysql(f"DROP DATABASE IF EXISTS `{DATABASE}`; CREATE DATABASE `{DATABASE}` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;", self.password)
            mysql((ROOT / "db/migrations/000001_initial.up.sql").read_text(encoding="utf-8"), self.password, database=True)
            mysql((ROOT / "db/seeds/000001_development.sql").read_text(encoding="utf-8"), self.password, database=True)
            request_config = json.dumps({
                "baseURL": self.mock_base_url,
                "method": "POST",
                "path": "/orders",
                "assertions": [{"key": "created", "type": "status", "operator": "eq", "expected": 201}],
            }, separators=(",", ":"))
            mysql(
                "INSERT INTO users (id, email, display_name, platform_role, status) "
                f"VALUES ('{MEMBER_TARGET}', 'e2e.member@example.test', 'E2E Member', 'member', 'active');",
                self.password,
                database=True,
            )
            mysql(
                "INSERT INTO api_operations (id, system_id, operation_key, method, path, content_hash) VALUES "
                f"('{GET_ORDER_OPERATION}', '{OMS}', 'get-order', 'GET', '/orders/{{id}}', '{'1' * 64}'),"
                f"('{CREATE_ORDER_OPERATION}', '{OMS}', 'create-order', 'POST', '/orders', '{'2' * 64}'),"
                f"('{GET_ORDER_STATUS_OPERATION}', '{OMS}', 'get-order-status', 'GET', '/orders/{{id}}/status', '{'3' * 64}');",
                self.password,
                database=True,
            )
            mysql(
                "INSERT INTO scenarios (id, system_id, scenario_key, name, status, created_by) "
                f"VALUES ('{SCENARIO}', '{OMS}', 'e2e-order', 'E2E Order', 'active', '{ALICE}'); "
                "INSERT INTO scenario_versions (id, system_id, scenario_id, version_no, source_type, bundle_document, created_by) "
                f"VALUES ('{SCENARIO_VERSION}', '{OMS}', '{SCENARIO}', 1, 'manual', '{{}}', '{ALICE}'); "
                f"UPDATE scenarios SET current_version_id = '{SCENARIO_VERSION}' WHERE id = '{SCENARIO}'; "
                "INSERT INTO scenario_steps (id, system_id, scenario_version_id, step_key, name, position, step_type, request_config) "
                f"VALUES ('{SCENARIO_STEP}', '{OMS}', '{SCENARIO_VERSION}', 'create-order', 'Create order', 1, 'http', '{request_config}'); "
                "INSERT INTO environments (id, system_id, environment_key, name, status, created_by) "
                f"VALUES ('{ENVIRONMENT}', '{OMS}', 'e2e', 'E2E', 'active', '{ALICE}');",
                self.password,
                database=True,
            )
            self._start_api()
            return self
        except BaseException:
            self.close()
            raise

    def __exit__(self, *_: object) -> None:
        self.close()

    def _start_api(self) -> None:
        go = executable("go", Path(r"C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe"))
        self._temporary_directory = tempfile.TemporaryDirectory(prefix="bizdevops-e2e-")
        executable_name = "bizdevops-api.exe" if os.name == "nt" else "bizdevops-api"
        api_binary = Path(self._temporary_directory.name) / executable_name
        build = subprocess.run(
            [go, "build", "-o", str(api_binary), "./cmd/server"],
            cwd=ROOT / "apps/api",
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
        if build.returncode != 0:
            raise RuntimeError(f"Go API build failed:\n{build.stdout}")

        port = reserve_port()
        self.base_url = f"http://127.0.0.1:{port}"
        environment = os.environ.copy()
        environment.update(
            {
                "APP_ENV": "e2e",
                "AUTH_MODE": "development",
                "CONNECTOR_HTTP_ALLOWED_HOSTS": "127.0.0.1",
                "CONNECTOR_HTTP_ALLOW_PRIVATE": "true",
                "HTTP_ADDRESS": f"127.0.0.1:{port}",
                "MYSQL_DSN": f"{MYSQL_USER}:{self.password}@tcp({MYSQL_HOST}:{MYSQL_PORT})/{DATABASE}?parseTime=true&charset=utf8mb4&loc=UTC",
            }
        )
        self._log = tempfile.TemporaryFile(mode="w+", encoding="utf-8")
        self.process = subprocess.Popen(
            [str(api_binary)],
            cwd=ROOT / "apps/api",
            env=environment,
            stdout=self._log,
            stderr=subprocess.STDOUT,
            text=True,
        )
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            if self.process.poll() is not None:
                break
            try:
                with urllib.request.urlopen(self.base_url + "/healthz", timeout=1) as response:
                    if response.status == 200:
                        return
            except (OSError, urllib.error.URLError):
                time.sleep(0.1)
        self._log.seek(0)
        raise RuntimeError(f"Go API did not become healthy:\n{self._log.read()}")

    def _start_mock_api(self) -> None:
        self.mock_server = ThreadingHTTPServer(("127.0.0.1", 0), MockBusinessAPIHandler)
        host, port = self.mock_server.server_address
        self.mock_base_url = f"http://{host}:{port}"
        self.mock_thread = threading.Thread(target=self.mock_server.serve_forever, daemon=True)
        self.mock_thread.start()

    def close(self) -> None:
        if self.process is not None and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
        self.process = None
        if self._log is not None:
            self._log.close()
            self._log = None
        if self._temporary_directory is not None:
            self._temporary_directory.cleanup()
            self._temporary_directory = None
        if self.mock_server is not None:
            self.mock_server.shutdown()
            self.mock_server.server_close()
            self.mock_server = None
        if self.mock_thread is not None:
            self.mock_thread.join(timeout=5)
            self.mock_thread = None
        # Cleanup uses the same environment-only secret path as setup.
        mysql(f"DROP DATABASE IF EXISTS `{DATABASE}`;", self.password)


class SystemAPIMySQLE2ETest(unittest.TestCase):
    def test_authorization_and_member_management_use_mysql_state(self) -> None:
        with SystemAPIHarness() as harness:
            cases = ((ALICE, "oms", "owner"), (BOB, "wms", "maintainer"))
            for user_id, expected_code, expected_role in cases:
                with self.subTest(user=user_id):
                    status, envelope = api_request(harness.base_url, "/api/v1/systems", user_id)
                    self.assertEqual(200, status)
                    self.assertEqual(1, len(envelope["data"]))
                    self.assertEqual(expected_code, envelope["data"][0]["code"])
                    self.assertEqual(expected_role, envelope["data"][0]["myRole"])

            status, envelope = api_request(harness.base_url, "/api/v1/systems", OUTSIDER)
            self.assertEqual(200, status)
            self.assertEqual([], envelope["data"])

            status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}", OUTSIDER)
            self.assertEqual(404, status)
            self.assertEqual("system_not_found", envelope["error"]["code"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{WMS}/members",
                BOB,
                body={"userId": MEMBER_TARGET, "role": "viewer"},
            )
            self.assertEqual(403, status)
            self.assertEqual("forbidden", envelope["error"]["code"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/members",
                ALICE,
                body={"userId": MEMBER_TARGET, "role": "runner"},
            )
            self.assertEqual(200, status)
            self.assertEqual(MEMBER_TARGET, envelope["data"]["userId"])
            self.assertEqual("runner", envelope["data"]["role"])
            self.assertEqual("E2E Member", envelope["data"]["displayName"])

            status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}/members", ALICE)
            self.assertEqual(200, status)
            updated = next(member for member in envelope["data"] if member["userId"] == MEMBER_TARGET)
            self.assertEqual("runner", updated["role"])
            self.assertEqual("E2E Member", updated["displayName"])

            for resource in ("scans", "api-operations", "scenario-imports"):
                status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}/{resource}", ALICE)
                self.assertEqual(200, status, resource)
                expected_count = 3 if resource == "api-operations" else 0
                self.assertEqual(expected_count, len(envelope["data"]), resource)

                status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}/{resource}", OUTSIDER)
                self.assertEqual(404, status, resource)
                self.assertEqual("system_not_found", envelope["error"]["code"])

            postman_document = {
                "info": {
                    "name": "Orders",
                    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
                },
                "item": [{"name": "List", "request": {"method": "GET", "url": "/orders"}}],
            }
            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/scenario-imports",
                ALICE,
                body={"fileName": "orders.postman_collection.json", "document": postman_document},
            )
            self.assertEqual(201, status)
            import_id = envelope["data"]["id"]
            self.assertEqual("ready", envelope["data"]["status"])
            self.assertNotIn("rawDocument", envelope["data"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/scenario-imports/{import_id}/apply",
                VIEWER,
                body={},
            )
            self.assertEqual(403, status)
            self.assertEqual("forbidden", envelope["error"]["code"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/scenario-imports/{import_id}/apply",
                ALICE,
                body={},
            )
            self.assertEqual(200, status)
            self.assertEqual("applied", envelope["data"]["status"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/discoveries",
                ALICE,
                body={
                    "type": "code",
                    "name": "订单创建 P0",
                    "operations": [
                        {"id": GET_ORDER_OPERATION, "systemId": OMS, "operationKey": "get-order", "method": "GET", "path": "/orders/{id}"},
                        {"id": CREATE_ORDER_OPERATION, "systemId": OMS, "operationKey": "create-order", "method": "POST", "path": "/orders"},
                        {"id": GET_ORDER_STATUS_OPERATION, "systemId": OMS, "operationKey": "get-order-status", "method": "GET", "path": "/orders/{id}/status"},
                    ],
                },
            )
            self.assertEqual(201, status)
            discovery_id = envelope["data"]["discovery"]["id"]
            candidate = envelope["data"]["candidates"][0]
            self.assertEqual("P0", candidate["priority"])
            self.assertTrue(candidate["requiresReview"])

            decision_path = (
                f"/api/v1/systems/{OMS}/discoveries/{discovery_id}/candidates/"
                f"{candidate['id']}/accept"
            )
            status, envelope = api_request(harness.base_url, decision_path, VIEWER, body={"note": "viewer"})
            self.assertEqual(403, status)
            status, envelope = api_request(harness.base_url, decision_path, ALICE, body={"note": "accepted"})
            self.assertEqual(200, status)
            self.assertEqual("accepted", envelope["data"]["reviewStatus"])

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/discoveries/{discovery_id}/candidates/{candidate['id']}/promote",
                ALICE,
                body={},
            )
            self.assertEqual(201, status)
            promoted_scenario_id = envelope["data"]["scenario"]["id"]
            self.assertTrue(envelope["data"]["created"])

            status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}/scenarios", VIEWER)
            self.assertEqual(200, status)
            self.assertIn(promoted_scenario_id, {item["id"] for item in envelope["data"]})

            status, envelope = api_request(
                harness.base_url,
                f"/api/v1/systems/{OMS}/scenario-runs",
                ALICE,
                body={
                    "scenarioId": SCENARIO,
                    "scenarioVersionId": SCENARIO_VERSION,
                    "environmentId": ENVIRONMENT,
                    "stopAfterStepId": SCENARIO_STEP,
                },
            )
            self.assertEqual(201, status)
            self.assertEqual("passed", envelope["data"]["status"])
            self.assertEqual("succeeded", envelope["data"]["outcome"])

            status, envelope = api_request(harness.base_url, f"/api/v1/systems/{OMS}/scenario-runs", VIEWER)
            self.assertEqual(200, status)
            self.assertEqual(1, len(envelope["data"]))
            self.assertEqual("passed", envelope["data"][0]["status"])
            self.assertEqual(1, len(envelope["data"][0]["attempts"]))


if __name__ == "__main__":
    unittest.main(verbosity=2)
