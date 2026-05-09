"""
ApproveDownstreamBump Lambda.

Same trust gate shape as ApprovePostReleasePR, applied to bump PRs in
upstream language host repos (pulumi-dotnet, pulumi-java, pulumi-yaml,
pulumi-labs/pulumi-hcl).

Refuses to approve unless:
- Author is `pulumi-bot` (or another known bot)
- PR title contains `pulumi/pulumi to v` (the conventional shape)
- Files changed are limited to the host's own go.mod / package.json /
  pyproject.toml / etc. (no source-code drift via this PR)

Input: { "repo": "pulumi/pulumi-yaml", "pr_number": 1079 }
"""

from __future__ import annotations

import re

from pulumi_release import gh


ALLOWED_AUTHORS = {"pulumi-bot", "app/pulumi-renovate"}
TITLE_PATTERN = re.compile(r"pulumi/pulumi to v\d+\.\d+\.\d+", re.IGNORECASE)
# Conservative file allow-list. Hosts vary; leave the door open for
# pyproject/go.mod/package.json updates and changelog entries.
ALLOWED_PATTERNS = (
    re.compile(r"^go\.mod$"),
    re.compile(r"^go\.sum$"),
    re.compile(r"^.*/go\.mod$"),
    re.compile(r"^.*/go\.sum$"),
    re.compile(r"^.*/package\.json$"),
    re.compile(r"^.*/pyproject\.toml$"),
    re.compile(r"^\.changes/unreleased/.+\.yaml$"),  # changie convention
    re.compile(r"^CHANGELOG\.md$"),
    re.compile(r"^CHANGELOG_PENDING\.md$"),
)


def _validate(repo: str, pr: dict, files: list[dict]) -> None:
    if pr["user"]["login"] not in ALLOWED_AUTHORS:
        raise RuntimeError(
            f"refusing to approve {repo}#{pr['number']}: author "
            f"{pr['user']['login']!r} not allow-listed"
        )
    if not TITLE_PATTERN.search(pr["title"]):
        raise RuntimeError(
            f"refusing to approve {repo}#{pr['number']}: title doesn't match expected shape"
        )
    for f in files:
        name = f["filename"]
        if not any(p.match(name) for p in ALLOWED_PATTERNS):
            raise RuntimeError(
                f"refusing to approve {repo}#{pr['number']}: unexpected file {name!r}"
            )


_APPROVE = """
mutation Approve($pr: ID!) {
  addPullRequestReview(input: {pullRequestId: $pr, event: APPROVE}) {
    pullRequestReview { id }
  }
}
"""

_AUTOMERGE = """
mutation EnableAutoMerge($pr: ID!) {
  enablePullRequestAutoMerge(input: {pullRequestId: $pr, mergeMethod: SQUASH}) {
    pullRequest { number }
  }
}
"""


def handle(event: dict, context) -> dict:
    repo = event["repo"]
    pr_number = event["pr_number"]
    pr = gh.get(f"repos/{repo}/pulls/{pr_number}").json()
    files = gh.get(f"repos/{repo}/pulls/{pr_number}/files?per_page=100").json()
    _validate(repo, pr, files)

    gh.graphql(_APPROVE, {"pr": pr["node_id"]})
    gh.graphql(_AUTOMERGE, {"pr": pr["node_id"]})
    return {"repo": repo, "pr_number": pr_number, "approved": True}
