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

# Regresses [pulumi/pulumi#8633]: sequential operations like
# `stack.up` with inline programs should be isolated from each other,
# so that errors from the first operation do not infect the subsequent
# operations.

import asyncio
import os
import sys
import typing
import uuid

import pytest

import pulumi
from pulumi import automation
from .test_utils import get_test_org


class BadResource(pulumi.CustomResource):
    def __init__(
        self, resource_name: str, opts: typing.Optional[pulumi.ResourceOptions] = None
    ):
        if opts is None:
            opts = pulumi.ResourceOptions()
        super().__init__("badprovider::BadResource", resource_name, {}, opts)


def program():
    config = pulumi.Config()
    bad = config.get_int("bad") or 0
    if bad == 1:
        BadResource("bad_resource")


def ignore(*args, **kw):
    pass


def check_isolation(minimal=False):
    stack_name = automation.fully_qualified_stack_name(
        get_test_org(), "isolation-test", f"isolation-test-{uuid.uuid4()}"
    )

    stack = automation.create_stack(
        stack_name=stack_name, project_name="isolation-test", program=program
    )

    with pytest.raises(automation.errors.CommandError):
        stack.set_config("bad", automation.ConfigValue("1"))
        stack.up(on_output=ignore)

    if not minimal:
        stack.set_config("bad", automation.ConfigValue("0"))
        stack.up(on_output=ignore)

    destroy_res = stack.destroy()
    assert destroy_res.summary.kind == "destroy"
    assert destroy_res.summary.result == "succeeded"

    stack.workspace.remove_stack(stack_name)


async def async_stack_up(stack):
    return stack.up(on_output=ignore)


async def async_stack_destroy(stack):
    return stack.destroy()


@pytest.mark.asyncio
async def test_parallel_updates():
    first_stack_name = automation.fully_qualified_stack_name(get_test_org(), "test-parallel", f"stack-{uuid.uuid4()}")
    second_stack_name = automation.fully_qualified_stack_name(get_test_org(), "test-parallel", f"stack-{uuid.uuid4()}")
    stacks = [
        automation.create_stack(
            stack_name, project_name="test-parallel", program=program
        )
        for stack_name in {first_stack_name, second_stack_name}
    ]
    stack_up_responses = await asyncio.gather(
        *[async_stack_up(stack) for stack in stacks]
    )
    assert all(
        {
            stack_response.summary.result == "succeeded"
            for stack_response in stack_up_responses
        }
    )
    stack_destroy_responses = await asyncio.gather(
        *[async_stack_destroy(stack) for stack in stacks]
    )
    assert all(
        {
            stack_response.summary.result == "succeeded"
            for stack_response in stack_destroy_responses
        }
    )


@pytest.mark.skipif(
    sys.platform == "win32", reason="TODO[pulumi/pulumi#8716] fails on Windows"
)
def test_isolation():
    check_isolation()


if __name__ == "__main__":
    import argparse

    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--minimal", action="store_true", help="Minimal test: no sequencing"
    )
    args = ap.parse_args()
    check_isolation(minimal=args.minimal)
