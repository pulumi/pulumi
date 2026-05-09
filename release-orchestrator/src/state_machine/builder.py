"""
Generate the Step Functions Amazon States Language (ASL) JSON for the
release orchestrator.

Construct via build_state_machine(arns) where arns maps each activity name
to its Lambda ARN (or a placeholder for tests). Returns a dict that can be
json.dumps'd and passed to the AWS SDK / Pulumi.
"""

from __future__ import annotations

import json
from typing import Any


# Default per-activity timeout & retry policy. Override in build_state_machine
# kwargs if a state needs something different.
DEFAULT_RETRY = [
    {
        "ErrorEquals": ["Lambda.ServiceException", "Lambda.AWSLambdaException", "Lambda.SdkClientException"],
        "IntervalSeconds": 2,
        "MaxAttempts": 3,
        "BackoffRate": 2.0,
    }
]


def task(resource_arn: str, *, parameters: dict | None = None,
         result_path: str | None = "$",
         timeout_seconds: int | None = None,
         heartbeat_seconds: int | None = None,
         retry: list[dict] | None = None,
         next_state: str | None = None,
         end: bool = False) -> dict:
    state: dict[str, Any] = {"Type": "Task", "Resource": resource_arn, "Retry": retry or DEFAULT_RETRY}
    if parameters is not None:
        state["Parameters"] = parameters
    if result_path is not None:
        state["ResultPath"] = result_path
    if timeout_seconds is not None:
        state["TimeoutSeconds"] = timeout_seconds
    if heartbeat_seconds is not None:
        state["HeartbeatSeconds"] = heartbeat_seconds
    if end:
        state["End"] = True
    elif next_state:
        state["Next"] = next_state
    return state


def wait_for_event(arns: dict[str, str], predicate: dict, *,
                   timeout_seconds: int = 86400,
                   next_state: str | None = None,
                   end: bool = False,
                   result_path: str = "$.last_event") -> dict:
    """A standard task that calls WaitForGitHubEvent with .waitForTaskToken."""
    return task(
        arns["WaitForGitHubEvent"] + ".waitForTaskToken",
        parameters={
            "predicate": predicate,
            "taskToken.$": "$$.Task.Token",
        },
        timeout_seconds=timeout_seconds,
        heartbeat_seconds=min(timeout_seconds, 86400),
        result_path=result_path,
        next_state=next_state,
        end=end,
    )


def build_state_machine(arns: dict[str, str]) -> dict:
    """Returns the ASL JSON for the orchestrator. `arns` maps activity name -> Lambda ARN."""
    states: dict[str, dict] = {}

    # ---- Top-level pipeline ----

    states["SurveyUpstream"] = task(
        arns["SurveyUpstream"],
        result_path="$.survey",
        next_state="AnyHostsNeedRelease",
    )

    states["AnyHostsNeedRelease"] = {
        "Type": "Choice",
        "Choices": [{
            # JSONPath: any host with has_unreleased_commits=true
            "Variable": "$.survey.language_hosts[?(@.has_unreleased_commits==true)]",
            "IsPresent": True,
            "Next": "ReleaseHosts",
        }],
        "Default": "ResurveyUpstream",
    }

    states["ReleaseHosts"] = {
        "Type": "Map",
        "ItemsPath": "$.survey.language_hosts",
        "MaxConcurrency": 4,
        "ItemProcessor": _per_host_release(arns),
        "ResultPath": "$.release_results",
        "Next": "ResurveyUpstream",
    }

    states["ResurveyUpstream"] = task(
        arns["SurveyUpstream"],
        result_path="$.survey",
        next_state="ComputeBumpVersions",
    )

    states["ComputeBumpVersions"] = task(
        arns["ComputeBumpVersions"],
        parameters={"language_hosts.$": "$.survey.language_hosts"},
        result_path="$.bump_plan",
        next_state="HasBumps",
    )

    states["HasBumps"] = {
        "Type": "Choice",
        "Choices": [{
            "Variable": "$.bump_plan.bumps[0]",
            "IsPresent": True,
            "Next": "OpenBumpPR",
        }],
        "Default": "OpenFreezePR",
    }

    states["OpenBumpPR"] = task(
        arns["OpenBumpPR"],
        parameters={"language_host_versions.$": "$.bump_plan.bumps"},
        result_path="$.bump",
        next_state="WaitForBumpMerged",
    )

    states["WaitForBumpMerged"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "pull_request.merged",
            "pr_number.$": "$.bump.pr_number",
        },
        timeout_seconds=86400,
        next_state="OpenFreezePR",
        result_path="$.bump_merged",
    )

    states["OpenFreezePR"] = task(
        arns["OpenFreezePR"],
        result_path="$.freeze",
        next_state="WaitForFreezeMerged",
    )

    states["WaitForFreezeMerged"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "pull_request.merged",
            "pr_number.$": "$.freeze.pr_number",
        },
        timeout_seconds=86400,
        next_state="WaitForDraftReady",
        result_path="$.freeze_merged",
    )

    states["WaitForDraftReady"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "workflow_run.completed",
            "workflow": "build-test",
            "head_sha.$": "$.freeze_merged.head_sha",
        },
        timeout_seconds=10800,
        next_state="PublishRelease",
        result_path="$.draft_ready",
    )

    states["PublishRelease"] = task(
        arns["PublishRelease"],
        parameters={"version.$": "$.freeze.version"},
        result_path="$.publish",
        next_state="WaitForReleaseFanOut",
    )

    states["WaitForReleaseFanOut"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "workflow_run.completed",
            "workflow": "release",
            "version.$": "$.freeze.version",
        },
        timeout_seconds=14400,
        next_state="WaitForPostReleasePR",
        result_path="$.release_fan_out",
    )

    states["WaitForPostReleasePR"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "pull_request.opened",
            "title.$": "States.Format('Changelog and go.mod updates for v{}', $.freeze.version)",
        },
        timeout_seconds=10800,
        next_state="ApprovePostReleasePR",
        result_path="$.post_release_pr_event",
    )

    states["ApprovePostReleasePR"] = task(
        arns["ApprovePostReleasePR"],
        parameters={"pr_number.$": "$.post_release_pr_event.pr_number"},
        next_state="WaitForPkgTag",
    )

    states["WaitForPkgTag"] = wait_for_event(
        arns,
        predicate={
            "repo": "pulumi/pulumi",
            "event": "create",
            "ref_type": "tag",
            "ref.$": "States.Format('pkg/v{}', $.freeze.version)",
        },
        timeout_seconds=10800,
        next_state="MergeDownstreamBumps",
        result_path="$.pkg_tag",
    )

    states["MergeDownstreamBumps"] = {
        "Type": "Map",
        "ItemsPath": "$.survey.language_hosts",
        "MaxConcurrency": 4,
        "ItemSelector": {
            "language_host.$": "$$.Map.Item.Value",
            "version.$": "$.freeze.version",
        },
        "ItemProcessor": _per_host_downstream_bump(arns),
        "End": True,
    }

    return {
        "Comment": "pulumi/pulumi release orchestrator",
        "StartAt": "SurveyUpstream",
        "States": states,
    }


def _per_host_release(arns: dict[str, str]) -> dict:
    """Inner state machine for the per-language-host release fan-out."""
    states = {
        "HasUnreleasedCommits": {
            "Type": "Choice",
            "Choices": [{
                "Variable": "$.has_unreleased_commits",
                "BooleanEquals": True,
                "Next": "BranchOnHost",
            }],
            "Default": "SkipHost",
        },
        "BranchOnHost": {
            "Type": "Choice",
            "Choices": [{
                "Variable": "$.name",
                "StringEquals": "hcl",
                "Next": "PushChangelogHcl",
            }],
            "Default": "OpenChangelogPR",
        },
        "OpenChangelogPR": task(
            arns["OpenChangelogPR"],
            parameters={
                "language_host.$": "$.name",
                "repo.$": "$.repo",
                "next_version.$": "$.next_version",
            },
            result_path="$.changelog_pr",
            next_state="WaitForChangelogPRMerged",
        ),
        "WaitForChangelogPRMerged": wait_for_event(
            arns,
            predicate={
                "repo.$": "$.repo",
                "event": "pull_request.merged",
                "pr_number.$": "$.changelog_pr.pr_number",
            },
            timeout_seconds=86400,
            next_state="WaitForUpstreamTag",
            result_path="$.changelog_merged",
        ),
        "PushChangelogHcl": task(
            arns["PushChangelogHcl"],
            parameters={"next_version.$": "$.next_version"},
            result_path="$.changelog_push",
            next_state="WaitForUpstreamTag",
        ),
        "WaitForUpstreamTag": wait_for_event(
            arns,
            predicate={
                "repo.$": "$.repo",
                "event": "release.published",
                "tag.$": "$.next_version",
            },
            timeout_seconds=10800,
            end=True,
            result_path="$.upstream_release",
        ),
        "SkipHost": {"Type": "Pass", "End": True},
    }
    return {"StartAt": "HasUnreleasedCommits", "States": states}


def _per_host_downstream_bump(arns: dict[str, str]) -> dict:
    states = {
        "WaitForBumpPR": wait_for_event(
            arns,
            predicate={
                "repo.$": "$.language_host.repo",
                "event": "pull_request.opened",
                "author": "pulumi-bot",
                "title_contains.$": "States.Format('pulumi/pulumi to v{}', $.version)",
            },
            timeout_seconds=86400,
            next_state="ApproveAndMergeBump",
            result_path="$.bump_pr_event",
        ),
        "ApproveAndMergeBump": task(
            arns["ApproveDownstreamBump"],
            parameters={
                "repo.$": "$.language_host.repo",
                "pr_number.$": "$.bump_pr_event.pr_number",
            },
            end=True,
        ),
    }
    return {"StartAt": "WaitForBumpPR", "States": states}


def to_json(arns: dict[str, str]) -> str:
    return json.dumps(build_state_machine(arns), indent=2)


if __name__ == "__main__":
    # For inspection: print the state machine with placeholder ARNs.
    placeholder = {name: f"arn:aws:lambda:us-west-2:000000000000:function:{name}" for name in [
        "SurveyUpstream", "ComputeBumpVersions", "OpenChangelogPR", "PushChangelogHcl",
        "OpenBumpPR", "OpenFreezePR", "PublishRelease", "ApprovePostReleasePR",
        "ApproveDownstreamBump", "WaitForGitHubEvent",
    ]}
    print(to_json(placeholder))
