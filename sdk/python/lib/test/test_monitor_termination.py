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

from typing import Any, Optional
import asyncio
import functools
import logging
import grpc
import pytest

import pulumi

from .helpers import raises


# Verify that when the monitor becomes unavailable (via
# unavailable_mocks), programs fail with a `RunError` and do not hang.
@raises(pulumi.RunError)
@pytest.mark.timeout(10)
@pulumi.runtime.test
def test_resource_registration_does_not_hang_when_monitor_unavailable(
    unavailable_mocks,
):
    MyCustom("mycustom", {"inprop": "hello"})


class Unavailable(grpc.RpcError):
    def code(self):
        return grpc.StatusCode.UNAVAILABLE


class UnavailableMocks(pulumi.runtime.Mocks):
    def call(self, args: pulumi.runtime.MockCallArgs) -> Any:
        return {}

    def new_resource(self, args: pulumi.runtime.MockResourceArgs) -> Any:
        raise Unavailable()


class MyCustom(pulumi.CustomResource):
    outprop: pulumi.Output[str]

    def __init__(self, resource_name, props: Optional[dict] = None, opts=None) -> None:
        super().__init__("pkg:index:MyCustom", resource_name, props, opts)
        inprop = (props or {}).get("inprop", None)
        if inprop is None:
            raise TypeError("Missing required property 'inprop'")
        self.outprop = pulumi.Output.from_input(inprop).apply(lambda x: f"output: {x}")


@pytest.fixture
def unavailable_mocks():
    old_settings = pulumi.runtime.settings.SETTINGS
    try:
        mocks = UnavailableMocks()
        pulumi.runtime.mocks.set_mocks(mocks)
        yield mocks
    finally:
        pulumi.runtime.settings.configure(old_settings)
