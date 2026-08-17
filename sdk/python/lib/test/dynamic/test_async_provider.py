import asyncio
import base64
from typing import Any

import dill
from google.protobuf import struct_pb2

from pulumi.dynamic import CreateResult, ResourceProvider
from pulumi.dynamic.__main__ import PROVIDER_KEY, DynamicResourceProviderServicer
from pulumi.runtime import proto


class AsyncProvider(ResourceProvider):
    async def create(self, props: dict[str, Any]) -> CreateResult:
        await asyncio.sleep(0)
        return CreateResult(id_="async-id", outs={"echo": props["echo"]})


def test_create_awaits_async_provider():
    props = struct_pb2.Struct()
    props.update(
        {
            "echo": "hello",
            PROVIDER_KEY: base64.b64encode(dill.dumps(AsyncProvider())).decode(),
        }
    )

    response = DynamicResourceProviderServicer().Create(
        proto.CreateRequest(properties=props), None
    )

    assert response.id == "async-id"
    assert response.properties["echo"] == "hello"
