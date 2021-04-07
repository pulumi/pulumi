import pytest

from pulumi.runtime.proto.provider_pb2 import ConstructRequest
from pulumi.provider.server import ProviderServicer, _as_struct
from pulumi.runtime import proto, rpc


@pytest.mark.asyncio
async def test_construct_inputs_parses_request():
    value = 'foobar'
    inputs = _as_struct({'echo': value})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    fut_v = await inputs['echo'].future()
    assert(fut_v == value)


@pytest.mark.asyncio
async def test_construct_inputs_preserves_unknowns():
    unknown = '04da6b54-80e4-46f7-96ec-b56ff0331ba9'
    inputs = _as_struct({'echo': unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    fut_v = await inputs['echo'].future()
    assert(fut_v is None)
