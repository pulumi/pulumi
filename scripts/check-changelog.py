#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = ["pyyaml"]
# ///

"""
Validate changelog entries in `changelog/pending/`.

Supports two formats:
- go-change format (*.yaml):   validates against changelog/config.yaml
- changie format (.changie-*.yaml): validates against .changie.yaml
"""

import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
GOCHANGE_CONFIG_PATH = REPO_ROOT / "changelog" / "config.yaml"
CHANGIE_CONFIG_PATH = REPO_ROOT / ".changie.yaml"
PENDING_DIR = REPO_ROOT / "changelog" / "pending"


def load_gochange_config():
    with GOCHANGE_CONFIG_PATH.open() as f:
        config = yaml.safe_load(f)
    types = set(config.get("types", {}).keys())
    scopes = config.get("scopes", {}) or {}
    return types, scopes


def load_changie_config():
    with CHANGIE_CONFIG_PATH.open() as f:
        config = yaml.safe_load(f)
    kinds = {k["key"] for k in config.get("kinds", [])}
    return kinds


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


def check_gochange_entry(path, allowed_types, allowed_scopes):
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

    # Validate go-change format entries (plain *.yaml, not dotfiles)
    gochange_entries = [
        p for p in sorted(PENDING_DIR.glob("*.yaml")) if not p.name.startswith(".")
    ]
    if gochange_entries:
        if not GOCHANGE_CONFIG_PATH.exists():
            for path in gochange_entries:
                failures.append(
                    (path.relative_to(REPO_ROOT), "go-change format entry found but changelog/config.yaml has been removed; convert to changie format")
                )
        else:
            allowed_types, allowed_scopes = load_gochange_config()
            for path in gochange_entries:
                for err in check_gochange_entry(path, allowed_types, allowed_scopes):
                    failures.append((path.relative_to(REPO_ROOT), err))

    # Validate changie format entries (.changie-*.yaml)
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
