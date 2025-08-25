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

"""
Provides GRPC stubs that can be used to test GRPC calls. The async context
managers start a real GRPC server and return a stub object that can be used to
make GRPC calls.

>>> provider = MockProvider()
>>> servicer = ProviderServicer(provider, [], "")
>>>
>>> async with provider_servicer_stub(servicer) as stub:
>>>     request = proto.ConstructRequest()
>>>     response = await stub.Construct(request)
>>>     assert response.urn.startswith("urn:pulumi:")
>>>     assert "test-resource" in response.urn

"""

from contextlib import asynccontextmanager

import grpc

from pulumi.provider.server import ProviderServicer
from pulumi.runtime.proto import callback_pb2_grpc, provider_pb2_grpc
from pulumi.runtime.proto import resource_pb2_grpc
from pulumi.runtime.proto.resource_pb2_grpc import ResourceMonitorServicer
from pulumi.runtime.proto.callback_pb2_grpc import CallbacksServicer


@asynccontextmanager
async def monitor_servicer_stub(servicer: ResourceMonitorServicer):
    server = grpc.aio.server()
    resource_pb2_grpc.add_ResourceMonitorServicer_to_server(servicer, server)
    port = server.add_insecure_port("[::]:0")
    await server.start()
    channel = grpc.aio.insecure_channel(f"localhost:{port}")
    stub = resource_pb2_grpc.ResourceMonitorStub(channel)
    try:
        yield stub
    finally:
        await channel.close()
        await server.stop(None)


@asynccontextmanager
async def callback_servicer_stub(servicer: CallbacksServicer):
    server = grpc.aio.server()
    callback_pb2_grpc.add_CallbacksServicer_to_server(servicer, server)
    port = server.add_insecure_port("[::]:0")
    await server.start()
    channel = grpc.aio.insecure_channel(f"localhost:{port}")
    stub = callback_pb2_grpc.CallbacksStub(channel)
    try:
        yield stub
    finally:
        await channel.close()
        await server.stop(None)


@asynccontextmanager
async def provider_servicer_stub(servicer: ProviderServicer):
    server = grpc.aio.server()
    provider_pb2_grpc.add_ResourceProviderServicer_to_server(servicer, server)
    port = server.add_insecure_port("[::]:0")
    await server.start()
    channel = grpc.aio.insecure_channel(f"localhost:{port}")
    stub = provider_pb2_grpc.ResourceProviderStub(channel)
    try:
        yield stub
    finally:
        await channel.close()
        await server.stop(None)
