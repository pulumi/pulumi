# Copyright 2016-2018, Pulumi Corporation.
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

import unittest
from google.protobuf import struct_pb2
from pulumi.runtime import rpc, Unknown

class PropertyDeserializeTests(unittest.TestCase):
    """
    Series of tests that ensures that we correctly deserialize Protobuf
    messages into Python data structures.
    """
    def test_empty_struct(self):
        """
        Tests that the empty Struct deserializes to {}.
        """
        empty = struct_pb2.Struct()
        deserialized = rpc.deserialize_resource_props(empty)
        self.assertDictEqual({}, deserialized)

    def test_struct_with_list_field(self):
        """
        Tests that we serialize Structs containing Lists to dictionaries
        containing Python lists.
        """
        proto = struct_pb2.Struct()

        # pylint: disable=no-member
        proto_list = proto.get_or_create_list("foo")
        proto_list.append("42")
        proto_list.append("bar")
        proto_list.append("baz")
        deserialized = rpc.deserialize_resource_props(proto)
        self.assertDictEqual({
            "foo": ["42", "bar", "baz"]
        }, deserialized)

    def test_struct_with_nested_struct(self):
        """
        Tests that we deserialize nested Structs correctly.
        """
        proto = struct_pb2.Struct()

        # pylint: disable=no-member
        subproto = proto.get_or_create_struct("bar")
        subproto["baz"] = 42
        deserialized = rpc.deserialize_resource_props(proto)
        self.assertDictEqual({
            "bar": {
                "baz": 42
            }
        }, deserialized)

    def test_unknown_sentinel(self):
        """
        Tests that we deserialize the UNKNOWN sentinel as None.
        """
        proto = struct_pb2.Struct()

        # pylint: disable=unsupported-assignment-operation
        proto["vpc_id"] = rpc.UNKNOWN
        deserialized = rpc.deserialize_resource_props(proto)
        self.assertTrue(isinstance(deserialized["vpc_id"], Unknown))

if __name__ == '__main__':
    unittest.main()
