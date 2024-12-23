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

"""Verifies that type-related mistakes in dynamic providers result in
exceptions and not hangs. Regresses
https://github.com/pulumi/pulumi/issues/6981

"""

import contextlib
from typing import Dict
import uuid
import pytest

from pulumi import Input, Output
from pulumi.runtime import settings, mocks
import pulumi
import pulumi.dynamic as dyn

from .helpers import raises


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        result = XProvider().create(args=args.inputs)
        return [result.id, result.outs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


@pytest.fixture
def my_mocks():
    old_settings = settings.SETTINGS
    mm = MyMocks()
    mocks.set_mocks(mm, preview=False)
    try:
        yield mm
    finally:
        settings.configure(old_settings)


class XInputs(object):
    x: Input[Dict[str, str]]

    def __init__(self, x):
        self.x = x


class XProvider(dyn.ResourceProvider):
    def create(self, args):
        # intentional bug changing the type
        outs = {"x": {"my_key_1": {"extra_buggy_key": args["x"]["my_key_1"] + "!"}}}
        return dyn.CreateResult(f"schema-{uuid.uuid4()}", outs=outs)


class X(dyn.Resource):
    x: Output[Dict[str, str]]

    def __init__(self, name: str, args: XInputs, opts=None):
        super().__init__(XProvider(), name, vars(args), opts)


@raises(AssertionError)
@pytest.mark.timeout(10)
@pulumi.runtime.test
def test_pulumi_broken_dynamic_provider(my_mocks):
    x = X(name="my_x", args=XInputs({"my_key_1": "my_value_1"}))
    return x.x.apply(print)
