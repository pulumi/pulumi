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

from pulumi.resource import ResourceTransformArgs
from pulumi.runtime._callbacks import _CallbackServicer
from pulumi.runtime.proto.provider_pb2 import InvokeRequest
from pulumi.runtime.proto.resource_pb2_grpc import ResourceMonitorServicer

from ..grpc_stubs import monitor_servicer_stub, callback_servicer_stub


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
