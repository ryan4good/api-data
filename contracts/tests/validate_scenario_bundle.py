"""Validate the canonical Scenario Bundle example against its JSON Schema."""
import json
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker


ROOT = Path(__file__).resolve().parents[2]
SCHEMA_PATH = ROOT / "contracts/scenario-bundle.schema.json"
EXAMPLE_PATH = ROOT / "contracts/examples/scenario-bundle.example.json"


def main() -> None:
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    example = json.loads(EXAMPLE_PATH.read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    errors = sorted(validator.iter_errors(example), key=lambda error: list(error.path))
    assert not errors, "\n".join(
        f"{'.'.join(map(str, error.path)) or '<root>'}: {error.message}" for error in errors
    )
    assert len({step["key"] for step in example["scenario"]["steps"]}) == len(
        example["scenario"]["steps"]
    ), "step keys must be unique"
    print(f"PASS: scenario bundle with {len(example['scenario']['steps'])} steps")


if __name__ == "__main__":
    main()
