from typing import Dict, Any

import pytest

from pulumi.runtime.proto.provider_pb2 import ConstructRequest
from pulumi.provider.server import ProviderServicer
from pulumi.runtime import proto, rpc
import google.protobuf.struct_pb2 as struct_pb2


@pytest.mark.asyncio
async def test_construct_inputs_parses_request():
    value = 'foobar'
    inputs = _as_struct({'echo': value})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    fut_v = await inputs['echo'].future()
    assert fut_v == value


@pytest.mark.asyncio
async def test_construct_inputs_preserves_unknowns():
    unknown = '04da6b54-80e4-46f7-96ec-b56ff0331ba9'
    inputs = _as_struct({'echo': unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    fut_v = await inputs['echo'].future()
    assert fut_v is None



def _as_struct(key_values: Dict[str,Any]) -> struct_pb2.Struct:
    the_struct = struct_pb2.Struct()
    the_struct.update(key_values)  # pylint: disable=no-member
    return the_struct
