# Copyright 2025, Pulumi Corporation.
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

import asyncio
import json

import grpc
import pytest

import pulumi
from pulumi.resource import (
    ResourceTransformArgs,
    StateMigrationArgs,
    StateMigrationResult,
)
from pulumi.runtime import resource as runtime_resource
from pulumi.runtime import settings
from pulumi.runtime._callbacks import _CallbackServicer
from pulumi.runtime.proto.callback_pb2 import Callback, CallbackInvokeRequest
from pulumi.runtime.proto.provider_pb2 import InvokeRequest
from pulumi.runtime.proto.resource_pb2 import (
    RegisterResourceResponse,
    StateMigrationRequest,
    StateMigrationResponse,
    SupportsFeatureResponse,
)
from pulumi.runtime.proto.resource_pb2_grpc import ResourceMonitorServicer
from pulumi.runtime.settings import Settings

from ..grpc_stubs import monitor_servicer_stub, callback_servicer_stub


class _RecordingMonitor:
    def __init__(self):
        self.requests = []

    def RegisterResource(self, request):
        self.requests.append(request)
        return RegisterResourceResponse(
            urn=f"urn:pulumi:stack::project::{request.type}::{request.name}",
            object=request.object,
        )

    def SupportsFeature(self, _request):
        return SupportsFeatureResponse(hasSupport=True)


class _RecordingCallbacks:
    def __init__(self):
        self.migrations = []

    def register_state_migration(self, migration):
        self.migrations.append(migration)
        return Callback(
            target="127.0.0.1:1234", token=f"migration-{len(self.migrations)}"
        )


def _configure_registration_test(monitor, supports_state_migrations):
    test_settings = Settings(project="project", stack="stack", monitor=monitor)
    test_settings.feature_support = {
        "stateMigrations": supports_state_migrations,
    }
    settings.configure(test_settings)


@pytest.mark.asyncio
# This test will hang indefinitely if we don't abort the GRPC connection
@pytest.mark.timeout(60)
async def test_callback_servicer_transform_errors():
    """
    Tests that the callbacks server returns an error when a callback fails.
    Special care needs to be take to handle asyncio task cancellation since
    CancelledError does not derive from Exception.
    """

    def transform_exception(args: ResourceTransformArgs):
        """A transform that raises an exception."""
        raise Exception("beep")

    async def transform_cancelled_error(args: ResourceTransformArgs):
        """A transform that raises a cancelled error."""
        coro = asyncio.sleep(10)
        await asyncio.sleep(0)
        coro.throw(asyncio.CancelledError("noes"))

    async with monitor_servicer_stub(ResourceMonitorServicer()) as monitor_stub:
        servicer = _CallbackServicer(monitor_stub)
        servicer._servicers.remove(
            servicer
        )  # Remove this servicer from the global list, we manage the shutdown ourselves
        cb_exception = servicer.register_transform(transform_exception)
        cb_cancelled = servicer.register_transform(transform_cancelled_error)

        async with callback_servicer_stub(servicer) as stub:
            request = InvokeRequest(tok=cb_exception.token)
            try:
                await stub.Invoke(request)
                assert False, "should have raised"
            except Exception as e:
                # The error we get via GRPC has the file, function name and exception
                assert "lib/test/runtime/test_callbacks.py" in str(e)
                assert "in transform_exception" in str(e)
                assert 'Exception("beep")' in str(e)

            request = InvokeRequest(tok=cb_cancelled.token)
            try:
                await stub.Invoke(request)
                assert False, "should have raised"
            except Exception as e:
                # The error we get via GRPC has the file, function name and exception
                assert "lib/test/runtime/test_callbacks.py" in str(e)
                assert "in transform_cancelled_error" in str(e)
                assert 'CancelledError("noes")' in str(e)

            await servicer.shutdown()


@pytest.mark.asyncio
@pytest.mark.timeout(60)
async def test_callback_servicer_state_migration():
    """
    Tests that state migration callbacks round-trip the checkpoint-format JSON state and the
    successor mapping, including a many-to-one fold, and that returning None leaves the state
    unchanged.
    """

    comp_urn = "urn:pulumi:stack::proj::my:module:Comp::comp"
    child_a_urn = "urn:pulumi:stack::proj::my:module:Comp$pkg:m:typA::childA"
    child_b_urn = "urn:pulumi:stack::proj::my:module:Comp$pkg:m:typA::childB"
    child_c_urn = "urn:pulumi:stack::proj::my:module:Comp$pkg:m:typA::childC"
    old_state = [
        {"urn": comp_urn, "type": "my:module:Comp"},
        {"urn": child_a_urn, "type": "pkg:m:typA", "custom": True, "id": "id-a"},
        {"urn": child_b_urn, "type": "pkg:m:typA", "custom": True, "id": "id-b"},
    ]

    def migrate_fold(args: StateMigrationArgs):
        assert args.urn == comp_urn
        assert args.old_state == old_state
        new_state = [dict(args.old_state[0]), dict(args.old_state[1])]
        new_state[1]["urn"] = child_c_urn
        return StateMigrationResult(
            new_state=new_state,
            successors={child_a_urn: child_c_urn, child_b_urn: child_c_urn},
        )

    async def migrate_noop(args: StateMigrationArgs):
        await asyncio.sleep(0)
        future = asyncio.get_running_loop().create_future()
        asyncio.get_running_loop().call_soon(future.set_result, "done")
        assert await future == "done"
        return None

    async with monitor_servicer_stub(ResourceMonitorServicer()) as monitor_stub:
        servicer = _CallbackServicer(monitor_stub)
        servicer._servicers.remove(
            servicer
        )  # Remove this servicer from the global list, we manage the shutdown ourselves
        cb_fold = servicer.register_state_migration(migrate_fold)
        cb_noop = servicer.register_state_migration(migrate_noop)

        # Registering the same callable again returns the same token.
        assert servicer.register_state_migration(migrate_fold).token == cb_fold.token

        async with callback_servicer_stub(servicer) as stub:
            migration_request = StateMigrationRequest(
                urn=comp_urn, old_state=json.dumps(old_state).encode("utf-8")
            )
            request = CallbackInvokeRequest(
                token=cb_fold.token, request=migration_request.SerializeToString()
            )
            result = await stub.Invoke(request)
            response = StateMigrationResponse.FromString(result.response)
            new_state = json.loads(response.new_state)
            assert new_state[0]["urn"] == comp_urn
            assert new_state[1]["urn"] == child_c_urn
            assert dict(response.successors) == {
                child_a_urn: child_c_urn,
                child_b_urn: child_c_urn,
            }

            request = CallbackInvokeRequest(
                token=cb_noop.token, request=migration_request.SerializeToString()
            )
            result = await stub.Invoke(request)
            response = StateMigrationResponse.FromString(result.response)
            # An unchanged state is signaled by an empty new_state.
            assert response.new_state == b""
            assert len(response.successors) == 0

            await servicer.shutdown()


@pytest.mark.asyncio
async def test_state_migration_registration_wires_callback_to_field_43(monkeypatch):
    monitor = _RecordingMonitor()
    callbacks = _RecordingCallbacks()
    _configure_registration_test(monitor, supports_state_migrations=True)

    async def get_callbacks():
        return callbacks

    monkeypatch.setattr(runtime_resource, "_get_callbacks", get_callbacks)

    def migrate(_args):
        return None

    resource = pulumi.ComponentResource(
        "test:index:Component",
        "component",
        opts=pulumi.ResourceOptions(state_migrations=[migrate]),
    )
    await resource.urn.future()

    assert callbacks.migrations == [migrate]
    assert len(monitor.requests) == 1
    request = monitor.requests[0]
    assert request.DESCRIPTOR.fields_by_name["state_migrations"].number == 43
    assert [
        (callback.target, callback.token) for callback in request.state_migrations
    ] == [("127.0.0.1:1234", "migration-1")]


@pytest.mark.asyncio
async def test_state_migration_registration_rejects_unsupported_monitor(monkeypatch):
    monitor = _RecordingMonitor()
    callbacks = _RecordingCallbacks()
    _configure_registration_test(monitor, supports_state_migrations=False)

    async def get_callbacks():
        return callbacks

    monkeypatch.setattr(runtime_resource, "_get_callbacks", get_callbacks)

    resource = pulumi.ComponentResource(
        "test:index:Component",
        "component",
        opts=pulumi.ResourceOptions(state_migrations=[lambda _args: None]),
    )
    with pytest.raises(Exception, match="does not support state migrations"):
        await resource.urn.future()

    assert callbacks.migrations == []
    assert monitor.requests == []


@pytest.mark.asyncio
@pytest.mark.timeout(60)
async def test_state_migration_rejects_pulumi_runtime_operations():
    comp_urn = "urn:pulumi:stack::proj::my:module:Comp::comp"
    migration_request = StateMigrationRequest(
        urn=comp_urn,
        old_state=json.dumps([{"urn": comp_urn, "type": "my:module:Comp"}]).encode(
            "utf-8"
        ),
    )

    def construct_resource(args: StateMigrationArgs):
        pulumi.ComponentResource("test:index:Component", "inside-migration")

    async def invoke(args: StateMigrationArgs):
        await asyncio.sleep(0)
        await pulumi.runtime.invoke_async("test:index:getThing", {})

    def invoke_output(args: StateMigrationArgs):
        pulumi.runtime.invoke_output("test:index:getThing", {})

    def register_transform(args: StateMigrationArgs):
        pulumi.runtime.register_resource_transform(lambda transform_args: None)

    migrations = [
        ("resource construction", construct_resource),
        ("invoke", invoke),
        ("invoke", invoke_output),
        ("register resource transform", register_transform),
    ]

    async with monitor_servicer_stub(ResourceMonitorServicer()) as monitor_stub:
        servicer = _CallbackServicer(monitor_stub)
        servicer._servicers.remove(
            servicer
        )  # Remove this servicer from the global list, we manage the shutdown ourselves

        async with callback_servicer_stub(servicer) as stub:
            for operation, migration in migrations:
                callback = servicer.register_state_migration(migration)
                request = CallbackInvokeRequest(
                    token=callback.token,
                    request=migration_request.SerializeToString(),
                )

                with pytest.raises(grpc.aio.AioRpcError) as error:
                    await asyncio.wait_for(stub.Invoke(request), timeout=5)

                assert error.value.code() == grpc.StatusCode.UNKNOWN
                details = error.value.details()
                assert details is not None
                assert (
                    f"Pulumi runtime operation '{operation}' is not allowed inside "
                    "the state migration callback"
                ) in details
                assert comp_urn in details

            await servicer.shutdown()
