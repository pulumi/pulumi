# Copyright 2026, Pulumi Corporation.
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

import pulumi
import pytest


def test_helpers_build_awsx_style_migration():
    old_component_urn = "urn:pulumi:stack::project::awsx:ecr:Repository::repo"
    old_repository_urn = (
        "urn:pulumi:stack::project::awsx:ecr:Repository$"
        "aws:ecr/repository:Repository::repo"
    )
    old_policy_urn = (
        "urn:pulumi:stack::project::awsx:ecr:Repository$"
        "aws:ecr/lifecyclePolicy:LifecyclePolicy::repo"
    )
    new_repository_urn = (
        "urn:pulumi:stack::project::aws:ecr/repository:Repository::repository"
    )
    new_policy_urn = (
        "urn:pulumi:stack::project::aws:ecr/repository:Repository$"
        "aws:ecr/lifecyclePolicy:LifecyclePolicy::lifecycle-policy"
    )
    old_state = [
        {"urn": old_component_urn, "type": "awsx:ecr:Repository"},
        {
            "urn": old_repository_urn,
            "type": "aws:ecr/repository:Repository",
            "parent": old_component_urn,
            "custom": True,
            "id": "repository-id",
        },
        {
            "urn": old_policy_urn,
            "type": "aws:ecr/lifecyclePolicy:LifecyclePolicy",
            "parent": old_component_urn,
            "custom": True,
            "id": "policy-id",
        },
    ]
    args = pulumi.StateMigrationArgs(new_repository_urn, old_state)
    ctx = pulumi.StateMigrationContext(args)

    repository = ctx.rename(
        old_repository_urn,
        new_repository_urn,
        parent=None,
    )
    ctx.merge(old_component_urn, into=repository)
    ctx.rename(old_policy_urn, new_policy_urn, parent=repository)
    result = ctx.result()

    assert old_state[1]["urn"] == old_repository_urn
    assert old_state[1]["parent"] == old_component_urn
    assert [resource["urn"] for resource in result.new_state] == [
        new_repository_urn,
        new_policy_urn,
    ]
    assert result.new_state[0].get("parent") is None
    assert result.new_state[1]["parent"] == new_repository_urn
    assert result.successors == {
        old_component_urn: new_repository_urn,
        old_repository_urn: new_repository_urn,
        old_policy_urn: new_policy_urn,
    }


def test_replace_preserves_position_and_copies_replacement():
    root_urn = "urn:pulumi:stack::project::example:index:Component::component"
    child_urn = (
        "urn:pulumi:stack::project::example:index:Component$pkg:index:Old::child"
    )
    new_child_urn = (
        "urn:pulumi:stack::project::example:index:Component$pkg:index:New::child"
    )
    ctx = pulumi.StateMigrationContext(
        pulumi.StateMigrationArgs(
            root_urn,
            [
                {"urn": root_urn, "type": "example:index:Component"},
                {"urn": child_urn, "type": "pkg:index:Old", "inputs": {}},
            ],
        )
    )
    replacement = {
        "urn": new_child_urn,
        "type": "pkg:index:New",
        "inputs": {"value": "before"},
    }

    prepared = ctx.replace(child_urn, replacement)
    replacement["inputs"]["value"] = "after"
    result = ctx.result()

    assert prepared["inputs"] == {"value": "before"}
    assert result.new_state[1] is prepared
    assert result.successors == {child_urn: new_child_urn}


def test_raw_state_requires_explicit_successor():
    root_urn = "urn:pulumi:stack::project::example:index:Component::component"
    child_urn = (
        "urn:pulumi:stack::project::example:index:Component$pkg:index:Old::child"
    )
    ctx = pulumi.StateMigrationContext(
        pulumi.StateMigrationArgs(
            root_urn,
            [
                {"urn": root_urn, "type": "example:index:Component"},
                {"urn": child_urn, "type": "pkg:index:Old"},
            ],
        )
    )
    ctx.state.pop()

    with pytest.raises(ValueError, match="has no successor"):
        ctx.result()

    ctx.successor(child_urn, root_urn)
    assert ctx.result().successors == {child_urn: root_urn}


def test_repeated_renames_compose_successors():
    old_urn = "urn:pulumi:stack::project::pkg:index:Old::resource"
    intermediate_urn = "urn:pulumi:stack::project::pkg:index:Middle::resource"
    final_urn = "urn:pulumi:stack::project::pkg:index:New::resource"
    ctx = pulumi.StateMigrationContext(
        pulumi.StateMigrationArgs(
            final_urn,
            [{"urn": old_urn, "type": "pkg:index:Old"}],
        )
    )

    intermediate = ctx.rename(old_urn, intermediate_urn)
    final = ctx.rename(intermediate, final_urn)
    result = ctx.result()

    assert final["type"] == "pkg:index:New"
    assert result.successors == {old_urn: final_urn}
