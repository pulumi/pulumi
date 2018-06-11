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
"""
Support for serializing and deserializing properties going into or flowing
out of RPC calls.
"""

import six
from six.moves import map
from google.protobuf import struct_pb2
from .unknown import Unknown

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

_custom_resource_type = None
"""The type of CustomResource. Filled-in as the Pulumi package is initializing."""

def serialize_resource_props(props):
    """
    Serializes resource properties so that they are ready for marshaling to the gRPC endpoint.
    """
    struct = struct_pb2.Struct()
    for k, v in props.items():
        struct[k] = serialize_resource_value(v) # pylint: disable=unsupported-assignment-operation
    return struct

def serialize_resource_value(value):
    """
    Serializes a resource property value so that it's ready for marshaling to the gRPC endpoint.
    """

    assert _custom_resource_type is not None, "failed to set CustomResource type"
    if isinstance(value, _custom_resource_type):
        # Resource objects aren't serializable.  Instead, serialize them as references to their IDs.
        return serialize_resource_value(value.id)
    elif isinstance(value, dict):
        # Deeply serialize dictionaries.
        return {k: serialize_resource_value(v) for (k, v) in value.items()}
    elif isinstance(value, list):
        # Deeply serialize lists.
        return list(map(serialize_resource_value, value))
    elif isinstance(value, Unknown):
        # Serialize instances of Unknown as the UNKNOWN guid
        return UNKNOWN
    else:
        # All other values are directly serializable.
        # TODO[pulumi/pulumi#1063]: eventually, we want to think about Output, Properties, and so on.
        return value

def deserialize_resource_props(props_struct):
    """
    Deserializes a protobuf `struct_pb2.Struct` into a Python dictionary containing normal
    Python types.
    """
    # Check out this link for details on what sort of types Protobuf is going to generate:
    # https://developers.google.com/protocol-buffers/docs/reference/python-generated
    #
    # We assume that we are deserializing properties that we got from a Resource RPC endpoint,
    # which has type `Struct` in our gRPC proto definition.
    assert isinstance(props_struct, struct_pb2.Struct)

    # Struct is duck-typed like a dictionary, so we can iterate over it in the normal ways.
    return {k: deserialize_property(v) for (k, v) in props_struct.items()}

def deserialize_property(prop):
    """
    Deserializes a single protobuf value (either `Struct` or `ListValue`) into idiomatic
    Python values.
    """

    if prop == UNKNOWN:
        return Unknown()

    # ListValues are projected to lists
    if isinstance(prop, struct_pb2.ListValue):
        return [deserialize_property(p) for p in prop]

    # Structs are projected to dictionaries
    if isinstance(prop, struct_pb2.Struct):
        return deserialize_resource_props(prop)

    # Everything else is identity projected.
    return prop

def register_custom_resource_type(class_obj):
    """
    Registers the given class object as the CustomResource type,
    for use in serialization.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _custom_resource_type
    _custom_resource_type = class_obj
