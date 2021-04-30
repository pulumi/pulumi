# Copyright 2016-2021, Pulumi Corporation.
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

from typing import Dict, Any

import os
import pytest

from pulumi.provider.server import ProviderServicer
from pulumi.runtime import proto, rpc
from pulumi.runtime.proto.provider_pb2 import ConstructRequest
import google.protobuf.struct_pb2 as struct_pb2
import pulumi.output


def test_construct_inputs_parses_request():
    value = 'foobar'
    inputs = _as_struct({'echo': value})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    assert inputs['echo'] == value


def test_construct_inputs_preserves_unknowns():
    unknown = '04da6b54-80e4-46f7-96ec-b56ff0331ba9'
    inputs = _as_struct({'echo': unknown})
    req = ConstructRequest(inputs=inputs)
    inputs = ProviderServicer._construct_inputs(req)
    assert len(inputs) == 1
    assert isinstance(inputs['echo'], pulumi.output.Unknown)


def _as_struct(key_values: Dict[str, Any]) -> struct_pb2.Struct:
    the_struct = struct_pb2.Struct()
    the_struct.update(key_values)  # pylint: disable=no-member
    return the_struct
