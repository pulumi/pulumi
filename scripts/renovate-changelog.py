#!/usr/bin/env python3

"""
Generate changelog entries for language runtime version bumps that renovate applied to
scripts/get-language-providers.sh (bundled runtimes) or pkg/util/plugin.go (unbundled runtimes).

Invoked from `make renovate` (which is itself called from a renovate postUpgradeTasks hook). Detects bumps by diffing
the files against HEAD and shells out to `changie new` for each one. No-op when nothing relevant changed.
"""

import re
import subprocess
import sys

# Files renovate updates, paired with a regex matching a single version entry in them. Each
# regex captures the language name and its version (without the leading "v", if any).
SOURCES = [
    # Matches an array entry like: "dotnet v3.102.0"
    (
        "scripts/get-language-providers.sh",
        re.compile(r'^\s*"([a-z]+)\s+v([0-9][0-9.]*)"\s*$'),
    ),
    # Matches a map entry like: "hcl": semver.MustParse("0.16.2"),
    (
        "pkg/util/plugin.go",
        re.compile(r'^\s*"([a-z]+)":\s+semver\.MustParse\("([0-9][0-9.]*)"\),?\s*$'),
    ),
]

SCOPE_FOR = {
    "dotnet": "sdk/dotnet",
    "hcl": "hcl",
    "java": "java",
    "yaml": "yaml",
}

CHANGIE = [
    "mise",
    "exec",
    "--yes",
    "--",
    "changie",
    "new",
    "--interactive=false",
]


def added_entries(diff: str, entry_re: re.Pattern):
    """Yield (lang, version) tuples for added lines in a unified diff."""
    for line in diff.splitlines():
        # Skip diff headers like "+++ b/..." but keep "+<content>" lines.
        if not line.startswith("+") or line.startswith("+++"):
            continue
        match = entry_re.match(line[1:])
        if match:
            yield match.group(1), match.group(2)


def bumps():
    """Yield (lang, version) tuples for every version bump in the working tree."""
    for path, entry_re in SOURCES:
        diff = subprocess.run(
            ["git", "diff", "HEAD", "--", path],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
        yield from added_entries(diff, entry_re)


def main():
    for lang, version in bumps():
        scope = SCOPE_FOR.get(lang, lang)
        description = f"Upgrade {lang} to v{version}"
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
