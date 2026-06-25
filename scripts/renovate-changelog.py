#!/usr/bin/env python3

"""
Generate changelog entries for language-runtime version bumps that renovate applied to
versions.json, the repo-root single source of truth for pinned external language runtimes.

Invoked from `make renovate` (itself called from a renovate postUpgradeTasks hook). Detects
bumps by comparing versions.json against HEAD and shells out to `go-change create` for each
changed runtime. No-op when nothing relevant changed.
"""

import json
import subprocess
import sys

VERSIONS_FILE = "versions.json"

# Changelog scope for each runtime. Unlisted runtimes fall back to their own name.
SCOPE_FOR = {
    "dotnet": "sdk/dotnet",
    "java": "java",
    "yaml": "yaml",
}

# Pinned version to match the version used by `make changelog` in the Makefile.
GO_CHANGE = ["go", "run", "github.com/pulumi/go-change@v0.1.3", "create"]


def runtimes(doc):
    """Flatten the runtime sections of a parsed versions.json into {name: version}."""
    result = {}
    for section in ("bundledLanguageRuntimes", "unbundledLanguageRuntimes"):
        for name, entry in (doc.get(section) or {}).items():
            result[name] = entry["version"]
    return result


def head_runtimes():
    """Return the runtimes recorded in versions.json at HEAD, or {} if it is absent."""
    proc = subprocess.run(
        ["git", "show", f"HEAD:{VERSIONS_FILE}"],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        return {}
    return runtimes(json.loads(proc.stdout))


def main():
    with open(VERSIONS_FILE, encoding="utf-8") as f:
        current = runtimes(json.load(f))
    previous = head_runtimes()

    for lang, version in current.items():
        # Only emit for an existing runtime whose pinned version actually changed.
        if previous.get(lang, version) == version:
            continue
        scope = SCOPE_FOR.get(lang, lang)
        title = f"upgrade-{lang}-to-{version.replace('.', '-')}"
        description = f"Upgrade {lang} to {version}"
        subprocess.run(
            GO_CHANGE
            + [
                "-t",
                "chore",
                "-s",
                scope,
                "-d",
                description,
                "--title",
                title,
            ],
            check=True,
            stdin=subprocess.DEVNULL,
        )


if __name__ == "__main__":
    sys.exit(main())
