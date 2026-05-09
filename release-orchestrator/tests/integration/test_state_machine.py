"""
Integration test against Step Functions Local with mocked Lambda
responses. Spins up the SFN Local container if not already running.

Prereq: Docker daemon running. The container is launched by
`scripts/test-local.sh` (which this file expects to have been run, or
which can be invoked from a test fixture).
"""

from __future__ import annotations

import json
import os
import time
import sys
from pathlib import Path

import boto3
import pytest


SF_LOCAL_ENDPOINT = os.environ.get("SFN_LOCAL_ENDPOINT", "http://localhost:8083")
ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "src"))

from state_machine import builder  # noqa: E402


def _client():
    return boto3.client(
        "stepfunctions",
        endpoint_url=SF_LOCAL_ENDPOINT,
        region_name="us-west-2",
        aws_access_key_id="dummy",
        aws_secret_access_key="dummy",
    )


def _placeholder_arns() -> dict[str, str]:
    return {name: f"arn:aws:lambda:us-west-2:000000000000:function:{name}" for name in [
        "SurveyUpstream", "ComputeBumpVersions", "OpenChangelogPR",
        "PushChangelogHcl", "OpenBumpPR", "OpenFreezePR", "PublishRelease",
        "ApprovePostReleasePR", "ApproveDownstreamBump", "WaitForGitHubEvent",
    ]}


@pytest.fixture(scope="module")
def state_machine_arn():
    sfn = _client()
    definition = builder.to_json(_placeholder_arns())
    resp = sfn.create_state_machine(
        name="release-orchestrator",
        definition=definition,
        roleArn="arn:aws:iam::000000000000:role/dummy",
    )
    yield resp["stateMachineArn"]


def _wait_for_completion(sfn, exec_arn, timeout=60):
    start = time.time()
    while True:
        resp = sfn.describe_execution(executionArn=exec_arn)
        if resp["status"] != "RUNNING":
            return resp
        if time.time() - start > timeout:
            raise TimeoutError(f"execution {exec_arn} did not finish in {timeout}s")
        time.sleep(0.5)


@pytest.mark.skipif("SFN_LOCAL_ENDPOINT" not in os.environ,
                    reason="set SFN_LOCAL_ENDPOINT to run integration tests")
def test_happy_path_no_language_bumps(state_machine_arn):
    sfn = _client()
    resp = sfn.start_execution(
        stateMachineArn=f"{state_machine_arn}#happy-path-no-language-bumps",
        input=json.dumps({}),
    )
    final = _wait_for_completion(sfn, resp["executionArn"])
    assert final["status"] == "SUCCEEDED", final


@pytest.mark.skipif("SFN_LOCAL_ENDPOINT" not in os.environ,
                    reason="set SFN_LOCAL_ENDPOINT to run integration tests")
def test_happy_path_with_yaml_bump(state_machine_arn):
    sfn = _client()
    resp = sfn.start_execution(
        stateMachineArn=f"{state_machine_arn}#happy-path-with-yaml-bump",
        input=json.dumps({}),
    )
    final = _wait_for_completion(sfn, resp["executionArn"])
    assert final["status"] == "SUCCEEDED", final
