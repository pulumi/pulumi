#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = ["pyyaml"]
# ///

"""
Validate changelog entries in `changelog/pending/` against `changelog/config.yaml`.

Each entry must have a `type` declared in config and a `scope` matching one of:
  - a parent listed under `scopes:`
  - `<parent>/<sub>` where `<sub>` is in the parent's allowed list (subscopes may
    be comma-separated, e.g. `sdk/nodejs,python`)
"""

import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
CONFIG_PATH = REPO_ROOT / "changelog" / "config.yaml"
PENDING_DIR = REPO_ROOT / "changelog" / "pending"


def load_config():
    with CONFIG_PATH.open() as f:
        config = yaml.safe_load(f)
    types = set(config.get("types", {}).keys())
    scopes = config.get("scopes", {}) or {}
    return types, scopes


def check_scope(scope, allowed_scopes):
    parents = sorted(allowed_scopes)
    if not isinstance(scope, str) or not scope:
        return f"scope is missing or empty (allowed parents: {parents})"
    parent, _, sub = scope.partition("/")
    if parent not in allowed_scopes:
        return f"parent scope {parent!r} is not declared in config (allowed: {parents})"
    if not sub:
        return None
    allowed_subs = sorted(allowed_scopes.get(parent) or [])
    bad = [s for s in sub.split(",") if s not in allowed_subs]
    if bad:
        return f"sub-scope(s) {bad} not declared under {parent!r} (allowed: {allowed_subs or '<none>'})"
    return None


def check_entry(path, allowed_types, allowed_scopes):
    errors = []
    try:
        with path.open() as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError as e:
        return [f"YAML parse error: {e}"]
    if not isinstance(data, dict) or "changes" not in data:
        return ["missing top-level `changes:` list"]
    changes = data.get("changes")
    if not isinstance(changes, list) or not changes:
        return ["`changes:` must be a non-empty list"]
    for i, ch in enumerate(changes):
        prefix = f"changes[{i}]"
        if not isinstance(ch, dict):
            errors.append(f"{prefix} is not a mapping")
            continue
        t = ch.get("type")
        if t not in allowed_types:
            errors.append(
                f"{prefix}.type {t!r} is not declared in config (allowed: {sorted(allowed_types)})"
            )
        scope_err = check_scope(ch.get("scope"), allowed_scopes)
        if scope_err:
            errors.append(f"{prefix}.{scope_err}")
        desc = ch.get("description")
        if not isinstance(desc, str) or not desc.strip():
            errors.append(f"{prefix}.description is missing or empty")
    return errors


def main():
    if not PENDING_DIR.is_dir():
        print(f"no pending changelog directory at {PENDING_DIR}", file=sys.stderr)
        return 0
    allowed_types, allowed_scopes = load_config()
    failures = []
    for path in sorted(PENDING_DIR.glob("*.yaml")):
        for err in check_entry(path, allowed_types, allowed_scopes):
            failures.append((path.relative_to(REPO_ROOT), err))
    if not failures:
        return 0
    print("Invalid changelog entries:", file=sys.stderr)
    for path, err in failures:
        print(f"  {path}: {err}", file=sys.stderr)
    print(
        f"\n{len(failures)} problem(s) found. "
        "Update the entry, or add the type/scope to changelog/config.yaml.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
