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

import struct
from google.protobuf import json_format, struct_pb2
from pulumi.runtime.otel_logger import (
    PropertyValue,
    encode_property_value,
    emit,
    SEVERITY_INFO,
)
from pulumi.runtime.rpc import (
    _special_sig_key,
    _special_secret_sig,
)


class TestEncodePropertyValue:
    def test_magic_prefix(self):
        encoded = encode_property_value("hello")
        assert encoded[:8] == b"pulumiPv"

    def test_magic_prefix_le_uint64(self):
        encoded = encode_property_value("hello")
        magic_int = struct.unpack_from("<Q", encoded, 0)[0]
        # Must match Go's PropertyValueLogMagic = 0x7650696d756c7570
        assert magic_int == 0x7650696D756C7570

    def test_round_trip_object(self):
        input_val = {"name": "my-bucket", "count": 42}
        encoded = encode_property_value(input_val)

        # Skip magic, deserialize as protobuf Value.
        proto_value = struct_pb2.Value()
        proto_value.ParseFromString(encoded[8:])
        decoded = json_format.MessageToDict(proto_value)

        assert decoded["name"] == "my-bucket"
        assert decoded["count"] == 42

    def test_preserves_secret_signatures(self):
        input_val = {
            "name": "my-bucket",
            "password": {
                _special_sig_key: _special_secret_sig,
                "value": "hunter2",
            },
        }
        encoded = encode_property_value(input_val)

        proto_value = struct_pb2.Value()
        proto_value.ParseFromString(encoded[8:])
        decoded = json_format.MessageToDict(proto_value)

        assert decoded["name"] == "my-bucket"
        assert decoded["password"][_special_sig_key] == _special_secret_sig
        assert decoded["password"]["value"] == "hunter2"

    def test_handles_arrays(self):
        input_val = [1, "two", True]
        encoded = encode_property_value(input_val)

        proto_value = struct_pb2.Value()
        proto_value.ParseFromString(encoded[8:])
        decoded = json_format.MessageToDict(proto_value)

        assert decoded == [1.0, "two", True]

    def test_handles_primitives(self):
        for val in ["hello", 42, True, None]:
            encoded = encode_property_value(val)
            proto_value = struct_pb2.Value()
            proto_value.ParseFromString(encoded[8:])
            decoded = json_format.MessageToDict(proto_value)
            if isinstance(val, int) and not isinstance(val, bool):
                assert decoded == float(val)
            else:
                assert decoded == val


class TestPropertyValueWrapper:
    def test_wraps_value(self):
        pv = PropertyValue({"key": "val"})
        assert pv.value == {"key": "val"}


class TestEmit:
    def test_noop_without_init(self):
        # Should not raise when logger is not initialized.
        emit(SEVERITY_INFO, "test message", {
            "key": "value",
            "pv": PropertyValue({"secret": "data"}),
        })
