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
import inspect
from typing import List, Any

from google.protobuf import struct_pb2
from . import known_types
from ..output import Output, Inputs, Input
from ..resource import Resource

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

_special_sig_key = "4dabf18193072939515e22adb298388d"
"""specialSigKey is sometimes used to encode type identity inside of a map.  See pkg/resource/properties.go."""

_special_asset_sig = "c44067f5952c0a294b673a41bacd8c17"
"""specialAssetSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go."""

_special_archive_sig = "0def7320c3a5731c473e5ecbe6d01bc7"
"""specialArchiveSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go."""


async def serialize_properties(inputs: Inputs, deps: List[Resource]) -> struct_pb2.Struct:
    """
    Serializes an arbitrary Input bag into a Protobuf structure, keeping track of the list
    of dependent resources in the `deps` list. Serializing properties is inherently async
    because it awaits any futures that are contained transitively within the input bag.
    """
    struct = struct_pb2.Struct()
    for k, v in inputs.items():
        struct[k] = await serialize_property(v, deps)

    return struct


async def serialize_property(value: Input[Any], deps: List[Resource]) -> Any:
    """
    Serializes a single Input into a form suitable for remoting to the engine, awaiting
    any futures required to do so.
    """
    if isinstance(value, list):
        props = []
        for elem in value:
            props.append(await serialize_property(elem, deps))

        return props

    if known_types.is_custom_resource(value):
        deps.append(value)
        return await serialize_property(value.id, deps)

    if known_types.is_asset(value):
        # Serializing an asset requires the use of a magical signature key, since otherwise it would look
        # like any old weakly typed object/map when received by the other side of the RPC boundary.
        obj = {
            _special_sig_key: _special_asset_sig
        }

        if hasattr(value, "path"):
            obj["path"] = await serialize_property(value.path, deps)
        elif hasattr(value, "text"):
            obj["text"] = await serialize_property(value.text, deps)
        elif hasattr(value, "uri"):
            obj["uri"] = await serialize_property(value.uri, deps)
        else:
            raise AssertionError(f"unknown asset type: {value}")

        return obj

    if known_types.is_archive(value):
        # Serializing an archive requires the use of a magical signature key, since otherwise it would look
        # like any old weakly typed object/map when received by the other side of the RPC boundary.
        obj = {
            _special_sig_key: _special_archive_sig
        }

        if hasattr(value, "assets"):
            obj["assets"] = await serialize_property(value.assets, deps)
        elif hasattr(value, "path"):
            obj["path"] = await serialize_property(value.path, deps)
        elif hasattr(value, "uri"):
            obj["uri"] = await serialize_property(value.uri, deps)
        else:
            raise AssertionError(f"unknown archive type: {value}")

        return obj

    if inspect.isawaitable(value):
        return await serialize_property(await value, deps)

    if isinstance(value, Output):
        deps.extend(value.resources())

        # When serializing an Output, we will either serialize it as its resolved value or the "unknown value"
        # sentinel. We will do the former for all outputs created directly by user code (such outputs always
        # resolve isKnown to true) and for any resource outputs that were resolved with known values.
        is_known = await value._is_known
        value = await serialize_property(value.future(), deps)
        return value if is_known else UNKNOWN

    if isinstance(value, dict):
        obj = {}
        for k, v in value.items():
            obj[k] = await serialize_property(v, deps)

        return obj

    return value


def deserialize_properties(props_struct: struct_pb2.Struct) -> Any:
    """
    Deserializes a protobuf `struct_pb2.Struct` into a Python dictionary containing normal
    Python types.
    """
    # Check out this link for details on what sort of types Protobuf is going to generate:
    # https://developers.google.com/protocol-buffers/docs/reference/python-generated
    #
    # We assume that we are deserializing properties that we got from a Resource RPC endpoint,
    # which has type `Struct` in our gRPC proto definition.
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


def deserialize_property(value: Any) -> Any:
    """
    Deserializes a single protobuf value (either `Struct` or `ListValue`) into idiomatic
    Python values.
    """
    if value == UNKNOWN:
        return None

    # ListValues are projected to lists
    if isinstance(value, struct_pb2.ListValue):
        return [deserialize_property(v) for v in value]

    # Structs are projected to dictionaries
    if isinstance(value, struct_pb2.Struct):
        return deserialize_properties(value)

    # Everything else is identity projected.
    return value

