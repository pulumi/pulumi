# Copyright 2026, Pulumi Corporation.
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
from typing import Optional

import pytest

from pulumi.provider.experimental import provider
from pulumi.provider.experimental.property_value import PropertyValue
from pulumi.provider.experimental.server import ProviderServicer
from pulumi.runtime import config, proto, settings
from test.grpc_stubs import provider_servicer_stub


class ConcurrentConstructCallProvider(provider.Provider):
    def __init__(self):
        super().__init__()
        self.construct_waiting = asyncio.Event()
        self.call_ran = asyncio.Event()
        self.construct_stack_after_call: Optional[str] = None
        self.construct_config_after_call: Optional[str] = None

    async def construct(
        self, request: provider.ConstructRequest
    ) -> provider.ConstructResponse:
        assert settings.get_stack() == "construct-stack"
        assert config.get_config("test:key") == "construct-config"

        self.construct_waiting.set()
        await self.call_ran.wait()
        self.construct_stack_after_call = settings.get_stack()
        self.construct_config_after_call = config.get_config("test:key")

        return provider.ConstructResponse(
            urn=f"urn:pulumi:{request.name}::{request.resource_type}::test-resource",
            state={"result": PropertyValue("construct")},
            state_dependencies={},
        )

    async def call(self, request: provider.CallRequest) -> provider.CallResponse:
        assert settings.get_stack() == "call-stack"
        assert config.get_config("test:key") == "call-config"
        self.call_ran.set()
        return provider.CallResponse({"result": PropertyValue("call")})


@pytest.mark.asyncio
async def test_construct_and_call_can_run_concurrently():
    test_provider = ConcurrentConstructCallProvider()
    servicer = ProviderServicer([], "1.0.0", test_provider, "")

    async with provider_servicer_stub(servicer) as stub:
        construct_request = proto.ConstructRequest(
            name="construct",
            type="test:index:Component",
            stack="construct-stack",
            config={"test:key": "construct-config"},
        )
        construct_task = asyncio.ensure_future(stub.Construct(construct_request))

        await asyncio.wait_for(test_provider.construct_waiting.wait(), timeout=2)

        call_request = proto.CallRequest(
            tok="test:index:call",
            stack="call-stack",
            config={"test:key": "call-config"},
        )
        call_response = await asyncio.wait_for(stub.Call(call_request), timeout=2)
        construct_response = await asyncio.wait_for(construct_task, timeout=2)

        assert getattr(call_response, "return")["result"] == "call"
        assert construct_response.urn.endswith("::test-resource")
        assert test_provider.construct_stack_after_call == "construct-stack"
        assert test_provider.construct_config_after_call == "construct-config"
