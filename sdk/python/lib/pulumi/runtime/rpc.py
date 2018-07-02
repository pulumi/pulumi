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
from __future__ import absolute_import

import six
from six.moves import map

from google.protobuf import struct_pb2
from .unknown import Unknown
from . import known_types

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

_special_sig_key = "4dabf18193072939515e22adb298388d"
"""specialSigKey is sometimes used to encode type identity inside of a map.  See pkg/resource/properties.go."""

_special_asset_sig = "c44067f5952c0a294b673a41bacd8c17"
"""specialAssetSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go."""

_special_archive_sig = "0def7320c3a5731c473e5ecbe6d01bc7"
"""specialArchiveSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go."""

def serialize_resource_props(props):
    """
    Serializes resource properties so that they are ready for marshaling to the gRPC endpoint.
    """
    struct = struct_pb2.Struct()
    for k, v in list(props.items()):
        struct[k] = serialize_resource_value(v) # pylint: disable=unsupported-assignment-operation
    return struct

def serialize_resource_value(value):
    """
    Serializes a resource property value so that it's ready for marshaling to the gRPC endpoint.
    """

    assert known_types._custom_resource_type is not None, "failed to set CustomResource type"
    if known_types.is_custom_resource(value):
        # Resource objects aren't serializable.  Instead, serialize them as references to their IDs.
        return serialize_resource_value(value.id)
    elif isinstance(value, dict):
        # Deeply serialize dictionaries.
        return {k: serialize_resource_value(v) for (k, v) in list(value.items())}
    elif isinstance(value, list):
        # Deeply serialize lists.
        return list(map(serialize_resource_value, value))
    elif isinstance(value, Unknown):
        # Serialize instances of Unknown as the UNKNOWN guid
        return UNKNOWN
    elif known_types.is_asset(value):
        # Serializing an asset requires the use of a magical signature key, since otherwise it would look
        # like any old weakly typed object/map when received by the other side of the RPC boundary.
        obj = {
            _special_sig_key: _special_asset_sig
        }

        if hasattr(value, "path"):
            obj["path"] = value.path
        elif hasattr(value, "text"):
            obj["text"] = value.text
        elif hasattr(value, "uri"):
            obj["uri"] = value.uri
        else:
            raise AssertionError("unknown asset type: " + str(value))

        return obj
    elif known_types.is_archive(value):
        # Serializing an archive requires the use of a magical signature key, since otherwise it would look
        # like any old weakly typed object/map when received by the other side of the RPC boundary.
        obj = {
            _special_sig_key: _special_archive_sig
        }

        if hasattr(value, "assets"):
            obj["assets"] = serialize_resource_value(value.assets)
        elif hasattr(value, "path"):
            obj["path"] = value.path
        elif hasattr(value, "uri"):
            obj["uri"] = value.uri
        else:
            raise AssertionError("unknown archive type: " + str(value))

        return obj
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

    if _special_sig_key in props_struct:
        if props_struct[_special_sig_key] == _special_asset_sig:
            # This is an asset. Re-hydrate this object into an Asset.
            if "path" in props_struct:
                return known_types.new_file_asset(props_struct["path"])
            if "text" in props_struct:
                return known_types.new_string_asset(props_struct["text"])
            if "uri" in props_struct:
                return known_types.new_remote_asset(props_struct["uri"])
            raise AssertionError("Invalid asset encountered when unmarshaling resource property")
        elif props_struct[_special_sig_key] == _special_archive_sig:
            # This is an archive. Re-hydrate this object into an Archive.
            if "assets" in props_struct:
                return known_types.new_asset_archive(deserialize_property(props_struct["assets"]))
            if "path" in props_struct:
                return known_types.new_file_archive(props_struct["path"])
            if "uri" in props_struct:
                return known_types.new_remote_archive(props_struct["uri"])

        raise AssertionError("Unrecognized signature when unmarshaling resource property")


    # Struct is duck-typed like a dictionary, so we can iterate over it in the normal ways.
    return {k: deserialize_property(v) for (k, v) in list(props_struct.items())}

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
