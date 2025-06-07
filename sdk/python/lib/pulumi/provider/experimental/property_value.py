# Copyright 2025, Pulumi Corporation.
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

import copy
from enum import Enum
import types
from typing import (
    Any,
    Dict,
    Optional,
    Set,
    Union,
    cast,
    Iterable,
    Sequence,
    Mapping,
)
from dataclasses import dataclass

from google.protobuf import struct_pb2

import pulumi


class PropertyValueType(Enum):
    """
    Represents the types of property values.
    """

    NULL = "Null"
    BOOL = "Bool"
    NUMBER = "Number"
    STRING = "String"
    ARRAY = "Array"
    MAP = "Map"
    ASSET = "Asset"
    ARCHIVE = "Archive"
    RESOURCE = "Resource"
    COMPUTED = "Computed"


@dataclass(frozen=True)
class ResourceReference:
    """
    Represents a reference to a resource.
    """

    urn: str
    """The URN of the resource."""
    resource_id: Optional["PropertyValue"] = None
    """The ID of the resource, if applicable."""
    package_version: Optional[str] = None
    """The package version of the resource."""


@dataclass(frozen=True)
class Computed:
    def __eq__(self, value):
        if isinstance(value, PropertyValue.Computed):
            return True
        return False

    def __hash__(self) -> int:
        return hash(0)


PythonValue = Optional[
    Union[
        bool,
        float,
        str,
        pulumi.Asset,
        pulumi.Archive,
        Sequence["PropertyValue"],
        Mapping[str, "PropertyValue"],
        ResourceReference,
        Computed,
    ]
]
"""
PythonValue is a type alias for the set of Python values that can be contained inside a PropertyValue.
"""


@dataclass(frozen=True)
class PropertyValue:
    """
    Represents a property value.
    """

    is_secret: bool = False
    """Whether the property value is a secret."""

    dependencies: frozenset[str] = frozenset()
    """The dependencies of the property value."""

    value: PythonValue = None
    """The value of the property."""

    def __init__(
        self,
        value: PythonValue,
        is_secret: bool = False,
        dependencies: Optional[Iterable[str]] = None,
    ) -> None:
        """
        :param value: The value of the property.
        :param is_secret: Whether the value is secret.
        :param dependencies: The dependencies of the property value.
        """
        # Wrap Sequence and Mapping types in immutable types to ensure they are hashable.
        if isinstance(value, Sequence) and not isinstance(value, str):
            value = tuple(value)
        elif isinstance(value, Mapping):
            value = types.MappingProxyType(value)
        elif isinstance(value, pulumi.Asset) or isinstance(value, pulumi.Archive):
            # Ensure the Asset/Archive is immutable by deep copying
            value = copy.deepcopy(value)
        object.__setattr__(self, "value", value)
        object.__setattr__(self, "is_secret", is_secret)
        object.__setattr__(
            self,
            "dependencies",
            frozenset(dependencies) if dependencies else frozenset(),
        )

    @staticmethod
    def computed() -> "PropertyValue":
        """
        Creates a computed property value.
        """
        return PropertyValue(Computed())

    @staticmethod
    def null() -> "PropertyValue":
        """
        Creates a null property value.
        """
        return PropertyValue(None)

    def with_secret(self, is_secret: bool) -> "PropertyValue":
        """
        Returns a copy of the PropertyValue with the specified secret status.
        """
        return PropertyValue(
            self.value,
            is_secret=is_secret,
            dependencies=self.dependencies,
        )

    def with_dependencies(self, dependencies: Iterable[str]) -> "PropertyValue":
        """
        Returns a copy of the PropertyValue with the specified dependencies.
        """
        return PropertyValue(
            self.value,
            is_secret=self.is_secret,
            dependencies=dependencies,
        )

    def with_value(self, value: PythonValue) -> "PropertyValue":
        """
        Returns a copy of the PropertyValue with the specified value.
        """
        return PropertyValue(
            value,
            is_secret=self.is_secret,
            dependencies=self.dependencies,
        )

    @property
    def type(self) -> PropertyValueType:
        """
        Determines the type of the property value.
        """
        if self.value is None:
            return PropertyValueType.NULL
        if isinstance(self.value, bool):
            return PropertyValueType.BOOL
        if isinstance(self.value, (int, float)):
            return PropertyValueType.NUMBER
        if isinstance(self.value, str):
            return PropertyValueType.STRING
        if isinstance(self.value, pulumi.Asset):
            return PropertyValueType.ASSET
        if isinstance(self.value, pulumi.Archive):
            return PropertyValueType.ARCHIVE
        if isinstance(self.value, Sequence):
            return PropertyValueType.ARRAY
        if isinstance(self.value, Mapping):
            return PropertyValueType.MAP
        if isinstance(self.value, ResourceReference):
            return PropertyValueType.RESOURCE
        if isinstance(self.value, Computed):
            return PropertyValueType.COMPUTED
        raise ValueError(f"Unsupported value type: {type(self.value)}")

    def __eq__(self, other: Any) -> bool:
        if not isinstance(other, PropertyValue):
            return False
        return (
            self.value == other.value
            and self.is_secret == other.is_secret
            and self.dependencies == other.dependencies
        )

    def __hash__(self) -> int:
        return hash([self.value, self.is_secret, self.dependencies])

    def __str__(self) -> str:
        v = str(self.value)
        # If we have secretness or dependencies, we want to include that in the string representation.
        if self.is_secret:
            v = f"{v} (secret)"
        if self.dependencies:
            deps = ", ".join(self.dependencies)
            v = f"{v} (dependencies: {deps})"
        return v

    def all_dependencies(self) -> Set[str]:
        """
        Returns all dependencies of the property value, including dependencies of nested values.
        """
        deps = set(self.dependencies)
        if isinstance(self.value, Sequence) and not isinstance(self.value, str):
            for item in self.value:
                if isinstance(item, PropertyValue):
                    deps.update(item.all_dependencies())
        elif isinstance(self.value, Mapping):
            for item in self.value.values():
                if isinstance(item, PropertyValue):
                    deps.update(item.all_dependencies())
        return deps

    def contains_secret(self) -> bool:
        """
        Determines if the property value contains a secret.
        """
        if self.is_secret:
            return True
        if isinstance(self.value, Sequence) and not isinstance(self.value, str):
            return any(v.contains_secret() for v in self.value)
        if isinstance(self.value, Mapping):
            return any(v.contains_secret() for v in self.value.values())
        return False

    def marshal(self) -> struct_pb2.Value:
        """
        Marshals a PropertyValue into a protobuf struct value.

        :return: A protobuf struct value representation of the PropertyValue.
        """

        def marshal_value(value: PythonValue) -> struct_pb2.Value:
            if value is None:
                return struct_pb2.Value(null_value=struct_pb2.NULL_VALUE)

            def marshal_asset(asset: pulumi.Asset) -> struct_pb2.Value:
                pbstruct = {}
                pbstruct[pulumi.runtime.rpc._special_sig_key] = struct_pb2.Value(
                    string_value=pulumi.runtime.rpc._special_asset_sig
                )

                if hasattr(asset, "path"):
                    file_asset = cast("pulumi.FileAsset", asset)
                    pbstruct["path"] = struct_pb2.Value(string_value=file_asset.path)
                elif hasattr(asset, "text"):
                    str_asset = cast("pulumi.StringAsset", asset)
                    pbstruct["text"] = struct_pb2.Value(string_value=str_asset.text)
                elif hasattr(asset, "uri"):
                    remote_asset = cast("pulumi.RemoteAsset", asset)
                    pbstruct["uri"] = struct_pb2.Value(string_value=remote_asset.uri)

                return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

            def marshal_archive(archive: pulumi.Archive) -> struct_pb2.Value:
                pbstruct = {}
                pbstruct[pulumi.runtime.rpc._special_sig_key] = struct_pb2.Value(
                    string_value=pulumi.runtime.rpc._special_archive_sig
                )

                if hasattr(archive, "path"):
                    file_archive = cast("pulumi.FileArchive", archive)
                    pbstruct["path"] = struct_pb2.Value(string_value=file_archive.path)
                elif hasattr(archive, "uri"):
                    remote_archive = cast("pulumi.RemoteArchive", archive)
                    pbstruct["uri"] = struct_pb2.Value(string_value=remote_archive.uri)
                elif hasattr(archive, "assets"):
                    asset_archive = cast("pulumi.AssetArchive", archive)
                    assets = {}
                    for k, v in asset_archive.assets.items():
                        if isinstance(v, pulumi.Asset):
                            assets[k] = marshal_asset(v)
                        elif isinstance(v, pulumi.Archive):
                            assets[k] = marshal_archive(v)
                        else:
                            raise ValueError(
                                "Invalid asset archive encountered when marshaling resource property"
                            )
                    pbstruct["assets"] = struct_pb2.Value(
                        struct_value=struct_pb2.Struct(fields=assets)
                    )

                return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

            if isinstance(value, bool):
                return struct_pb2.Value(bool_value=value)

            if isinstance(value, float):
                return struct_pb2.Value(number_value=value)

            if isinstance(value, str):
                return struct_pb2.Value(string_value=value)

            if isinstance(value, Sequence) and not isinstance(value, str):
                pblist = []
                for item in value:
                    pblist.append(item.marshal())
                return struct_pb2.Value(list_value=struct_pb2.ListValue(values=pblist))

            if isinstance(value, Mapping):
                pbstruct = {}
                for key, item in value.items():
                    pbstruct[key] = item.marshal()
                return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

            if isinstance(value, pulumi.Asset):
                return marshal_asset(value)

            if isinstance(value, pulumi.Archive):
                return marshal_archive(value)

            if isinstance(value, ResourceReference):
                pbstruct = {}
                pbstruct[pulumi.runtime.rpc._special_sig_key] = struct_pb2.Value(
                    string_value=pulumi.runtime.rpc._special_resource_sig
                )
                pbstruct["urn"] = struct_pb2.Value(string_value=value.urn)
                if value.resource_id is not None:
                    pbstruct["resource_id"] = value.resource_id.marshal()
                if value.package_version is not None:
                    pbstruct["package_version"] = struct_pb2.Value(
                        string_value=value.package_version
                    )
                return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

            raise ValueError(f"Unsupported value type: {type(value)}")

        val = marshal_value(self.value)
        if self.dependencies:
            pbstruct = {}
            pbstruct[pulumi.runtime.rpc._special_sig_key] = struct_pb2.Value(
                string_value=pulumi.runtime.rpc._special_output_value_sig
            )
            pbstruct["value"] = val
            pbstruct["dependencies"] = struct_pb2.Value(
                list_value=struct_pb2.ListValue(
                    values=[
                        struct_pb2.Value(string_value=dep) for dep in self.dependencies
                    ]
                )
            )
            pbstruct["secret"] = struct_pb2.Value(bool_value=self.is_secret)
            return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

        if self.is_secret:
            pbstruct = {}
            pbstruct[pulumi.runtime.rpc._special_sig_key] = struct_pb2.Value(
                string_value=pulumi.runtime.rpc._special_secret_sig
            )
            pbstruct["value"] = val
            return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

        return val

    @staticmethod
    def unmarshal(value: struct_pb2.Value) -> "PropertyValue":
        kind = value.WhichOneof("kind")

        if kind == "null_value":
            return PropertyValue.null()

        if kind == "bool_value":
            return PropertyValue(value.bool_value)

        if kind == "number_value":
            return PropertyValue(value.number_value)

        if kind == "string_value":
            return PropertyValue(value.string_value)

        if kind == "list_value":
            list_result = []
            for item in value.list_value.values:
                list_result.append(PropertyValue.unmarshal(item))
            return PropertyValue(list_result)

        if kind == "struct_value":
            fields = value.struct_value.fields
            sig = fields.get(pulumi.runtime.rpc._special_sig_key)

            if sig is None:
                struct_result = {}
                for key, item in fields.items():
                    struct_result[key] = PropertyValue.unmarshal(item)
                return PropertyValue(struct_result)

            if sig.string_value == pulumi.runtime.rpc._special_secret_sig:
                inner = fields.get("value")
                if inner is None:
                    raise ValueError("Secret value missing 'value' field.")
                return PropertyValue.unmarshal(inner).with_secret(True)

            if sig.string_value == pulumi.runtime.rpc._special_asset_sig:
                if "path" in fields and fields["path"].HasField("string_value"):
                    return PropertyValue(pulumi.FileAsset(fields["path"].string_value))
                if "text" in fields and fields["text"].HasField("string_value"):
                    return PropertyValue(
                        pulumi.StringAsset(fields["text"].string_value)
                    )
                if "uri" in fields and fields["uri"].HasField("string_value"):
                    return PropertyValue(pulumi.RemoteAsset(fields["uri"].string_value))
                raise AssertionError(
                    "Invalid asset encountered when unmarshalling resource property"
                )

            if sig.string_value == pulumi.runtime.rpc._special_archive_sig:
                if "assets" in fields and fields["assets"].HasField("struct_value"):
                    assets = {}
                    # Check all the PropertyValues are assets/archives
                    for k, v in PropertyValue.unmarshal_map(
                        fields["assets"].struct_value
                    ).items():
                        if not isinstance(v.value, (pulumi.Asset, pulumi.Archive)):
                            raise ValueError(
                                "Invalid archive encountered when unmarshalling resource property"
                            )
                        assets[k] = v.value
                    return PropertyValue(pulumi.AssetArchive(assets))
                if "path" in fields and fields["path"].HasField("string_value"):
                    return PropertyValue(
                        pulumi.FileArchive(fields["path"].string_value)
                    )
                if "uri" in fields and fields["uri"].HasField("string_value"):
                    return PropertyValue(
                        pulumi.RemoteArchive(fields["uri"].string_value)
                    )
                raise AssertionError(
                    "Invalid archive encountered when unmarshalling resource property"
                )

            if sig.string_value == pulumi.runtime.rpc._special_resource_sig:
                if "urn" not in fields or not fields["urn"].HasField("string_value"):
                    raise ValueError("Resource reference missing 'urn' field.")
                urn = fields["urn"].string_value
                resource_id = fields.get("resource_id")
                package_version = fields.get("package_version")
                resource_id_value: Optional[PropertyValue] = None
                if resource_id is not None:
                    resource_id_value = PropertyValue.unmarshal(resource_id)
                package_version_str: Optional[str] = None
                if package_version is not None:
                    if not package_version.HasField("string_value"):
                        raise ValueError(
                            "Resource reference 'package_version' field is not a string."
                        )
                    package_version_str = package_version.string_value
                return PropertyValue(
                    ResourceReference(
                        urn=urn,
                        resource_id=resource_id_value,
                        package_version=package_version_str,
                    )
                )

            if sig.string_value == pulumi.runtime.rpc._special_output_value_sig:
                inner = fields.get("value")
                if inner is None:
                    raise ValueError("Output value missing 'value' field.")
                dependencies = fields.get("dependencies")
                if dependencies is not None:
                    if not dependencies.HasField("list_value"):
                        raise ValueError(
                            "Output value 'dependencies' field is not a list."
                        )
                    deps = set()
                    for dep in dependencies.list_value.values:
                        if not dep.HasField("string_value"):
                            raise ValueError(
                                "Output value 'dependencies' field contains non-string values."
                            )
                        deps.add(dep.string_value)
                secret = fields.get("secret")
                if secret is not None:
                    if not secret.HasField("bool_value"):
                        raise ValueError(
                            "Output value 'secret' field is not a boolean."
                        )
                    is_secret = secret.bool_value

                return (
                    PropertyValue.unmarshal(inner)
                    .with_dependencies(deps)
                    .with_secret(is_secret)
                )

            raise ValueError(f"Unknown signature key for struct value: {sig}")

        raise ValueError(f"Unsupported value type: {kind}")

    @staticmethod
    def unmarshal_map(
        value: struct_pb2.Struct,
    ) -> Dict[str, "PropertyValue"]:
        """
        Unmarshals a protobuf struct value into a dictionary of PropertyValues.

        :param value: The protobuf struct value to unmarshal.
        :return: A dictionary of PropertyValues.
        """
        if not isinstance(value, struct_pb2.Struct):
            raise ValueError("Expected a protobuf struct.")

        result: Dict[str, PropertyValue] = {}
        for key, item in value.fields.items():
            result[key] = PropertyValue.unmarshal(item)
        return result

    @staticmethod
    def marshal_map(
        value: Dict[str, "PropertyValue"],
    ) -> struct_pb2.Struct:
        """
        Marshals a dictionary of PropertyValues into a protobuf struct value.

        :param value: The dictionary of PropertyValues to marshal.
        :return: A protobuf struct value representation of the PropertyValues.
        """
        pbstruct = {}
        for key, item in value.items():
            pbstruct[key] = item.marshal()
        return struct_pb2.Struct(fields=pbstruct)
