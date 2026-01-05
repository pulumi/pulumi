# Copyright 2016-2021, Pulumi Corporation.
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

"""Verifies that a Stack always calls RegisterResourceOutputs even if
there are no outputs. This makes sure removing stack outputs from a
program actually deletes them from the stack.

Regresses https://github.com/pulumi/pulumi/issues/8273

"""

from copy import deepcopy
from typing import Optional

from pulumi.runtime.stack import Stack
import pytest
import pytest_asyncio
from pulumi.runtime import get_root_resource, settings, mocks
import pulumi


class MockResource(pulumi.CustomResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("python:test:MockResource", name, {}, opts)


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        raise Exception("call")


class MyMonitor(mocks.MockMonitor):
    def __init__(self, args):
        super().__init__(args)
        self.outputs = None

    def RegisterResourceOutputs(self, outputs):
        self.outputs = outputs


@pytest_asyncio.fixture
async def my_mocks():
    settings.reset_options()
    old_settings = deepcopy(settings.SETTINGS)
    mm = MyMocks()
    monitor = MyMonitor(mm)
    mocks.set_mocks(mm, preview=False, monitor=monitor)

    try:
        yield mm
    finally:
        settings.configure(old_settings)

        # Asserts are here in an unusual place since it is tricky to
        # place them inside a test and make the code run after the
        # test stack completes constructing.
        assert monitor.outputs is not None
        assert isinstance(monitor.outputs.urn, str)


@pulumi.runtime.test
@pytest.mark.asyncio
async def test_component_registers_outputs(my_mocks):
    MockResource(name="res")


@pytest.mark.asyncio
async def test_stack_registers_outputs():
    settings.reset_options()
    old_settings = deepcopy(settings.SETTINGS)

    def program():
        pulumi.export("fruit", "banana")

    try:
        Stack(program)
        stack = get_root_resource()
        assert stack.outputs is not None
        assert stack.outputs["fruit"] == "banana"
    finally:
        settings.configure(old_settings)
