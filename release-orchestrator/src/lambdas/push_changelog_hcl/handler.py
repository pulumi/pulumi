"""
PushChangelogHcl Lambda.

pulumi-labs/pulumi-hcl ships when CHANGELOG.md is updated on master.
This activity reads the current CHANGELOG.md, prepends a new section for
`next_version`, and pushes directly to master. The host's release.yml
fires on the push and produces the new tag.

Input: { "next_version": "v0.2.0" }
Output: { "commit_sha": "<sha>" }

TODO: the production version of this Lambda needs to render the section
body from the host's `.changes/unreleased/` (changie format), not just
prepend a stub. The shape here is the placeholder.
"""

from __future__ import annotations


def handle(event: dict, context) -> dict:
    raise NotImplementedError(
        "PushChangelogHcl: needs changie-shaped section rendering. "
        "v0 keeps this manual; the orchestrator pages the operator and "
        "waits for the upstream tag instead of pushing CHANGELOG.md."
    )
