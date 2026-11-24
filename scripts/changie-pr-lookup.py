#!/usr/bin/env python3
"""Amend pending changie entries with the PR number that introduced them.

Iterates over every YAML file in ``changelog/pending``, uses ``git log``
to find the commit that added the file, extracts the PR number from the
trailing ``(#NNNN)`` of the squash-merge commit subject, and writes it
into the entry's ``custom.PR`` field. Entries that already have a ``PR``
value are left alone. Entries whose introducing commit cannot be
resolved to a PR number are also left alone (e.g. uncommitted entries
created locally for the current release PR).

Intended to be run before ``changie batch`` in CI. See
https://github.com/miniscruff/changie/issues/895 for background.
"""

import pathlib
import re
import subprocess
import sys

PR_RE = re.compile(r"\(#(\d+)\)\s*$")
PENDING_DIR = pathlib.Path("changelog/pending")


def find_pr_number(filename: pathlib.Path) -> str | None:
    result = subprocess.run(
        ["git", "log", "--diff-filter=A", "--format=%s", "--", str(filename)],
        capture_output=True,
        text=True,
        check=True,
    )
    for line in result.stdout.splitlines():
        m = PR_RE.search(line.strip())
        if m:
            return m.group(1)
    return None


def amend(path: pathlib.Path, pr: str) -> bool:
    """Insert ``PR: "<pr>"`` into the file's custom section.

    Returns True if the file was modified. We edit the YAML textually to
    preserve formatting and avoid a PyYAML dependency.
    """
    text = path.read_text()
    lines = text.splitlines(keepends=True)

    # If a PR is already present anywhere under custom, leave it.
    if re.search(r"^\s+PR:\s", text, re.MULTILINE):
        return False

    custom_idx = next(
        (i for i, l in enumerate(lines) if re.match(r"^custom\s*:", l)),
        None,
    )
    pr_line = f'    PR: "{pr}"\n'
    if custom_idx is None:
        if lines and not lines[-1].endswith("\n"):
            lines[-1] += "\n"
        lines.append("custom:\n")
        lines.append(pr_line)
    else:
        lines.insert(custom_idx + 1, pr_line)

    path.write_text("".join(lines))
    return True


def main() -> int:
    if not PENDING_DIR.is_dir():
        print(f"no {PENDING_DIR} directory found", file=sys.stderr)
        return 0

    for path in sorted(PENDING_DIR.glob("*.yaml")):
        pr = find_pr_number(path)
        if not pr:
            continue
        if amend(path, pr):
            print(f"{path}: PR #{pr}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
