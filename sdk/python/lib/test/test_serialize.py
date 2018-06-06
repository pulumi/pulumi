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
from pulumi.runtime import rpc

class PropertySerializeTests(unittest.TestCase):
    """
    Series of tests that ensures that we serialize Python datatypes
    into Protobuf datatypes correctly.
    """
    def test_empty(self):
        """
        Tests that we serialize the empty Struct correctly.
        """

        struct = rpc.serialize_resource_props({})
        self.assertTrue(isinstance(struct, struct_pb2.Struct))
        self.assertEqual(0, len(struct))

    def test_dict_of_lists(self):
        """
        Tests that we serialize a struct containing a list correctly.
        """

        struct = rpc.serialize_resource_props({
            "foo": [1, "2", True]
        })
        self.assertTrue(isinstance(struct, struct_pb2.Struct))

        # pylint: disable=unsubscriptable-object
        proto_list = struct["foo"]
        self.assertTrue(isinstance(proto_list, struct_pb2.ListValue))
        self.assertEqual(1, proto_list[0])
        self.assertEqual("2", proto_list[1])
        self.assertEqual(True, proto_list[2])
