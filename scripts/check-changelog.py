#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = ["pyyaml"]
# ///

"""
Validate changelog entries in `changelog/pending/`.

Expects changie format (.changie-*.yaml): validates against .changie.yaml
"""

import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
CHANGIE_CONFIG_PATH = REPO_ROOT / ".changie.yaml"
PENDING_DIR = REPO_ROOT / "changelog" / "pending"


def load_changie_config():
    with CHANGIE_CONFIG_PATH.open() as f:
        config = yaml.safe_load(f)
    kinds = {k["key"] for k in config.get("kinds", [])}
    return kinds


def check_changie_entry(path, allowed_kinds):
    errors = []
    try:
        with path.open() as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError as e:
        return [f"YAML parse error: {e}"]
    if not isinstance(data, dict):
        return ["entry is not a YAML mapping"]
    kind = data.get("kind")
    if kind not in allowed_kinds:
        errors.append(
            f"kind {kind!r} is not declared in .changie.yaml (allowed: {sorted(allowed_kinds)})"
        )
    body = data.get("body")
    if not isinstance(body, str) or not body.strip():
        errors.append("body is missing or empty")
    custom = data.get("custom") or {}
    scope = custom.get("Scope")
    if not isinstance(scope, str) or not scope.strip():
        errors.append("custom.Scope is missing or empty")
    return errors


def main():
    if not PENDING_DIR.is_dir():
        print(f"no pending changelog directory at {PENDING_DIR}", file=sys.stderr)
        return 0

    failures = []

    changie_entries = sorted(PENDING_DIR.glob(".changie-*.yaml"))
    if changie_entries:
        if not CHANGIE_CONFIG_PATH.exists():
            for path in changie_entries:
                failures.append(
                    (path.relative_to(REPO_ROOT), "changie format entry found but .changie.yaml is missing")
                )
        else:
            allowed_kinds = load_changie_config()
            for path in changie_entries:
                for err in check_changie_entry(path, allowed_kinds):
                    failures.append((path.relative_to(REPO_ROOT), err))

    if not failures:
        return 0
    print("Invalid changelog entries:", file=sys.stderr)
    for path, err in failures:
        print(f"  {path}: {err}", file=sys.stderr)
    print(
        f"\n{len(failures)} problem(s) found.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
