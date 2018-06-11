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
from pulumi import CustomResource
from pulumi.runtime import rpc, Unknown

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

    def test_unknown(self):
        """
        Tests that we serialize instances of the Unknown class to
        the unknown GUID.
        """
        struct = rpc.serialize_resource_props({
            "unknown_prop": Unknown()
        })

        # pylint: disable=unsubscriptable-object
        unknown = struct["unknown_prop"]
        self.assertEqual(rpc.UNKNOWN, unknown)

class FakeCustomResource(object):
    """
    Fake CustomResource class that duck-types to the real CustomResource.
    This class is substituted for the real CustomResource for the below test.
    """
    def __init__(self, id):
        self.id = id


class CustomResourceSerializeTest(unittest.TestCase):
    """
    Tests that we serialize CustomResources by serializing their ID.
    """
    def setUp(self):
        """
        Sets up the test by replacing the CustomResource that the rpc serialization
        system knows about with the above FakeCustomResource, which doesn't interact
        with the resource monitor.
        """
        rpc.register_custom_resource_type(FakeCustomResource)

    def tearDown(self):
        """
        Tears down the test by re-setting the rpc serialization system's known CustomResource.
        """
        rpc.register_custom_resource_type(CustomResource)

    def test_custom_resource(self):
        """
        Tests that the class registered by `register_custom_resource_type`
        is serialized by serializing its ID field.
        """
        struct = rpc.serialize_resource_props({
            "fake": FakeCustomResource(42)
        })

        self.assertTrue(isinstance(struct, struct_pb2.Struct))

        # pylint: disable=unsubscriptable-object
        serialized_resource = struct["fake"]
        self.assertEqual(42, serialized_resource)
