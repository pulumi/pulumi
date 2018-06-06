import unittest
import pulumi
import pulumi.runtime
from google.protobuf import struct_pb2

class PropertySerializeDeserializeTests(unittest.TestCase):
    """
    Series of tests that ensures that we correctly deserialize Protobuf
    messages into Python data structures.
    """
    def test_empty_struct(self):
        """
        Tests that the empty Struct deserializes to {}.
        """
        empty = struct_pb2.Struct()
        deserialized = pulumi.runtime.rpc.deserialize_resource_props(empty)
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
        deserialized = pulumi.runtime.rpc.deserialize_resource_props(proto)
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
        deserialized = pulumi.runtime.rpc.deserialize_resource_props(proto)
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
        proto["vpc_id"] = pulumi.runtime.rpc.UNKNOWN
        deserialized = pulumi.runtime.rpc.deserialize_resource_props(proto)
        self.assertDictEqual({
            "vpc_id": None
        }, deserialized)

if __name__ == '__main__':
    unittest.main()
