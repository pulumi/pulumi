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

import grpc
import pytest
from google.protobuf import struct_pb2

import pulumi
from pulumi.resource import ResourceTransformArgs
from pulumi.resource_hooks import ErrorHook, ResourceHook
from pulumi.runtime import settings
from pulumi.runtime._callbacks import _CallbackServicer
from pulumi.runtime.proto import resource_pb2
from pulumi.runtime.proto.provider_pb2 import InvokeRequest
from pulumi.runtime.proto.resource_pb2_grpc import ResourceMonitorServicer
from pulumi.runtime.settings import Settings

from ..grpc_stubs import monitor_servicer_stub, callback_servicer_stub


def _unregistered_resource_hook(name, callback):
    # Use __new__ to bypass __init__, which would register the hook.
    hook = ResourceHook.__new__(ResourceHook)
    hook.name = name
    hook.callback = callback
    hook.opts = None
    return hook


def _unregistered_error_hook(name, callback):
    # Use __new__ to bypass __init__, which would register the hook.
    hook = ErrorHook.__new__(ErrorHook)
    hook.name = name
    hook.callback = callback
    return hook


class _InvokeMonitor:
    def Invoke(self, _request):
        result = struct_pb2.Struct()
        result.update({"value": "ok"})
        return resource_pb2.ResourceInvokeResponse(**{"return": result})


def _untracked_callback_servicer(monitor):
    servicer = _CallbackServicer(monitor)
    # These tests invoke callbacks directly or manage the server themselves. Do
    # not leak their servicers into the runtime's global shutdown registry.
    _CallbackServicer._servicers.remove(servicer)
    return servicer


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
        servicer = _untracked_callback_servicer(monitor_stub)
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
@pytest.mark.parametrize("hook_kind", ["resource", "error"])
async def test_hooks_reject_resource_construction(hook_kind):
    urn = "urn:pulumi:stack::project::test:index:Resource::example"

    async def construct_resource(_args):
        # ContextVars must keep the restriction active after an async suspension.
        await asyncio.sleep(0)
        pulumi.ComponentResource("test:index:Component", "inside-hook")

    servicer = _untracked_callback_servicer(_InvokeMonitor())

    if hook_kind == "resource":
        hook = _unregistered_resource_hook("mutating-hook", construct_resource)
        registration = servicer.do_register_resource_hook(hook)
        request = resource_pb2.ResourceHookRequest(urn=urn)
    else:
        hook = _unregistered_error_hook("mutating-hook", construct_resource)
        registration = servicer.do_register_error_hook(hook)
        request = resource_pb2.ErrorHookRequest(urn=urn)

    callback = servicer._callbacks[registration.callback.token]
    response = await callback(request.SerializeToString())

    assert (
        "Pulumi runtime operation 'resource construction' is not allowed"
        in response.error
    )
    assert f"{hook_kind} hook 'mutating-hook'" in response.error
    assert urn in response.error


@pytest.mark.asyncio
async def test_resource_hook_allows_provider_invokes():
    monitor = _InvokeMonitor()
    settings.configure(Settings(project="project", stack="stack", monitor=monitor))
    result = None

    async def invoke(_args):
        nonlocal result
        await asyncio.sleep(0)
        result = await pulumi.runtime.invoke_async("test:index:getThing", {})

    hook = _unregistered_resource_hook("invoking-hook", invoke)
    servicer = _untracked_callback_servicer(monitor)
    registration = servicer.do_register_resource_hook(hook)
    callback = servicer._callbacks[registration.callback.token]
    response = await callback(
        resource_pb2.ResourceHookRequest(
            urn="urn:pulumi:stack::project::test:index:Resource::example"
        ).SerializeToString()
    )

    assert response.error == ""
    assert result == {"value": "ok"}
