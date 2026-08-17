#!/usr/bin/env python3

"""
Generate changelog entries for language provider version bumps that renovate applied to
scripts/get-language-providers.sh.

Invoked from `make renovate` (which is itself called from a renovate postUpgradeTasks hook). Detects bumps by diffing
the script against HEAD and shells out to `changie new` for each one. No-op when nothing relevant changed.
"""

import re
import subprocess
import sys

SCRIPT = "scripts/get-language-providers.sh"

SCOPE_FOR = {
    "dotnet": "sdk/dotnet",
    "java": "java",
    "yaml": "yaml",
}

# Matches an array entry like: "dotnet v3.102.0"
ENTRY_RE = re.compile(r'^\s*"([a-z]+)\s+(v[0-9][0-9.]*)"\s*$')

CHANGIE = [
    "mise",
    "exec",
    "--yes",
    "--",
    "changie",
    "new",
    "--interactive=false",
]


def added_entries(diff: str):
    """Yield (lang, version) tuples for added lines in a unified diff."""
    for line in diff.splitlines():
        # Skip diff headers like "+++ b/..." but keep "+<content>" lines.
        if not line.startswith("+") or line.startswith("+++"):
            continue
        match = ENTRY_RE.match(line[1:])
        if match:
            yield match.group(1), match.group(2)


def main():
    diff = subprocess.run(
        ["git", "diff", "HEAD", "--", SCRIPT],
        check=True,
        capture_output=True,
        text=True,
    ).stdout

    if not diff:
        return

    for lang, version in added_entries(diff):
        scope = SCOPE_FOR.get(lang, lang)
        description = f"Upgrade {lang} to {version}"
        subprocess.run(
            CHANGIE
            + [
                "--kind",
                "chore",
                "--component",
                scope,
                "--body",
                description,
            ],
            check=True,
            stdin=subprocess.DEVNULL,
        )


if __name__ == "__main__":
    sys.exit(main())
