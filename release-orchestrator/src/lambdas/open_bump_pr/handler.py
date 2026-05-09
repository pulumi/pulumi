"""
OpenBumpPR Lambda.

Curated "Bump language providers" PR. Edits scripts/get-language-providers.sh
to the new versions from ComputeBumpVersions, opens a PR.

The bump entries form is `"<lang> <version>"` strings inside an array.

Input:  { "language_host_versions": [{name, current_pin, new_version, repo}, ...] }
Output: { "pr_number": 22833 }
"""

from __future__ import annotations

import base64
import re

from pulumi_release import gh, versions


REPO = "pulumi/pulumi"
SCRIPT_PATH = "scripts/get-language-providers.sh"


def _read_master(path: str) -> tuple[str, str]:
    body = gh.get(f"repos/{REPO}/contents/{path}?ref=master").json()
    return base64.b64decode(body["content"]).decode(), body["sha"]


def _master_head() -> str:
    return gh.get(f"repos/{REPO}/git/refs/heads/master").json()["object"]["sha"]


def _apply_bumps(text: str, bumps: list[dict]) -> str:
    out = text
    for bump in bumps:
        # Replace `"<lang> <oldver>` with `"<lang> <newver>`. Lookahead for
        # closing quote or whitespace to avoid greedy matches.
        pattern = re.compile(rf'"{bump["name"]}\s+v[0-9.]+(?=\s|"|\\s)')
        new_str = f'"{bump["name"]} {versions.with_v(bump["new_version"])}'
        out = pattern.sub(new_str, out, count=1)
    return out


def handle(event: dict, context) -> dict:
    bumps = event["language_host_versions"]
    if not bumps:
        raise RuntimeError("OpenBumpPR: empty bump list")

    text, sha = _read_master(SCRIPT_PATH)
    new_text = _apply_bumps(text, bumps)
    if new_text == text:
        raise RuntimeError(
            "OpenBumpPR: no changes after applying bumps; check name match"
        )

    base_sha = _master_head()
    bump_summary = ", ".join(
        f"{b['name']} {b['current_pin']} -> {b['new_version']}" for b in bumps
    )
    branch = "automation/bump-language-providers-" + base_sha[:8]
    gh.post(f"repos/{REPO}/git/refs", json_body={
        "ref": f"refs/heads/{branch}", "sha": base_sha,
    })
    gh.post(f"repos/{REPO}/contents/{SCRIPT_PATH}", json_body={
        "message": f"Bump language providers ({bump_summary})",
        "content": base64.b64encode(new_text.encode()).decode(),
        "sha": sha,
        "branch": branch,
    })

    pr = gh.post(f"repos/{REPO}/pulls", json_body={
        "title": "Bump language providers",
        "head": branch,
        "base": "master",
        "body": (
            "Updates language plugin pins:\n\n"
            + "\n".join(
                f"- `{b['name']}`: {b['current_pin']} -> {b['new_version']}"
                for b in bumps
            )
            + "\n\nOpened by the release orchestrator. The merge queue will pick this up "
              "once CI is green."
        ),
    }).json()
    return {"pr_number": pr["number"], "branch": branch}
