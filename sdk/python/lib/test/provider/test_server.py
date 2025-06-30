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

import functools
from typing import Any, Awaitable, Callable, Dict, List, Optional, Set, Tuple

import grpc
from pulumi import Inputs
from pulumi.errors import (
    InputPropertyErrorDetails,
    RunError,
    InputPropertyError,
    InputPropertiesError,
)
from pulumi.provider.server import ComponentInitError
import pulumi.output
from pulumi.provider import ConstructResult
from pulumi.provider.provider import Provider
import pytest

from google.protobuf import struct_pb2
from pulumi.provider.server import ProviderServicer
from pulumi.resource import CustomResource, ResourceOptions
from pulumi.runtime import Mocks, ResourceModule, proto, rpc
from pulumi.runtime.proto.provider_pb2 import ConstructRequest
from pulumi.runtime.proto import provider_pb2, provider_pb2_grpc, status_pb2, errors_pb2
from pulumi.runtime.settings import Settings, configure
from semver import VersionInfo as Version
from test.provider.grpc_tester import GrpcTestHelper


def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)

    @functools.wraps(wrapped)
    def wrapper(*args, **kwargs):
        configure(Settings("project", "stack"))
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()

        wrapped(*args, **kwargs)

    return wrapper


@pytest.mark.asyncio
async def test_construct_inputs_parses_request():
    value = "foobar"
    inputs = _as_struct({"echo": value})
    req = ConstructRequest(inputs=inputs)
    inputs = await ProviderServicer._construct_inputs(req.inputs, req.inputDependencies)  # pylint: disable=no-member
    assert len(inputs) == 1
    assert inputs["echo"] == value


@pytest.mark.asyncio
async def test_construct_inputs_preserves_unknowns():
    unknown = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
    inputs = _as_struct({"echo": unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = await ProviderServicer._construct_inputs(req.inputs, req.inputDependencies)  # pylint: disable=no-member
    assert len(inputs) == 1
    assert isinstance(inputs["echo"], pulumi.output.Unknown)


def _as_struct(key_values: Dict[str, Any]) -> struct_pb2.Struct:
    the_struct = struct_pb2.Struct()
    the_struct.update(key_values)  # pylint: disable=no-member
    return the_struct


class MockResource(CustomResource):
    def __init__(self, name: str, opts: Optional[ResourceOptions] = None):
        CustomResource.__init__(self, "test:index:MockResource", name, opts=opts)


class MockInputDependencies:
    """A mock for ConstructRequest.inputDependencies

    We need only support a `get() -> T where T.urns: List[str]` operation.
    """

    def __init__(self, urns: Optional[List[str]]):
        self.urns = urns if urns else []

    def get(self, *args):
        # pylint: disable=unused-argument
        return self


class TestModule(ResourceModule):
    def construct(self, name: str, typ: str, urn: str):
        if typ == "test:index:MockResource":
            return MockResource(name, opts=ResourceOptions(urn=urn))
        raise Exception(f"unknown resource type {typ}")

    def version(self) -> Optional[Version]:
        return None


class TestMocks(Mocks):
    def call(self, args: pulumi.runtime.MockCallArgs) -> Any:
        raise Exception(f"unknown function {args.token}")

    def new_resource(
        self, args: pulumi.runtime.MockResourceArgs
    ) -> Tuple[Optional[str], dict]:
        return args.name + "_id", args.inputs


def assert_output_equal(
    value: Any, known: bool, secret: bool, deps: Optional[List[str]] = None
):
    async def check(actual: Any):
        assert isinstance(actual, pulumi.Output)

        if callable(value):
            res = value(await actual.future())
            if isinstance(res, Awaitable):
                await res
        else:
            assert (await actual.future()) == value

        assert known == await actual.is_known()
        assert secret == await actual.is_secret()

        actual_deps: Set[Optional[str]] = set()
        resources = await actual.resources()
        for r in resources:
            urn = await r.urn.future()
            actual_deps.add(urn)

        assert actual_deps == set(deps if deps else [])
        return True

    return check


def create_secret(value: Any):
    return {rpc._special_sig_key: rpc._special_secret_sig, "value": value}


def create_resource_ref(urn: str, id_: Optional[str]):
    ref = {rpc._special_sig_key: rpc._special_resource_sig, "urn": urn}
    if id_ is not None:
        ref["id"] = id_
    return ref


def create_output_value(
    value: Optional[Any] = None,
    secret: Optional[bool] = None,
    dependencies: Optional[List[str]] = None,
):
    val: Dict[str, Any] = {rpc._special_sig_key: rpc._special_output_value_sig}
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
    def __init__(
        self,
        name: str,
        input_: Any,
        deps: Optional[List[str]] = None,
        expected: Optional[Any] = None,
        assert_: Optional[Callable[[Any], Awaitable]] = None,
    ):
        self.name = name
        self.input_ = input_
        self.deps = deps
        self.expected = expected
        self.assert_ = assert_

    async def run(self):
        pulumi.runtime.set_mocks(TestMocks(), "project", "stack", True)
        pulumi.runtime.register_resource_module("test", "index", TestModule())
        # This registers the resource purely for the purpose of the test.
        pulumi.runtime.settings.get_monitor().resources[test_urn] = (
            pulumi.runtime.mocks.MockMonitor.ResourceRegistration(
                test_urn, test_id, dict()
            )
        )

        inputs = {"value": self.input_}
        input_struct = _as_struct(inputs)
        req = ConstructRequest(inputs=input_struct)
        result = await ProviderServicer._construct_inputs(
            req.inputs, MockInputDependencies(self.deps)
        )  # pylint: disable=no-member
        actual = result["value"]
        if self.assert_:
            await self.assert_(actual)
        else:
            assert actual == self.expected


class Assert:
    """Describes a series of asserts to be performed.

    Each assert can be:
    - An async value to be awaited and asserted.
      assert await val

    - A sync function to be called and asserted on. This will be called on the
      same set of arguments that the class was called on.
      assert fn(actual)

    - A plain value to be asserted on.
      assert val

    """

    def __init__(self, *asserts):
        self.asserts = asserts

    async def __call__(self, *args, **kargs):
        for assert_ in self.asserts:
            assert await Assert.__eval(assert_, *args, **kargs)

    @staticmethod
    async def __eval(a, *args, **kargs) -> Any:
        if isinstance(a, Awaitable):
            return await a
        elif isinstance(a, Callable):
            a_res = a(*args, **kargs)
            return await Assert.__eval(a_res, *args, **kargs)
        return a

    @staticmethod
    def async_equal(a, b):
        """Asserts that two values are equal when evaluated with async and
        given the args that `Asserts` were called on.
        """

        async def check(*args, **kargs):
            a_res = await Assert.__eval(a, *args, **kargs)
            b_res = await Assert.__eval(b, *args, **kargs)
            assert a_res == b_res
            return True

        return check


async def array_nested_resource_ref(actual):
    async def helper(v: Any):
        assert isinstance(v, list)
        assert isinstance(v[0], MockResource)
        assert await v[0].urn.future() == test_urn
        assert await v[0].id.future() == test_id

    await assert_output_equal(helper, True, False, [test_urn])(actual)


async def object_nested_resource_ref(actual):
    async def helper(v: Any):
        assert isinstance(v["foo"], MockResource)
        assert await v["foo"].urn.future() == test_urn
        assert await v["foo"].id.future() == test_id

    await assert_output_equal(helper, True, False, [test_urn])(actual)


async def object_nested_resource_ref_and_secret(actual):
    async def helper(v: Any):
        assert isinstance(v["foo"], MockResource)
        assert await v["foo"].urn.future() == test_urn
        assert await v["foo"].id.future() == test_id
        assert v["bar"] == "ssh"

    await assert_output_equal(helper, True, True, [test_urn])(actual)


deserialization_tests = [
    UnmarshalOutputTestCase(
        name="unknown",
        input_=rpc.UNKNOWN,
        deps=["fakeURN"],
        assert_=assert_output_equal(None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested unknown",
        input_=[rpc.UNKNOWN],
        deps=["fakeURN"],
        assert_=assert_output_equal(None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="object nested unknown",
        input_={"foo": rpc.UNKNOWN},
        deps=["fakeURN"],
        assert_=assert_output_equal(None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="unknown output value",
        input_=create_output_value(None, False, ["fakeURN"]),
        deps=["fakeURN"],
        assert_=assert_output_equal(None, False, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="unknown output value (no deps)",
        input_=create_output_value(),
        assert_=assert_output_equal(None, False, False),
    ),
    UnmarshalOutputTestCase(
        name="array nested unknown output value",
        input_=[create_output_value(None, False, ["fakeURN"])],
        deps=["fakeURN"],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal(None, False, False, ["fakeURN"])(
                actual[0]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="array nested unknown output value (no deps)",
        input_=[create_output_value(None, False, ["fakeURN"])],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal(None, False, False, ["fakeURN"])(
                actual[0]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested unknown output value",
        input_={"foo": create_output_value(None, False, ["fakeURN"])},
        deps=["fakeURN"],
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal(None, False, False, ["fakeURN"])(
                actual["foo"]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested unknown output value (no deps)",
        input_={"foo": create_output_value(None, False, ["fakeURN"])},
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal(None, False, False, ["fakeURN"])(
                actual["foo"]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="string value (no deps)",
        input_="hi",
        expected="hi",
    ),
    UnmarshalOutputTestCase(
        name="array nested string value (no deps)",
        input_=["hi"],
        expected=["hi"],
    ),
    UnmarshalOutputTestCase(
        name="object nested string value (no deps)",
        input_={"foo": "hi"},
        expected={"foo": "hi"},
    ),
    UnmarshalOutputTestCase(
        name="string output value",
        input_=create_output_value("hi", False, ["fakeURN"]),
        deps=["fakeURN"],
        assert_=assert_output_equal("hi", True, False, ["fakeURN"]),
    ),
    UnmarshalOutputTestCase(
        name="string output value (no deps)",
        input_=create_output_value("hi"),
        assert_=assert_output_equal("hi", True, False),
    ),
    UnmarshalOutputTestCase(
        name="array nested string output value",
        input_=[create_output_value("hi", False, ["fakeURN"])],
        deps=["fakeURN"],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal("hi", True, False, ["fakeURN"])(
                actual[0]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="array nested string output value (no deps)",
        input_=[create_output_value("hi", False, ["fakeURN"])],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal("hi", True, False, ["fakeURN"])(
                actual[0]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested string output value",
        input_={"foo": create_output_value("hi", False, ["fakeURN"])},
        deps=["fakeURN"],
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal("hi", True, False, ["fakeURN"])(
                actual["foo"]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested string output value (no deps)",
        input_={"foo": create_output_value("hi", False, ["fakeURN"])},
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal("hi", True, False, ["fakeURN"])(
                actual["foo"]
            ),
        ),
    ),
    UnmarshalOutputTestCase(
        name="string secrets (no deps)",
        input_=create_secret("shh"),
        assert_=assert_output_equal("shh", True, True),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secrets (no deps)",
        input_=[create_secret("shh")],
        assert_=assert_output_equal(["shh"], True, True),
    ),
    UnmarshalOutputTestCase(
        name="object nested string secrets (no deps)",
        input_={"foo": create_secret("shh")},
        assert_=assert_output_equal({"foo": "shh"}, True, True),
    ),
    UnmarshalOutputTestCase(
        name="string secret output value (no deps)",
        input_=create_output_value("shh", True),
        assert_=assert_output_equal("shh", True, True),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secret output value (no deps)",
        input_=[create_output_value("shh", True)],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal("shh", True, True)(actual[0]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested string secret output value (no deps)",
        input_={"foo": create_output_value("shh", True)},
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal("shh", True, True)(actual["foo"]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="string secret output value",
        input_=create_output_value("shh", True, ["fakeURN1", "fakeURN2"]),
        deps=["fakeURN1", "fakeURN2"],
        assert_=assert_output_equal("shh", True, True, ["fakeURN1", "fakeURN2"]),
    ),
    UnmarshalOutputTestCase(
        name="string secret output value (no deps)",
        input_=create_output_value("shh", True, ["fakeURN1", "fakeURN2"]),
        assert_=assert_output_equal("shh", True, True, ["fakeURN1", "fakeURN2"]),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secret output value",
        input_=[create_output_value("shh", True, ["fakeURN1", "fakeURN2"])],
        deps=["fakeURN1", "fakeURN2"],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal(
                "shh", True, True, ["fakeURN1", "fakeURN2"]
            )(actual[0]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="array nested string secret output value (no deps)",
        input_=[create_output_value("shh", True, ["fakeURN1", "fakeURN2"])],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: assert_output_equal(
                "shh", True, True, ["fakeURN1", "fakeURN2"]
            )(actual[0]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested string secret output value",
        input_={"foo": create_output_value("shh", True, ["fakeURN1", "fakeURN2"])},
        deps=["fakeURN1", "fakeURN2"],
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal(
                "shh", True, True, ["fakeURN1", "fakeURN2"]
            )(actual["foo"]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested string secret output value (no deps)",
        input_={"foo": create_output_value("shh", True, ["fakeURN1", "fakeURN2"])},
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: assert_output_equal(
                "shh", True, True, ["fakeURN1", "fakeURN2"]
            )(actual["foo"]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="resource ref",
        input_=create_resource_ref(test_urn, test_id),
        deps=[test_urn],
        assert_=Assert(
            lambda actual: isinstance(actual, MockResource),
            Assert.async_equal(lambda actual: actual.urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual.id.future(), test_id),
        ),
    ),
    UnmarshalOutputTestCase(
        name="resource ref (no deps)",
        input_=create_resource_ref(test_urn, test_id),
        assert_=Assert(
            lambda actual: isinstance(actual, MockResource),
            Assert.async_equal(lambda actual: actual.urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual.id.future(), test_id),
        ),
    ),
    UnmarshalOutputTestCase(
        name="array nested resource ref",
        input_=[create_resource_ref(test_urn, test_id)],
        deps=[test_urn],
        assert_=array_nested_resource_ref,
    ),
    UnmarshalOutputTestCase(
        name="array nested resource ref (no deps)",
        input_=[create_resource_ref(test_urn, test_id)],
        assert_=Assert(
            lambda actual: isinstance(actual, list),
            lambda actual: isinstance(actual[0], MockResource),
            Assert.async_equal(lambda actual: actual[0].urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual[0].id.future(), test_id),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref",
        input_={"foo": create_resource_ref(test_urn, test_id)},
        deps=[test_urn],
        assert_=object_nested_resource_ref,
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref (no deps)",
        input_={"foo": create_resource_ref(test_urn, test_id)},
        assert_=Assert(
            lambda actual: isinstance(actual["foo"], MockResource),
            Assert.async_equal(lambda actual: actual["foo"].urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual["foo"].id.future(), test_id),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref and secret",
        input_={
            "foo": create_resource_ref(test_urn, test_id),
            "bar": create_secret("ssh"),
        },
        deps=[test_urn],
        assert_=object_nested_resource_ref_and_secret,
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref and secret output value",
        input_={
            "foo": create_resource_ref(test_urn, test_id),
            "bar": create_output_value("shh", True),
        },
        deps=[test_urn],
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: isinstance(actual["foo"], MockResource),
            Assert.async_equal(lambda actual: actual["foo"].urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual["foo"].id.future(), test_id),
            lambda actual: assert_output_equal("shh", True, True)(actual["bar"]),
        ),
    ),
    UnmarshalOutputTestCase(
        name="object nested resource ref and secret output value (no deps)",
        input_={
            "foo": create_resource_ref(test_urn, test_id),
            "bar": create_output_value("shh", True),
        },
        assert_=Assert(
            lambda actual: not isinstance(actual, pulumi.Output),
            lambda actual: isinstance(actual["foo"], MockResource),
            Assert.async_equal(lambda actual: actual["foo"].urn.future(), test_urn),
            Assert.async_equal(lambda actual: actual["foo"].id.future(), test_id),
            lambda actual: assert_output_equal("shh", True, True)(actual["bar"]),
        ),
    ),
]


@pytest.mark.parametrize(
    "testcase",
    deserialization_tests,
    ids=list(map(lambda x: x.name, deserialization_tests)),
)
@pulumi_test
async def test_deserialize_correctly(testcase):
    await testcase.run()


class MockProvider(Provider):
    def __init__(
        self,
        version: str = "1.0.0",
        error_type: Optional[str] = None,
        error_message: str = "test error",
        property_path: str = "test.property",
    ):
        super().__init__(version)
        self.error_type = error_type
        self.error_message = error_message
        self.property_path = property_path

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: Inputs,
        options: Optional[ResourceOptions] = None,
    ) -> ConstructResult:
        if self.error_type == "run_error":
            raise RunError(self.error_message)
        elif self.error_type == "input_property_error":
            raise InputPropertyError(self.property_path, self.error_message)
        elif self.error_type == "input_properties_error":
            error_details: list[InputPropertyErrorDetails] = [
                {"property_path": self.property_path, "reason": "this is error 1"},
                {"property_path": self.property_path, "reason": "this is error 2"},
            ]
            raise InputPropertiesError(self.error_message, error_details)
        elif self.error_type == "component_init_error":
            inner_error = ValueError(self.error_message)
            raise ComponentInitError(inner_error)

        return ConstructResult(
            urn=f"urn:pulumi:{name}::{resource_type}::test-resource",
            state={"result": "success", "inputs": inputs},
        )


@pytest.mark.asyncio
async def test_construct_success():
    provider = MockProvider()
    servicer = ProviderServicer(provider, [], "")

    async with GrpcTestHelper(servicer) as stub:
        request = proto.ConstructRequest()
        response = await stub.Construct(request)

        assert response.urn.startswith("urn:pulumi:")
        assert "test-resource" in response.urn


@pytest.mark.asyncio
async def test_construct_run_error():
    provider = MockProvider(error_type="run_error", error_message="monitor shut down")
    servicer = ProviderServicer(provider, [], "")

    async with GrpcTestHelper(servicer) as stub:
        request = proto.ConstructRequest()

        try:
            await stub.Construct(request)
            assert False, "Expected RunError to be raised"
        except grpc.aio.AioRpcError as e:
            assert e.code() == grpc.StatusCode.UNKNOWN
            assert "monitor shut down" in e.details()


@pytest.mark.asyncio
async def test_construct_input_property_error():
    provider = MockProvider(
        error_type="input_property_error",
        error_message="Invalid property value",
        property_path="resource.name",
    )
    servicer = ProviderServicer(provider, [], "")

    async with GrpcTestHelper(servicer) as stub:
        request = proto.ConstructRequest()

        try:
            await stub.Construct(request)
            assert False, "Expected InputPropertyError to be raised"
        except grpc.aio.AioRpcError as e:
            assert e.code() == grpc.StatusCode.INVALID_ARGUMENT
            error_details = parse_grpc_error_details(e)
            assert "property_errors" in error_details
            assert len(error_details["property_errors"]) == 1
            prop_error = error_details["property_errors"][0]
            assert prop_error["property_path"] == "resource.name"
            assert prop_error["reason"] == "Invalid property value"


@pytest.mark.asyncio
async def test_construct_input_properties_error():
    provider = MockProvider(
        error_type="input_properties_error",
        error_message="Multiple property validation errors",
        property_path="resource.config",
    )
    servicer = ProviderServicer(provider, [], "")

    async with GrpcTestHelper(servicer) as stub:
        request = proto.ConstructRequest()

        try:
            await stub.Construct(request)
            assert False, "Expected InputPropertiesError to be raised"
        except grpc.aio.AioRpcError as e:
            assert e.code() == grpc.StatusCode.INVALID_ARGUMENT
            error_details = parse_grpc_error_details(e)
            assert "property_errors" in error_details
            assert len(error_details["property_errors"]) == 2
            prop_error_1 = error_details["property_errors"][0]
            prop_error_2 = error_details["property_errors"][1]
            assert prop_error_1["property_path"] == "resource.config"
            assert prop_error_1["reason"] == "this is error 1"
            assert prop_error_2["property_path"] == "resource.config"
            assert prop_error_2["reason"] == "this is error 2"
            assert (
                error_details["status_message"] == "Multiple property validation errors"
            )


@pytest.mark.asyncio
async def test_construct_component_init_error():
    provider = MockProvider(
        error_type="component_init_error",
        error_message="Component initialization failed",
    )
    servicer = ProviderServicer(provider, [], "")

    async with GrpcTestHelper(servicer) as stub:
        request = proto.ConstructRequest()

        try:
            await stub.Construct(request)
            assert False, "Expected ComponentInitError to be raised"
        except grpc.aio.AioRpcError as e:
            assert e.code() == grpc.StatusCode.UNKNOWN
            assert "Component initialization failed" in e.details()


def parse_grpc_error_details(grpc_error: grpc.aio.AioRpcError) -> dict:
    error_details = {
        "message": grpc_error.details(),
        "status_message": "",
        "property_errors": [],
    }

    trailing_metadata = grpc_error.trailing_metadata()
    if trailing_metadata:
        for key, value in trailing_metadata:
            if key == "grpc-status-details-bin":
                status = status_pb2.Status()
                status.ParseFromString(value)
                error_details["status_message"] = status.message

                for detail in status.details:
                    if detail.Is(errors_pb2.InputPropertiesError.DESCRIPTOR):
                        input_error = errors_pb2.InputPropertiesError()
                        detail.Unpack(input_error)

                        for prop_error in input_error.errors:
                            error_details["property_errors"].append(
                                {
                                    "property_path": prop_error.property_path,
                                    "reason": prop_error.reason,
                                }
                            )

    return error_details
