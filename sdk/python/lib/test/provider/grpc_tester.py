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

from concurrent import futures

import grpc

from pulumi.provider.server import ProviderServicer
from pulumi.runtime.proto import provider_pb2_grpc


class GrpcTestHelper:
    def __init__(self, servicer: ProviderServicer):
        self.servicer = servicer
        self.server = None
        self.channel = None
        self.stub = None
        self.port = None

    async def __aenter__(self):
        self.server = grpc.aio.server(futures.ThreadPoolExecutor(max_workers=10))
        provider_pb2_grpc.add_ResourceProviderServicer_to_server(
            self.servicer, self.server
        )
        self.port = self.server.add_insecure_port("[::]:0")
        await self.server.start()
        self.channel = grpc.aio.insecure_channel(f"localhost:{self.port}")
        self.stub = provider_pb2_grpc.ResourceProviderStub(self.channel)
        return self.stub

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.channel:
            await self.channel.close()
        if self.server:
            await self.server.stop(None)
