# Copyright 2016-2022, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
from typing import Optional
import pytest

from pulumi.automation._remote_workspace import _is_fully_qualified_stack_name

from pulumi.automation import (
    LocalWorkspace,
    OpType,
    RemoteGitAuth,
    RemoteWorkspaceOptions,
    create_remote_stack_git_source,
    create_or_select_remote_stack_git_source,
    select_remote_stack_git_source,
)

from .test_utils import stack_namer


test_repo = "https://github.com/pulumi/test-repo.git"


@pytest.mark.parametrize(
    "factory",
    [
        create_remote_stack_git_source,
        create_or_select_remote_stack_git_source,
        select_remote_stack_git_source,
    ],
)
@pytest.mark.parametrize(
    "error,stack_name,url,branch,commit_hash,auth",
    [
        ('stack name "" must be fully qualified.', "", "", None, None, None),
        ('stack name "name" must be fully qualified.', "name", "", None, None, None),
        (
            'stack name "owner/name" must be fully qualified.',
            "owner/name",
            "",
            None,
            None,
            None,
        ),
        ('stack name "/" must be fully qualified.', "/", "", None, None, None),
        ('stack name "//" must be fully qualified.', "//", "", None, None, None),
        ('stack name "///" must be fully qualified.', "///", "", None, None, None),
        (
            'stack name "owner/project/stack/wat" must be fully qualified.',
            "owner/project/stack/wat",
            "",
            None,
            None,
            None,
        ),
        ("url is required.", "owner/project/stack", None, None, None, None),
        ("url is required.", "owner/project/stack", "", None, None, None),
        (
            "either branch or commit_hash is required.",
            "owner/project/stack",
            test_repo,
            None,
            None,
            None,
        ),
        (
            "either branch or commit_hash is required.",
            "owner/project/stack",
            test_repo,
            "",
            "",
            None,
        ),
        (
            "branch and commit_hash cannot both be specified.",
            "owner/project/stack",
            test_repo,
            "branch",
            "commit",
            None,
        ),
        (
            "ssh_private_key and ssh_private_key_path cannot both be specified.",
            "owner/project/stack",
            test_repo,
            "branch",
            None,
            RemoteGitAuth(ssh_private_key="key", ssh_private_key_path="path"),
        ),
    ],
)
def test_remote_workspace_errors(
    factory,
    error: str,
    stack_name: str,
    url: str,
    branch: Optional[str],
    commit_hash: Optional[str],
    auth: Optional[RemoteGitAuth],
):
    with pytest.raises(Exception) as e_info:
        factory(
            stack_name=stack_name,
            url=url,
            branch=branch,
            commit_hash=commit_hash,
            auth=auth,
        )
    assert str(e_info.value) == error


# These tests require the service with access to Pulumi Deployments.
# Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
# and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the tests.
@pytest.mark.parametrize(
    "factory",
    [
        create_remote_stack_git_source,
        create_or_select_remote_stack_git_source,
    ],
)
@pytest.mark.skipif(
    "PULUMI_ACCESS_TOKEN" not in os.environ, reason="PULUMI_ACCESS_TOKEN not set"
)
@pytest.mark.skipif(
    "PULUMI_TEST_DEPLOYMENTS_API" not in os.environ,
    reason="PULUMI_TEST_DEPLOYMENTS_API not set",
)
def test_remote_workspace_stack_lifecycle(factory):
    project_name = "go_remote_proj"
    stack_name = stack_namer(project_name)
    stack = factory(
        stack_name=stack_name,
        url=test_repo,
        branch="refs/heads/master",
        project_path="goproj",
        opts=RemoteWorkspaceOptions(
            pre_run_commands=[
                f"pulumi config set bar abc --stack {stack_name}",
                f"pulumi config set --secret buzz secret --stack {stack_name}",
            ],
            skip_install_dependencies=True,
        ),
    )

    # pulumi up
    up_res = stack.up()
    assert len(up_res.outputs) == 3
    assert up_res.outputs["exp_static"].value == "foo"
    assert not up_res.outputs["exp_static"].secret
    assert up_res.outputs["exp_cfg"].value == "abc"
    assert not up_res.outputs["exp_cfg"].secret
    assert up_res.outputs["exp_secret"].value == "secret"
    assert up_res.outputs["exp_secret"].secret
    assert up_res.summary.kind == "update"
    assert up_res.summary.result == "succeeded"

    # pulumi preview
    preview_result = stack.preview()
    assert preview_result.change_summary.get(OpType.SAME) == 1

    # pulumi refresh
    refresh_res = stack.refresh()
    assert refresh_res.summary.kind == "refresh"
    assert refresh_res.summary.result == "succeeded"

    # pulumi destroy
    destroy_res = stack.destroy()
    assert destroy_res.summary.kind == "destroy"
    assert destroy_res.summary.result == "succeeded"

    LocalWorkspace().remove_stack(stack_name)


@pytest.mark.parametrize(
    "input,expected",
    [
        ("owner/project/stack", True),
        ("", False),
        ("name", False),
        ("owner/name", False),
        ("/", False),
        ("//", False),
        ("///", False),
        ("owner/project/stack/wat", False),
    ],
)
def test_config_get_with_defaults(input, expected):
    actual = _is_fully_qualified_stack_name(input)
    assert expected == actual
