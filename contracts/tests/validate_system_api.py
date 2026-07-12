"""Structural validation for the System & Access OpenAPI contract."""

from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
CONTRACT = ROOT / "contracts/openapi/system-api.yaml"


def main() -> None:
    document = yaml.safe_load(CONTRACT.read_text(encoding="utf-8"))
    assert document["openapi"] == "3.1.0"

    paths = document["paths"]
    for path in ("/systems", "/systems/{systemId}", "/systems/{systemId}/members"):
        assert path in paths, f"missing path: {path}"

    schemas = document["components"]["schemas"]
    roles = set(schemas["SystemRole"]["enum"])
    assert roles == {"owner", "maintainer", "reviewer", "runner", "viewer"}

    system_required = set(schemas["BusinessSystem"]["required"])
    assert {"id", "code", "name", "status", "myRole"} <= system_required

    print(f"PASS: System API contract with {len(paths)} paths and {len(roles)} roles")


if __name__ == "__main__":
    main()

