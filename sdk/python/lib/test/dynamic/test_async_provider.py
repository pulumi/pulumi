import asyncio
import base64
from typing import Any

import dill
from google.protobuf import struct_pb2

from pulumi.dynamic import ConfigureRequest, CreateResult, ResourceProvider
from pulumi.dynamic.__main__ import PROVIDER_KEY, DynamicResourceProviderServicer
from pulumi.runtime import proto


class AsyncProvider(ResourceProvider):
    async def create(self, props: dict[str, Any]) -> CreateResult:
        await asyncio.sleep(0)
        return CreateResult(id_="async-id", outs={"echo": props["echo"]})


class CrossCallProvider(ResourceProvider):
    async def configure(self, req: ConfigureRequest) -> None:
        self.ready: asyncio.Future = asyncio.get_running_loop().create_future()

    async def create(self, props: dict[str, Any]) -> CreateResult:
        asyncio.get_running_loop().call_soon(self.ready.set_result, "configured")
        return CreateResult(id_="async-id", outs={"echo": await self.ready})


def create(provider: ResourceProvider) -> proto.CreateResponse:
    props = struct_pb2.Struct()
    props.update(
        {
            "echo": "hello",
            PROVIDER_KEY: base64.b64encode(dill.dumps(provider)).decode(),
        }
    )
    return DynamicResourceProviderServicer().Create(
        proto.CreateRequest(properties=props), None
    )


def test_create_awaits_async_provider():
    response = create(AsyncProvider())

    assert response.id == "async-id"
    assert response.properties["echo"] == "hello"


def test_futures_are_shared_between_calls():
    # A future is bound to the loop that created it, so `configure` and `create`
    # have to run on the same loop.
    response = create(CrossCallProvider())

    assert response.properties["echo"] == "configured"
