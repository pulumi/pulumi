"""
SurveyUpstream Lambda.

For each language host, fetch:
- the latest tagged release
- the SHA of upstream master HEAD

If master HEAD differs from the latest tag's commit SHA, the host has
unreleased commits.

Returns:
    {
      "language_hosts": [
        {
          "name": "dotnet",
          "repo": "pulumi/pulumi-dotnet",
          "latest_tag": "v3.105.0",
          "latest_tag_sha": "<sha>",
          "master_sha": "<sha>",
          "has_unreleased_commits": true|false,
          "next_version": "v3.106.0"
        },
        ...
      ]
    }
"""

from __future__ import annotations

import os

from pulumi_release import gh, versions


HOSTS = [
    {"name": "dotnet", "repo": "pulumi/pulumi-dotnet"},
    {"name": "java",   "repo": "pulumi/pulumi-java"},
    {"name": "yaml",   "repo": "pulumi/pulumi-yaml"},
    {"name": "hcl",    "repo": "pulumi-labs/pulumi-hcl"},
]


def _latest_tag(repo: str) -> tuple[str, str]:
    """Return (tag_name, commit_sha) for the latest semver-shaped release."""
    # /repos/{repo}/releases/latest skips drafts/prereleases by default.
    rel = gh.get(f"repos/{repo}/releases/latest").json()
    tag = rel["tag_name"]
    # Resolve the tag to a commit SHA. lightweight-tag refs point straight
    # at the commit; annotated-tag refs point at a tag object that points
    # at the commit, so we have to follow.
    ref = gh.get(f"repos/{repo}/git/refs/tags/{tag}").json()
    obj = ref["object"]
    if obj["type"] == "tag":
        # annotated tag: dereference
        tag_obj = gh.get(f"repos/{repo}/git/tags/{obj['sha']}").json()
        return tag, tag_obj["object"]["sha"]
    return tag, obj["sha"]


def _master_sha(repo: str) -> str:
    ref = gh.get(f"repos/{repo}/git/refs/heads/master").json()
    return ref["object"]["sha"]


def _next_minor(tag: str) -> str:
    return versions.with_v(versions.bump_minor(tag))


def handle(event: dict, context) -> dict:
    if os.environ.get("DRY_RUN") == "true" or event.get("dry_run"):
        return _dry_run()

    out = []
    for host in HOSTS:
        try:
            tag, tag_sha = _latest_tag(host["repo"])
            master = _master_sha(host["repo"])
        except Exception as e:
            # Surface the error -- the orchestrator decides what to do.
            raise RuntimeError(f"survey {host['repo']}: {e}") from e

        out.append({
            "name": host["name"],
            "repo": host["repo"],
            "latest_tag": tag,
            "latest_tag_sha": tag_sha,
            "master_sha": master,
            "has_unreleased_commits": master != tag_sha,
            "next_version": _next_minor(tag),
        })
    return {"language_hosts": out}


def _dry_run() -> dict:
    return {"language_hosts": [
        {"name": h["name"], "repo": h["repo"],
         "latest_tag": "v0.0.0", "latest_tag_sha": "deadbeef",
         "master_sha": "deadbeef", "has_unreleased_commits": False,
         "next_version": "v0.1.0"}
        for h in HOSTS
    ]}
