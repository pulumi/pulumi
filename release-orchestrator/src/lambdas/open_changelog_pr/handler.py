"""
OpenChangelogPR Lambda.

Per-host changelog PR opener. Each language host has its own changelog
flow:
- pulumi-yaml: changie-based; runs `changie batch --include unreleased`
  followed by `changie merge`. The changelog PR title is
  `Changelog for vX.Y.Z`.
- pulumi-dotnet: similar (changie).
- pulumi-java: split flow; the "release-java-provider" PR shape.

Input: {
  "language_host": "yaml" | "dotnet" | "java",
  "repo": "pulumi/pulumi-yaml" | ...,
  "next_version": "v1.33.0"
}
Output: { "pr_number": 1079, "branch": "automation/changelog-vX.Y.Z" }

TODO: each host's changelog flow is its own beast. The current
implementation only handles the changie shape (yaml, dotnet) by:
  1. Branching from upstream master.
  2. Running changie batch via a Lambda Layer that bundles `changie`.
  3. Committing the rendered .changes/<version>.md and removed unreleased
     entries.
  4. Opening the PR.

For pulumi-java's split flow, this Lambda either fans out or returns an
explicit "manual" status that the orchestrator's Choice state surfaces
to the operator. (Step 1 of the v0 deployment of this orchestrator is to
keep pulumi-java manual; only yaml/dotnet/hcl auto-flow.)
"""

from __future__ import annotations

from pulumi_release import gh


def handle(event: dict, context) -> dict:
    raise NotImplementedError(
        "OpenChangelogPR: per-host changelog flow not yet implemented. "
        "v0 keeps this manual; the orchestrator pages the operator and "
        "waits for a manually-opened changelog PR. Implementation tracked "
        "separately."
    )
