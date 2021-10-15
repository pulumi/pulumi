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
    inputs = await ProviderServicer._construct_inputs(req.inputs, req.inputDependencies)
    assert len(inputs) == 1
    assert inputs['echo'] == value


@pytest.mark.asyncio
async def test_construct_inputs_preserves_unknowns():
    unknown = '04da6b54-80e4-46f7-96ec-b56ff0331ba9'
    inputs = _as_struct({'echo': unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = await ProviderServicer._construct_inputs(req.inputs, req.inputDependencies)
    assert len(inputs) == 1
    assert isinstance(inputs['echo'], pulumi.output.Unknown)


def _as_struct(key_values: Dict[str, Any]) -> struct_pb2.Struct:
    the_struct = struct_pb2.Struct()
    the_struct.update(key_values)  # pylint: disable=no-member
    return the_struct

class MockResource(CustomResource):
    def __init__(self, name: str, *opts, **kopts):
        super().__init__("test:index:MockResource", name, None, *opts, **kopts)

class MockInputDependencies:
    """
    """
    def __init__(self, urns: Optional[List[str]]):
        self.urns = urns if urns else []

    # We intentionally ignore args
    def get(self, *args):
        return self

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

    if callable(value):
        value(await actual.future())
    else:
        assert (await actual.future()) == value

    assert known == await actual.is_known()
    assert secret == await actual.is_secret()

    actual_deps: Set[str] = set()
    resources = await actual.resources()
    for r in resources:
        urn = await r.urn.future()
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
                 name: str,
                 input_: Any,
                 deps: Optional[List[str]] = None,
                 expected: Optional[Any] = None,
                 assert_: Optional[Callable[[Any], Awaitable]] = None):
        self.name = name
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
        test_resource = MockResource("name") # TODO: this doesn't make sense to me. Is it grabbing
                                             # something from pulumi.runtime?

        inputs = { "value": self.input_ }
        input_struct = _as_struct(inputs)
        req = ConstructRequest(inputs=input_struct)
        result = await ProviderServicer._construct_inputs(req.inputs, MockInputDependencies(self.deps))
        actual = result["value"]
        if self.assert_:
            await self.assert_(actual)
        else:
            assert actual == self.expected

deserialization_tests = [
    UnmarshalOutputTestCase(
        name="unknown",
        input_=rpc.UNKNOWN,
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested unknown",
        input_=[rpc.UNKNOWN],
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="object nested unknown",
        input_={"foo": rpc.UNKNOWN},
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="unknown output value",
        input_=create_output_value(),
        assert_=lambda actual: assert_output_equal(actual, None, False, False),
    ),
    UnmarshalOutputTestCase(
        name="unknown output value deps",
        input=create_output_value(None, False, ["fakeURN"]),
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested unknown output value deps",
        input_=[create_output_value(None, False, ["fakeURN"])],
        deps=["fakeURN"],
        assert_=array_nested_unknown_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="object nested unknown output value deps",
        input_= { "foo": create_output_value(None, False, ["fakeURN"]) },
        deps=["fakeURN"],
        assert_=object_nested_unknown_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="string value no deps",
        input_="hi",
        expected="hi",
    ),
    UnmarshalOutputTestCase(
        name="array nested string value no deps",
        input_=["hi"],
        expected=["hi"],
    ),
    UnmarshalOutputTestCase(
        name="object nested string value no deps",
        input_= { "foo": "hi" },
        expected= { "foo": "hi" },
    ),
    UnmarshalOutputTestCase(
        name="string output value no deps",
        input_=create_output_value("hi"),
        assert_=lambda actual: assert_output_equal(actual, "hi", True, False),
    ),
    UnmarshalOutputTestCase(
        name="string output value deps",
        input_=create_output_value("hi", False, ["fakeURN"]),
        deps=["fakeURN"],
        assert_=lambda actual: assert_output_equal(actual, "hi", True, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested string output value deps",
        input_=[created_output_value("hi", False, ["fakeURN"])],
        deps=["fakeURN"],
        assert_=array_nested_string_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="object nested string output value deps",
        input_={ "foo": created_output_value("hi", False, ["fakeURN"])},
        deps=["fakeURN"],
        assert_=array_nested_string_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="string secrets",
        input_=create_secret("shh"),
        assert_=lambda actual: assert_output_equal(actual, "shh", True, True),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secrets",
        input_=[create_secret("shh")],
        assert_=lambda actual: assert_output_equal(actual, ["shh"], True, True),
    ),
    UnmarshalOutputTestCase(
        name="object nested string secrets",
        input_={ "foo": create_secret("shh")},
        assert_=lambda actual: assert_output_equal(actual, {"foo": "shh"}, True, True),
    ),
    UnmarshalOutputTestCase(
        name="string secrets output value",
        input_=create_secret("shh", True),
        assert_=lambda actual: assert_output_equal(actual, "shh", True, True),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secrets output value",
        input_=[create_secret("shh", True)],
        assert_=array_nested_string_secrets_output_value,
    ),
    UnmarshalOutputTestCase(
        name="object nested string secrets output value",
        input_={ "foo": create_secret("shh", True)},
        assert_=lambda actual: assert_output_equal(actual, {"foo": "shh"}, True, True),
    ),
    UnmarshalOutputTestCase(
        name="string secret output value deps",
        input_=create_output_value("shh", True, ["fakeURN1", "fakeURN2"]),
        deps=["fakeURN1", "fakeURN2"],
        assert_=lambda actual: assert_output_equal(actual, "shh", True, True, ["fakeURN1", "fakeURN2"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secret output value deps",
        input_=[create_output_value("shh", True, ["fakeURN1", "fakeURN2"])],
        deps=["fakeURN1", "fakeURN2"],
        assert_=array_nested_string_secret_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="object nested string secret output value deps",
        input_={ "foo": create_output_value("shh", True, ["fakeURN1", "fakeURN2"])},
        deps=["fakeURN1", "fakeURN2"],
        assert_=object_nested_string_secret_output_value_deps,
    ),
    UnmarshalOutputTestCase(
        name="resource ref",
        input_=create_resource_ref(test_urn, test_id),
        deps=[test_urn],
        assert_=resource_ref,
    ),
    UnmarshalOutputTestCase(
        name="array nested resource ref",
        input_=[create_resource_ref(test_urn, test_id)],
        deps=[test_urn],
        assert_=array_nested_resource_ref,
    ),
     UnmarshalOutputTestCase(
        name="object nested resource ref",
        input_={ "foo": create_resource_ref(test_urn, test_id) },
        deps=[test_urn],
        assert_=object_nested_resource_ref,
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref and secret",
        input_={
            "foo": create_resource_ref(test_urn, test_id),
            "bar": create_secret("shh"),
        },
        deps=[test_urn],
        assert_=object_nested_resource_ref_and_secret
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref and secret output value",
        input_={
            "foo": create_resource_ref(test_urn, test_id),
            "bar": create_output_value("shh", True),
        },
        deps=[test_urn],
        assert_=object_nested_resource_ref_and_secret_output_value,
    ),
]

async def object_nested_resource_ref_and_secret_output_value(actual):
    assert not isinstance(actual, pulumi.Output)
    assert isinstance(actual["foo"], MockResource)
    assert await actual["foo"].urn.future() == test_urn
    assert await actual["foo"].id.future() == test_id
    await assert_output_equal(actual["bar"], "shh", True, True)

async def object_nested_resource_ref_and_secret(actual):
    async def helper(v: Any):
        assert isinstance(v["foo"], MockResource)
        assert await v["foo"].urn.future() == test_urn
        assert await v["foo"].id.future() == test_id
        assert v.bar == "ssh"
    await assert_output_equal(actual, helper, True, True, [test_urn])

async def object_nested_resource_ref(actual):
    assert isinstance(actual["foo"], MockResource)
    assert await actual["foo"].urn.future() == test_urn
    assert await actual["foo"].id().future() == test_id

async def array_nested_resource_ref(actual):
    assert isinstance(actual, list)
    assert isinstance(actual[0], MockResource)
    assert await actual[0].urn.future() == test_urn
    assert await actual[0].id().future() == test_id

async def resource_ref(actual):
    assert isinstance(actual, MockResource)
    assert await actual.urn.future() == test_urn
    assert await actual.id().future() == test_id

async def object_nested_string_secret_output_value_deps(actual):
    assert isinstance(actual, list)
    await assert_output_equal(actual["foo"], "shh", True, True, ["fakeURN1", "fakeURN2"])

async def array_nested_string_secret_output_value_deps(actual):
    assert isinstance(actual, list)
    await assert_output_equal(actual[0], "shh", True, True, ["fakeURN1", "fakeURN2"])

async def object_nested_string_secrets_output_value(actual):
    assert not isinstance(actual, pulumi.Output)
    await assert_output_equal(actual["foo"], "shh", True, True)

async def array_nested_string_secrets_output_value(actual):
    assert isinstance(actual, list)
    await assert_output_equal(actual[0], "shh", True, True)

async def object_nested_string_output_value_deps(actual):
    assert not isinstance(actual, pulumi.Output)
    await assert_output_equal(actual["foo"], "hi", False, False, ["fakeURN"])


async def array_nested_string_output_value_deps(actual):
    assert isinstance(actual, list)
    await assert_output_equal(actual[0], "hi", True, False, ["fakeURN"])

async def object_nested_unknown_output_value_deps(actual):
    assert not isinstance(actual, pulumi.Output)
    await assert_output_equal(actual["foo"], None, False, False, ["fakeURN"])

async def array_nested_unknown_output_value_deps(actual):
    assert isinstance(actual, list)
    await assert_output_equal(actual[0], None, False, False, ["fakeURN"])

@pytest.mark.parametrize(
    "testcase", deserialization_tests, map(lambda x: x.name, deserialization_tests))
@pytest.mark.asyncio
async def test_deserialize_correctly(testcase):
    await testcase.run()
