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

from typing import Dict, Any, Optional, Tuple, List, Set, Callable, Awaitable

import os
import pytest
from pulumi.runtime.settings import Settings, configure
from pulumi.provider.server import ProviderServicer
from pulumi.runtime import proto, rpc, rpc_manager, ResourceModule, Mocks
from pulumi.resource import CustomResource
from pulumi.runtime.proto.provider_pb2 import ConstructRequest
from google.protobuf import struct_pb2
import pulumi.output


@pytest.mark.asyncio
async def test_construct_inputs_parses_request():
    value = 'foobar'
    inputs = _as_struct({'echo': value})
    req = ConstructRequest(inputs=inputs)
    inputs = await ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    assert inputs['echo'] == value


@pytest.mark.asyncio
async def test_construct_inputs_preserves_unknowns():
    unknown = '04da6b54-80e4-46f7-96ec-b56ff0331ba9'
    inputs = _as_struct({'echo': unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = await ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    assert isinstance(inputs['echo'], pulumi.output.Unknown)


def _as_struct(key_values: Dict[str, Any]) -> struct_pb2.Struct:
    the_struct = struct_pb2.Struct()
    the_struct.update(key_values)  # pylint: disable=no-member
    return the_struct

class MockResource(CustomResource):
    def __init__(self, name: str, *opts, **kopts):
        super().__init__("test:index:MockResource", name, None, *opts, **kopts)


class TestModule(ResourceModule):
    def construct(self, name: str, typ: str, urn: str):
        if typ == "test:index:MockResource":
            return MockResource(name, urn=urn)
        raise Exception(f"unknown resource type {typ}")

    def version(self) -> Optional:
        return None

class TestMocks(Mocks):
    def call(self, args: pulumi.runtime.MockCallArgs) -> Any:
        raise Exception(f"unknown function {args.token}")

    def new_resource(self, args: pulumi.runtime.MockResourceArgs) -> Tuple[Optional[str], Any]:
        return (args.name+"_id", args.inputs)

async def assert_output_equal(actual: Any, value: Any, known: bool, secret: bool, deps: Optional[List[str]] = None):
    assert isinstance(actual, pulumi.Output)

    # TODO: ensure that actual.promise() translates
    if callable(value):
        value(await actual.promise())
    else:
        assert (await actual.promise()) == value

    assert known == await actual.is_known
    assert secret == await actual.is_secret

    actual_deps: Set[str] = set()
    resources = await actual.all_resources()
    for r in resources:
        urn = await r.urn.promise()
        actual_deps.add(urn)

    if deps is None:
        deps = []
    assert actual_deps == set(deps)


def create_secret(value: Any):
    return {rpc._special_sig_key: rpc._special_secret_sig, "value": value}

def create_resource_ref(urn: str, id_: Optional[str]):
    ref = {rpc._special_sig_key: rpc._special_resource_sig, "urn": urn}
    if id_:
        ref["id"] = id_
    return ref

def create_output_value(value: Optional[Any], secret: Optional[bool] = None, dependencies: Optional[List[str]] = None):
    val =  {rpc._special_sig_key: rpc._special_output_value_sig}
    if value is not None:
        val["value"] = value
    if secret is not None:
        val["secret"] = secret
    if dependencies is not None:
        val["dependencies"] = dependencies
    return val

test_urn = "urn:pulumi:stack::project::test:index:MockResource::name"
test_id = "name_id"

class UnmarshalOutputTestCase:
    def __init__(self,
                 input_: Any,
                 deps: Optional[List[str]],
                 expected: Optional[Any] = None,
                 assert_: Optional[Callable[[Any], Awaitable]] = None):
        self.input_ = input_
        self.deps = deps
        self.expected = expected
        self.assert_ = assert_

    @staticmethod
    def before_each():
        configure(Settings())
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()
        rpc_manager.RPC_MANAGER = rpc_manager.RPCManager()

    async def run(self):
        self.before_each()
        pulumi.runtime.set_mocks(TestMocks(), "project", "stack", True)
        pulumi.runtime.register_resource_module("test", "index", TestModule())
        test_resource = MockResource("name") # TODO: this doesn't make sense to me. Is it
                             # grabbing something from pulumi.runtime?

        inputs = { "value": self.input_ }
        input_struct = _as_struct(inputs)
        req = ConstructRequest(inputs=input_struct)
        # TODO: I'm unsure about this:
        for el in self.deps if self.deps else []:
            req.inputDependencies.get_or_create(el)
        result = await ProviderServicer._construct_inputs(req)
        actual = result["value"]
        if self.assert_:
            await self.assert_(actual)
        else:
            assert actual == self.expected


@pytest.mark.asyncio
async def test_deserialize_unkwown():
    await UnmarshalOutputTestCase(
        input_=rpc.UNKNOWN,
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"])).run()

@pytest.mark.asyncio
async def test_array_nested_unknown():
    await UnmarshalOutputTestCase(
        input_=[rpc.UNKNOWN],
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"])).run()

@pytest.mark.asyncio
async def test_object_nested_unknown():
    await UnmarshalOutputTestCase(
        input_={"foo": rpc.UNKNOWN},
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"])).run()
