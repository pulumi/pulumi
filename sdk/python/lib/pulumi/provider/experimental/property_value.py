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
import builtins
from collections import abc
import asyncio
import inspect
from enum import Enum
import types
from typing import (
    Any,
    Dict,
    Optional,
    Set,
    List,
    Union,
    cast,
    Callable,
    Sequence,
    Mapping,
    Iterable,
    get_args,
    get_origin,
)
from dataclasses import dataclass


from google.protobuf import struct_pb2

import pulumi.log as log
import pulumi
from pulumi.runtime import settings, known_types
from pulumi.runtime.rpc import _expand_dependencies, isLegalProtobufValue
import pulumi._types as _types
from pulumi.resource import DependencyResource


class PropertyValueType(Enum):
    """
    Represents the types of property values.
    """

    NULL = "Null"
    BOOL = "Bool"
    NUMBER = "Number"
    STRING = "String"
    ARRAY = "Array"
    OBJECT = "Object"
    ASSET = "Asset"
    ARCHIVE = "Archive"
    SECRET = "Secret"
    RESOURCE = "Resource"
    OUTPUT = "Output"
    COMPUTED = "Computed"


@dataclass(frozen=True)
class ResourceReference:
    """
    Represents a reference to a resource.
    """

    def __init__(
        self, urn: str, resource_id: Optional[Any], package_version: str
    ) -> None:
        """
        :param urn: The URN of the resource.
        :param resource_id: The ID of the resource.
        :param package_version: The package version of the resource.
        """
        self.urn = urn
        self.resource_id = resource_id
        self.package_version = package_version

    def __eq__(self, other: Any) -> bool:
        if not isinstance(other, ResourceReference):
            return False
        return (
            self.urn == other.urn
            and self.resource_id == other.resource_id
            and self.package_version == other.package_version
        )

    def __hash__(self) -> int:
        return hash((self.urn, self.resource_id, self.package_version))


@dataclass(frozen=True)
class OutputReference:
    """
    Represents a reference to an output value.
    """

    dependencies: frozenset[str]
    value: Optional["PropertyValue"]

    def __init__(
        self, value: Optional["PropertyValue"], dependencies: Iterable[str]
    ) -> None:
        """
        :param value: The value of the output, if known.
        :param dependencies: The dependencies of the output.
        """
        object.__setattr__(self, "value", value)
        object.__setattr__(self, "dependencies", frozenset(dependencies))

    def __eq__(self, other: Any) -> bool:
        if not isinstance(other, OutputReference):
            return False
        return self.value == other.value and self.dependencies == other.dependencies

    def __hash__(self) -> int:
        return hash((self.value, frozenset(self.dependencies)))

    def __str__(self) -> str:
        return f"Output({self.value}, {self.dependencies})"


@dataclass(frozen=True)
class PropertyValue:
    """
    Represents a property value.
    """

    @dataclass(frozen=True)
    class Computed:
        def __eq__(self, value):
            if isinstance(value, PropertyValue.Computed):
                return True
            return False

        def __hash__(self) -> int:
            return hash(0)

    value: Optional[
        Union[
            bool,
            float,
            str,
            pulumi.Asset,
            pulumi.Archive,
            "PropertyValue",
            Sequence["PropertyValue"],
            Mapping[str, "PropertyValue"],
            ResourceReference,
            OutputReference,
            "PropertyValue.Computed",
        ]
    ]

    def __init__(
        self,
        value: Optional[
            Union[
                bool,
                float,
                str,
                pulumi.Asset,
                pulumi.Archive,
                "PropertyValue",
                Sequence["PropertyValue"],
                Mapping[str, "PropertyValue"],
                ResourceReference,
                OutputReference,
                Computed,
            ]
        ],
    ) -> None:
        """
        :param value: The value of the property.
        :param is_computed: Whether the value is computed.
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

    @staticmethod
    def computed() -> "PropertyValue":
        """
        Creates a computed property value.
        """
        return PropertyValue(PropertyValue.Computed())

    @staticmethod
    def null() -> "PropertyValue":
        """
        Creates a null property value.
        """
        return PropertyValue(None)

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
        if isinstance(self.value, list):
            return PropertyValueType.ARRAY
        if isinstance(self.value, dict):
            return PropertyValueType.OBJECT
        if isinstance(self.value, ResourceReference):
            return PropertyValueType.RESOURCE
        if isinstance(self.value, OutputReference):
            return PropertyValueType.OUTPUT
        if isinstance(self.value, PropertyValue):
            return PropertyValueType.SECRET
        if isinstance(self.value, PropertyValue.Computed):
            return PropertyValueType.COMPUTED
        raise ValueError(f"Unsupported value type: {type(self.value)}")

    def __eq__(self, other: Any) -> bool:
        if not isinstance(other, PropertyValue):
            return False
        return self.value == other.value

    def __hash__(self) -> int:
        return hash(self.value)

    def __str__(self) -> str:
        return str(self.value)

    def is_secret(self) -> bool:
        """
        Determines if the property value is a secret.
        """
        return isinstance(self.value, PropertyValue)

    def contains_secret(self) -> bool:
        """
        Determines if the property value contains a secret.
        """
        if isinstance(self.value, PropertyValue):
            return True
        if isinstance(self.value, Sequence) and not isinstance(self.value, str):
            return any(v.contains_secret() for v in self.value)
        if isinstance(self.value, Mapping):
            return any(v.contains_secret() for v in self.value.values())
        if isinstance(self.value, OutputReference):
            return self.value.value is not None and self.value.value.contains_secret()
        return False

    def unwrap(self) -> "PropertyValue":
        """
        Attempts to unwrap the value if it is a secret or output.
        """
        if isinstance(self.value, PropertyValue):
            return self.value.unwrap()
        if isinstance(self.value, OutputReference):
            inner = self.value.value
            if inner is None:
                return PropertyValue.computed()
            return inner.unwrap()
        return self

    def marshal(
        self, property_dependencies: Optional[Set[str]] = None
    ) -> struct_pb2.Value:
        """
        Marshals a PropertyValue into a protobuf struct value.

        :param value: The PropertyValue to marshal.
        :return: A protobuf struct value representation of the PropertyValue.
        """
        if self.value is None:
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

        if isinstance(self.value, bool):
            return struct_pb2.Value(bool_value=self.value)

        if isinstance(self.value, float):
            return struct_pb2.Value(number_value=self.value)

        if isinstance(self.value, str):
            return struct_pb2.Value(string_value=self.value)

        if isinstance(self.value, Sequence) and not isinstance(self.value, str):
            pblist = []
            for item in self.value:
                pblist.append(item.marshal())
            return struct_pb2.Value(list_value=struct_pb2.ListValue(values=pblist))

        if isinstance(self.value, Mapping):
            pbstruct = {}
            for key, item in self.value.items():
                pbstruct[key] = item.marshal()
            return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

        if isinstance(self.value, pulumi.Asset):
            return marshal_asset(self.value)

        if isinstance(self.value, pulumi.Archive):
            return marshal_archive(self.value)

        if isinstance(self.value, PropertyValue):
            inner = self.value.marshal()
            pbstruct = {
                pulumi.runtime.rpc._special_sig_key: struct_pb2.Value(
                    string_value=pulumi.runtime.rpc._special_secret_sig
                ),
                "value": inner,
            }
            return struct_pb2.Value(struct_value=struct_pb2.Struct(fields=pbstruct))

        if isinstance(self.value, ResourceReference):
            raise NotImplementedError()

        if isinstance(self.value, OutputReference):
            raise NotImplementedError()

        raise ValueError(f"Unsupported value type: {type(self.value)}")

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
                return PropertyValue(PropertyValue.unmarshal(inner))

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

            raise ValueError(f"Unknown signature key for struct value: {sig}")

        raise ValueError(f"Unsupported value type: {kind}")

    @staticmethod
    def unmarshal_map(
        value: struct_pb2.Struct,
        dependencies: Optional[dict[str, Set[str]]] = None,
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

            if dependencies is not None:
                deps = dependencies.get(key)
                if deps is not None:
                    prop = result[key]
                    if isinstance(prop.value, OutputReference):
                        new_deps = prop.value.dependencies.union(deps)
                        result[key] = PropertyValue(
                            OutputReference(prop.value.value, dependencies=new_deps)
                        )
                    else:
                        result[key] = PropertyValue(
                            OutputReference(prop, dependencies=deps)
                        )
        return result

    @staticmethod
    def marshal_map(
        value: Dict[str, "PropertyValue"],
        property_dependencies: Optional[Dict[str, Set[str]]] = None,
    ) -> struct_pb2.Struct:
        """
        Marshals a dictionary of PropertyValues into a protobuf struct value.

        :param value: The dictionary of PropertyValues to marshal.
        :return: A protobuf struct value representation of the PropertyValues.
        """
        pbstruct = {}
        for key, item in value.items():
            dependencies: Set[str] = set()
            pbstruct[key] = item.marshal(dependencies)
            if property_dependencies is not None:
                property_dependencies[key] = dependencies

        return struct_pb2.Struct(fields=pbstruct)

    @staticmethod
    async def serialize(
        value: "pulumi.Input[Any]",
        deps: Optional[List["pulumi.Resource"]],
        property_key: Optional[str],
        resource_obj: Optional["pulumi.Resource"] = None,
        input_transformer: Optional[Callable[[str], str]] = None,
        typ: Optional[builtins.type] = None,
        keep_output_values: bool = False,
        exclude_resource_refs_from_deps: bool = False,
    ) -> "PropertyValue":
        """
        Serializes a value into a PropertyValue. This is roughly equivalent to the `rpc.serialize_property` function,
        except it serializes to a `PropertyValue` instead of a `struct_pb2.Value`.

        When `typ` is specified, the metadata from the type is used to translate Python snake_case
        names to Pulumi camelCase names, rather than using the `input_transformer`.

        :param Input[Any] value: The value to serialize.

        :param Optional[List["Resource"]] deps: Dependent resources discovered during serialization are added to this list.

        :param Optional[str] property_key: The name of the property being serialized.

        :param Optional[Resource] resource_obj: Optional resource object to use to provide better error messages.

        :param Optional[Callable[[str], str]]input_transformer: Optional name translator.

        :param Optional[type] typ: Optional input type to use for name translations rather than using the
        input_transformer.

        :param bool keep_output_values: If true, output values will be kept (only if the monitor
        supports output values).

        :param bool exclude_resource_refs_from_deps: If true, resource references will not be added to
        `deps` during serialization (only if the monitor supports resource references). This is
        useful for remote components (i.e. MLCs) and resource method calls where we want property
        dependencies to be empty for a property that only contains resource references.

        :return: A PropertyValue representation of the value.
        """
        if isinstance(value, PropertyValue):
            return value

        # Set typ to T if it's Optional[T], Input[T], or InputType[T].
        typ = pulumi._types.unwrap_type(typ) if typ else typ

        # If the typ is Any, set it to None to treat it as if we don't have any type information,
        # to avoid raising errors about unexpected types, since it could be any type.
        if typ is Any:
            typ = None

        # Exclude some built-in types that are instances of Sequence that we don't want to treat as sequences here.
        # From: https://github.com/python/cpython/blob/master/Lib/_collections_abc.py
        if isinstance(value, abc.Sequence) and not isinstance(
            value, (str, range, memoryview, bytes, bytearray)
        ):
            element_type = pulumi.runtime.rpc._get_list_element_type(typ)
            props = []
            for elem in value:
                props.append(
                    await PropertyValue.serialize(
                        elem,
                        deps,
                        property_key,
                        resource_obj,
                        input_transformer,
                        element_type,
                        keep_output_values,
                        exclude_resource_refs_from_deps,
                    )
                )

            return PropertyValue(props)

        if known_types.is_unknown(value):
            return PropertyValue.computed()

        if known_types.is_resource(value):
            resource = cast("pulumi.Resource", value)

            is_custom = known_types.is_custom_resource(value)
            resource_id = cast("pulumi.CustomResource", value).id if is_custom else None

            if (
                exclude_resource_refs_from_deps
                and await settings.monitor_supports_resource_references()
            ):
                # If excluding resource references from dependencies and the monitor supports resource
                # references, we don't want to track this dependency, so we set `deps` to `None` so when
                # serializing the `id` and `urn` the resource won't be included in the caller's `deps`.
                deps = None

            # If we're retaining resources, serialize the resource as a reference.
            if await settings.monitor_supports_resource_references():
                urn = await resource.urn.future()
                if urn is None:
                    raise ValueError(
                        "Resource URN is None, this is likely a bug in the Pulumi SDK."
                    )
                res = ResourceReference(urn, None, "")
                if resource_id is not None:
                    id = await resource_id.future()
                    res = ResourceReference(urn, id, "")
                return PropertyValue(res)

            # Otherwise, serialize the resource as either its ID (for custom resources) or its URN (for component resources)
            return await PropertyValue.serialize(
                resource_id if is_custom else resource.urn,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )

        if known_types.is_asset(value):
            return PropertyValue(cast(pulumi.Asset, value))

        if known_types.is_archive(value):
            return PropertyValue(cast(pulumi.Archive, value))

        if inspect.isawaitable(value):
            # Coroutines and Futures are both awaitable. Coroutines need to be scheduled.
            # asyncio.ensure_future returns futures verbatim while converting coroutines into
            # futures by arranging for the execution on the event loop.
            #
            # The returned future can then be awaited to yield a value, which we'll continue
            # serializing.
            awaitable = cast("Any", value)
            future_return = await asyncio.ensure_future(awaitable)
            return await PropertyValue.serialize(
                future_return,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                typ,
                keep_output_values,
                exclude_resource_refs_from_deps,
            )

        if known_types.is_output(value):
            output = cast("pulumi.Output", value)
            value_resources: Set["pulumi.Resource"] = await output.resources()
            if deps is not None:
                deps.extend(value_resources)

            # When serializing an Output, we will either serialize it as its resolved value or the
            # "unknown value" sentinel. We will do the former for all outputs created directly by user
            # code (such outputs always resolve isKnown to true) and for any resource outputs that were
            # resolved with known values.
            is_known = await output._is_known
            is_secret = await output._is_secret
            promise_deps: List["pulumi.Resource"] = []
            inner: Optional[PropertyValue] = None
            if is_known:
                inner = await PropertyValue.serialize(
                    output.future(),
                    promise_deps,
                    property_key,
                    resource_obj,
                    input_transformer,
                    typ,
                    keep_output_values=False,
                )
            if deps is not None:
                deps.extend(promise_deps)
            value_resources.update(promise_deps)

            if keep_output_values and await settings.monitor_supports_output_values():
                urn_deps: List["pulumi.Resource"] = []
                for resource in value_resources:
                    await PropertyValue.serialize(
                        resource.urn,
                        urn_deps,
                        property_key,
                        resource_obj,
                        input_transformer,
                        keep_output_values=False,
                    )
                promise_deps.extend(set(urn_deps))
                value_resources.update(urn_deps)

                resources = await _expand_dependencies(value_resources, None)
                dependencies = set()
                for _, resource in resources.items():
                    urn = await resource.urn.future()
                    if urn is not None:
                        dependencies.add(urn)

                output_value = PropertyValue(OutputReference(inner, dependencies))

                if is_secret:
                    output_value = PropertyValue(output_value)

                return output_value

            if inner is None:
                return PropertyValue.computed()
            if is_secret and await settings.monitor_supports_secrets():
                # Serializing an output with a secret value requires the use of a magical signature key,
                # which the engine detects.
                return PropertyValue(inner)
            return inner

        # If value is an input type, convert it to a dict.
        value_cls = type(value)
        if _types.is_input_type(value_cls):
            value = _types.input_type_to_dict(value)
            types = _types.input_type_types(value_cls)

            return PropertyValue(
                {
                    k: await PropertyValue.serialize(
                        v,
                        deps,
                        property_key,
                        resource_obj,
                        input_transformer,
                        types.get(k),
                        keep_output_values,
                        exclude_resource_refs_from_deps,
                    )
                    for k, v in value.items()
                }
            )

        if isinstance(value, abc.Mapping):
            # Default implementation of get_type that always returns None.
            get_type: Callable[[str], Optional[type]] = lambda k: None
            # Key translator.
            translate = input_transformer

            # If we have type information, we'll use it to do name translations rather than using
            # any passed-in input_transformer.
            if typ is not None:
                if _types.is_input_type(typ):
                    # If it's intended to be an input type, translate using the type's metadata.
                    py_name_to_pulumi_name = _types.input_type_py_to_pulumi_names(typ)
                    types = _types.input_type_types(typ)
                    translate = lambda k: py_name_to_pulumi_name.get(k) or k

                    get_type = types.get
                else:
                    # Otherwise, don't do any translation of user-defined dict keys.
                    origin = get_origin(typ)
                    if typ is dict or origin in [dict, Dict, Mapping, abc.Mapping]:
                        args = get_args(typ)
                        if len(args) == 2 and args[0] is str:
                            get_type = lambda k: args[1]
                            translate = None
                    else:
                        translate = None
                        # Note: Alternatively, we could assert here that we expected a dict type but got some other type,
                        # but there are cases where we've historically allowed a user-defined dict value to be passed even
                        # though the type annotation isn't a dict type (e.g. the `aws.s3.BucketPolicy.policy` input property
                        # is currently typed as `pulumi.Input[str]`, but we've allowed a dict to be passed, which will
                        # "magically" work at runtime because the provider will convert the dict value to a JSON string).
                        # Ideally, we'd update the type annotations for such cases to reflect that a dict could be passed,
                        # but we haven't done that yet and want these existing cases to continue to work as they have
                        # before.

            obj = {}
            # Don't use value.items() here, as it will error in the case of outputs with an `items` property.
            for k in value:
                transformed_key = k
                if translate is not None:
                    transformed_key = translate(k)
                    if settings.excessive_debug_output:
                        log.debug(
                            f"transforming input property: {k} -> {transformed_key}"
                        )
                obj[transformed_key] = await PropertyValue.serialize(
                    value[k],
                    deps,
                    k,
                    resource_obj,
                    input_transformer,
                    get_type(transformed_key),
                    keep_output_values,
                    exclude_resource_refs_from_deps,
                )

            return PropertyValue(obj)

        # Ensure that we have a value that Protobuf understands.
        if not isLegalProtobufValue(value):
            if property_key is not None and resource_obj is not None:
                raise ValueError(
                    f"unexpected input of type {type(value).__name__} for {property_key} in {type(resource_obj).__name__}"
                )
            if property_key is not None:
                raise ValueError(
                    f"unexpected input of type {type(value).__name__} for {property_key}"
                )
            if resource_obj is not None:
                raise ValueError(
                    f"unexpected input of type {type(value).__name__} in {type(resource_obj).__name__}"
                )
            raise ValueError(f"unexpected input of type {type(value).__name__}")

        return PropertyValue(cast(Any, value))

    @staticmethod
    def deserialize(
        property: "PropertyValue",
        keep_unknowns: bool = True,
        keep_secrets: bool = True,
    ) -> Any:
        """
        Deserializes a `PropertyValue` into  normal Python types.
        """

        if property is None:
            return None

        def make_computed() -> Any:
            if settings.is_dry_run() or keep_unknowns:
                resources: asyncio.Future[Set["pulumi.Resource"]] = asyncio.Future()
                future: asyncio.Future[Any] = asyncio.Future()
                is_known: asyncio.Future[bool] = asyncio.Future()
                is_secret: asyncio.Future[bool] = asyncio.Future()
                resources.set_result(set())
                future.set_result(None)
                is_known.set_result(False)
                is_secret.set_result(False)

                return pulumi.Output(resources, future, is_known, is_secret)
            else:
                return None

        if not isinstance(property, PropertyValue):
            raise TypeError(f"Expected a PropertyValue, got {type(property).__name__}")

        if isinstance(property.value, PropertyValue.Computed):
            return make_computed()

        if isinstance(property.value, PropertyValue):
            inner = PropertyValue.deserialize(property.value, keep_unknowns, False)
            if keep_secrets:
                return pulumi.Output.secret(inner)
            else:
                return inner

        if isinstance(property.value, abc.Mapping):
            secret = property.contains_secret()

            props = PropertyValue.deserialize_map(
                property.value, keep_unknowns, True, False
            )
            # If there are any secret values in the dictionary, push the secretness "up" a level by returning
            # a dictionary that is marked as a secret with raw values inside. Note: the isinstance check here is
            # important, since deserialize_properties will return either a dictionary or a concret type (in the case of
            # assets).
            if secret and keep_secrets:
                return pulumi.Output.secret(props)
            return props

        if isinstance(property.value, abc.Sequence) and not isinstance(
            property.value, str
        ):
            secret = property.contains_secret()
            values = [
                PropertyValue.deserialize(v, keep_unknowns, False)
                for v in property.value
            ]
            # If there are any secret values in the list, push the secretness "up" a level by returning
            # an array that is marked as a secret with raw values inside.
            if secret and keep_secrets:
                return pulumi.Output.secret(values)
            return values

        if isinstance(property.value, OutputReference):
            # If the value is an output reference, we need to unwrap it to get the actual value.

            deps: Set["pulumi.Resource"] = set()
            if property.value.dependencies:
                for urn in property.value.dependencies:
                    deps.add(DependencyResource(urn))

            resources: asyncio.Future[Set["pulumi.Resource"]] = asyncio.Future()
            future: asyncio.Future[Any] = asyncio.Future()
            is_known: asyncio.Future[bool] = asyncio.Future()
            is_secret: asyncio.Future[bool] = asyncio.Future()
            resources.set_result(deps)
            is_secret.set_result(False)

            inner = property.value.value
            if inner is None:
                is_known.set_result(False)
                future.set_result(None)
            else:
                inner_value = PropertyValue.deserialize(
                    inner, keep_unknowns, keep_secrets
                )
                # if it's known and has no dependencies we can just return the value
                if not deps:
                    return inner_value

                is_known.set_result(True)
                future.set_result(inner_value)

            return pulumi.Output(resources, future, is_known, is_secret)

        if isinstance(property.value, ResourceReference):
            raise NotImplementedError()

        # Everything else is identity projected.
        return property.value

    @staticmethod
    async def serialize_map(
        value: abc.Mapping[str, pulumi.Input[Any]],
        deps: Optional[List["pulumi.Resource"]] = None,
        property_key: Optional[str] = None,
        resource_obj: Optional["pulumi.Resource"] = None,
        input_transformer: Optional[Callable[[str], str]] = None,
        typ: Optional[builtins.type] = None,
        keep_output_values: bool = False,
        exclude_resource_refs_from_deps: bool = False,
    ) -> dict[str, "PropertyValue"]:
        result = await PropertyValue.serialize(
            value,
            deps,
            property_key,
            resource_obj,
            input_transformer,
            typ,
            keep_output_values,
            exclude_resource_refs_from_deps,
        )
        if isinstance(result.value, Mapping):
            return dict(result.value)
        raise TypeError(f"Expected a dictionary, got {type(result).__name__}")

    @staticmethod
    def deserialize_map(
        property: abc.Mapping[str, "PropertyValue"],
        keep_unknowns: bool = True,
        keep_internal: bool = False,
        keep_secrets: bool = True,
    ) -> dict[str, Any]:
        output = {}
        for k, v in property.items():
            # Unilaterally skip properties considered internal by the Pulumi engine.
            # These don't actually contribute to the exposed shape of the object, do
            # not need to be passed back to the engine, and often will not match the
            # expected type we are deserializing into.
            if not keep_internal and k.startswith("__"):
                continue

            value = PropertyValue.deserialize(v, keep_unknowns, keep_secrets)
            # We treat values that deserialize to "None" as if they don't exist.
            if value is not None:
                output[k] = value

        return output
