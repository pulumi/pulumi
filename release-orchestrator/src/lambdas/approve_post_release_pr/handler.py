"""
ApprovePostReleasePR Lambda.

The trust gate for the auto-approve. Refuses to approve unless:
1. Author is `pulumi-bot` (or whichever bot identity opens the PR).
2. PR title matches `Changelog and go.mod updates for vX.Y.Z`.
3. Changed files are within an expected set (CHANGELOG.md + go.mod
   updates + the deleted changelog/pending entries).

Approves via GraphQL `submitPullRequestReview { event: APPROVE }` and
sets auto-merge via the `enablePullRequestAutoMerge` mutation.

Input:  { "pr_number": 22853 }
Output: {}  (raises on refusal)
"""

from __future__ import annotations

import re

from pulumi_release import gh


REPO = "pulumi/pulumi"
EXPECTED_AUTHOR = "pulumi-bot"
TITLE_RE = re.compile(r"^Changelog and go\.mod updates for v\d+\.\d+\.\d+$")
ALLOWED_FILES = {
    "CHANGELOG.md",
    "pkg/go.mod",
    "pkg/go.sum",
}
ALLOWED_PREFIXES = ("changelog/pending/",)


def _validate(pr: dict, files: list[dict]) -> None:
    if pr["user"]["login"] != EXPECTED_AUTHOR:
        raise RuntimeError(
            f"refusing to approve PR #{pr['number']}: author "
            f"{pr['user']['login']!r} != {EXPECTED_AUTHOR!r}"
        )
    if not TITLE_RE.match(pr["title"]):
        raise RuntimeError(
            f"refusing to approve PR #{pr['number']}: title {pr['title']!r} "
            f"doesn't match expected pattern"
        )
    for f in files:
        name = f["filename"]
        if name in ALLOWED_FILES:
            continue
        if any(name.startswith(p) for p in ALLOWED_PREFIXES):
            continue
        raise RuntimeError(
            f"refusing to approve PR #{pr['number']}: unexpected file change {name!r}"
        )


_APPROVE_MUTATION = """
mutation Approve($pr: ID!) {
  addPullRequestReview(input: {pullRequestId: $pr, event: APPROVE}) {
    pullRequestReview { id state }
  }
}
"""

_AUTOMERGE_MUTATION = """
mutation EnableAutoMerge($pr: ID!) {
  enablePullRequestAutoMerge(input: {pullRequestId: $pr, mergeMethod: SQUASH}) {
    pullRequest { number }
  }
}
"""


def handle(event: dict, context) -> dict:
    pr_number = event["pr_number"]
    pr = gh.get(f"repos/{REPO}/pulls/{pr_number}").json()
    files = gh.get(f"repos/{REPO}/pulls/{pr_number}/files?per_page=100").json()
    _validate(pr, files)

    pr_node_id = pr["node_id"]
    gh.graphql(_APPROVE_MUTATION, {"pr": pr_node_id})
    gh.graphql(_AUTOMERGE_MUTATION, {"pr": pr_node_id})
    return {"pr_number": pr_number, "approved": True, "auto_merge": True}
