"""
OpenFreezePR Lambda.

Drives the freeze step: branch off origin/master, set sdk/.version (and
the four other version files set-version.py touches), commit, push, open
the freeze PR.

Implementation note: this Lambda does the file edits via the GitHub
Contents API (no git checkout in the Lambda runtime). For each version
file, it does a `GET /contents` -> mutate -> `PUT /contents` with the
new content + branch name, batched by ref.

Input: {} (the freeze inference is the same as scripts/freeze.py)
Output: {
  "pr_number": 22850,
  "version": "3.235.0",         # the version we're releasing
  "merged_sha": null            # filled in by WaitForFreezeMerged
}
"""

from __future__ import annotations

import base64
import re
from typing import Any

from pulumi_release import gh, versions


REPO = "pulumi/pulumi"


def _read_master_file(path: str) -> tuple[str, str]:
    """Return (decoded_text, blob_sha) of `path` at master HEAD."""
    body = gh.get(f"repos/{REPO}/contents/{path}?ref=master").json()
    return base64.b64decode(body["content"]).decode(), body["sha"]


def _put_contents(branch: str, path: str, content: str, sha: str, message: str) -> None:
    gh.post(f"repos/{REPO}/contents/{path}", json_body={
        "message": message,
        "content": base64.b64encode(content.encode()).decode(),
        "sha": sha,
        "branch": branch,
    })


def _master_head() -> str:
    return gh.get(f"repos/{REPO}/git/refs/heads/master").json()["object"]["sha"]


def _create_branch(name: str, base_sha: str) -> None:
    gh.post(f"repos/{REPO}/git/refs", json_body={
        "ref": f"refs/heads/{name}",
        "sha": base_sha,
    })


def _set_version_in_files(branch: str, new_version: str) -> None:
    """
    Apply the same file edits scripts/set-version.py does: sdk/.version,
    sdk/nodejs/package.json, sdk/nodejs/version.ts,
    sdk/python/lib/pulumi/_version.py, sdk/python/pyproject.toml.

    Note: sdk/python/uv.lock is NOT updated here -- doing so requires `uv
    sync`, which we can't run from a Lambda. The post-release CI catches
    drift; in practice a separate cleanup PR handles it. (TODO: shell out
    to a small Lambda Layer with `uv` if we want this end-to-end.)
    """
    edits: list[tuple[str, callable]] = [
        ("sdk/.version", lambda txt: f"{new_version}\n"),
        ("sdk/nodejs/package.json",
         lambda txt: re.sub(r'"version":\s*"[^"]*"', f'"version": "{new_version}"', txt, count=1)),
        ("sdk/nodejs/version.ts",
         lambda txt: re.sub(r'export const version = "[^"]*";',
                            f'export const version = "{new_version}";', txt, count=1)),
        ("sdk/python/lib/pulumi/_version.py",
         lambda txt: re.sub(r'_VERSION = "[^"]*"', f'_VERSION = "{new_version}"', txt, count=1)),
        ("sdk/python/pyproject.toml",
         lambda txt: re.sub(r'version = "[^"]*"', f'version = "{new_version}"', txt, count=1)),
    ]
    for path, mutator in edits:
        text, sha = _read_master_file(path)
        new_text = mutator(text)
        if new_text == text:
            continue
        _put_contents(branch, path, new_text, sha, f"freeze {new_version}")


def _render_changelog_body() -> str:
    """Render the pending changelog as the PR body via go-change.

    The Lambda runtime does not have Go; we do this via a separate
    `RenderChangelog` Lambda or, as a fallback, leave the body empty and
    let release-pr.yml regenerate it. For now, fallback.

    TODO: a small Go Lambda Layer with go-change pre-built.
    """
    return ""


def handle(event: dict, context) -> dict[str, Any]:
    # Read current sdk/.version to determine release version.
    text, _ = _read_master_file("sdk/.version")
    current = text.strip()
    if not versions.is_valid(current):
        raise RuntimeError(f"OpenFreezePR: invalid sdk/.version content {current!r}")
    release_version = current
    new_version = versions.bump_minor(current)

    branch = f"freeze-v{release_version}"
    base_sha = _master_head()
    _create_branch(branch, base_sha)
    _set_version_in_files(branch, new_version)

    pr = gh.post(f"repos/{REPO}/pulls", json_body={
        "title": f"freeze {release_version}",
        "head": branch,
        "base": "master",
        "body": _render_changelog_body(),
        "draft": False,
    }).json()
    return {
        "pr_number": pr["number"],
        "version": release_version,
        "branch": branch,
    }
