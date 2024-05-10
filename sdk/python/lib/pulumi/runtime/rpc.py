# Copyright 2016-2021, Pulumi Corporation.
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
import asyncio
import functools
import inspect
from abc import ABC, abstractmethod
from collections import abc
from enum import Enum
import os
from typing import (
    TYPE_CHECKING,
    Any,
    Callable,
    Dict,
    Iterable,
    List,
    Mapping,
    Optional,
    Sequence,
    Set,
    Union,
    cast,
    get_args,
    get_origin,
)

import six
from google.protobuf import struct_pb2
from semver import VersionInfo as Version

from .. import _types, log
from .. import urn as urn_util
from . import known_types, settings
from .resource_cycle_breaker import declare_dependency

if TYPE_CHECKING:
    from ..asset import (
        AssetArchive,
        FileArchive,
        FileAsset,
        RemoteArchive,
        RemoteAsset,
        StringAsset,
    )
    from ..output import Input, Inputs, Output
    from ..resource import CustomResource, ProviderResource, Resource

ERROR_ON_DEPENDENCY_CYCLES_VAR = "PULUMI_ERROR_ON_DEPENDENCY_CYCLES"
"""The name of the environment variable to set to false if you want to disable erroring on dependency cycles."""

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

_special_sig_key = "4dabf18193072939515e22adb298388d"
"""
_special_sig_key is sometimes used to encode type identity inside of a map.
See sdk/go/common/resource/properties.go.
"""

_special_asset_sig = "c44067f5952c0a294b673a41bacd8c17"
"""
special_asset_sig is a randomly assigned hash used to identify assets in maps.
See sdk/go/common/resource/asset.go.
"""

_special_archive_sig = "0def7320c3a5731c473e5ecbe6d01bc7"
"""
special_archive_sig is a randomly assigned hash used to identify assets in maps.
See sdk/go/common/resource/asset.go.
"""

_special_secret_sig = "1b47061264138c4ac30d75fd1eb44270"
"""
special_secret_sig is a randomly assigned hash used to identify secrets in maps.
See sdk/go/common/resource/properties.go.
"""

_special_resource_sig = "5cf8f73096256a8f31e491e813e4eb8e"
"""
special_resource_sig is a randomly assigned hash used to identify resources in maps.
See sdk/go/common/resource/properties.go.
"""

_special_output_value_sig = "d0e6a833031e9bbcd3f4e8bde6ca49a4"
"""
_special_output_value_sig is a randomly assigned hash used to identify outputs in maps.
See sdk/go/common/resource/properties.go.
"""

_INT_OR_FLOAT = six.integer_types + (float,)

# This setting overrides a hardcoded maximum protobuf size in the python protobuf bindings. This avoids deserialization
# exceptions on large gRPC payloads, but makes it possible to use enough memory to cause an OOM error instead [1].
# Note: We hit the default maximum protobuf size in practice when processing Kubernetes CRDs [2]. If this setting ends
# up causing problems, it should be possible to work around it with more intelligent resource chunking in the k8s
# provider.
#
# [1] https://github.com/protocolbuffers/protobuf/blob/0a59054c30e4f0ba10f10acfc1d7f3814c63e1a7/python/google/protobuf/pyext/message.cc#L2017-L2024
# [2] https://github.com/pulumi/pulumi-kubernetes/issues/984
#
# This setting requires a platform-specific and python version-specific .so file called
# `_message.cpython-[py-version]-[platform].so`, which is not present in situations when a new python version is
# released but the corresponding dist wheel has not been. So, we wrap the import in a try/except to avoid breaking all
# python programs using a new version.
try:
    from google.protobuf.pyext._message import (  # pylint: disable-msg=C0412
        SetAllowOversizeProtos,
    )  # pylint: disable-msg=E0611

    SetAllowOversizeProtos(True)
except ImportError:
    pass

# New versions of protobuf have moved the above import to api_implementation
try:
    from google.protobuf.pyext import (  # type: ignore
        cpp_message,
    )  # pylint: disable-msg=E0611

    if cpp_message._message is not None:
        cpp_message._message.SetAllowOversizeProtos(True)
except ImportError:
    pass


def isLegalProtobufValue(value: Any) -> bool:
    """
    Returns True if the given value is a legal Protobuf value as per the source at
    https://github.com/protocolbuffers/protobuf/blob/master/python/google/protobuf/internal/well_known_types.py#L714-L732
    """
    return value is None or isinstance(
        value, (bool, six.string_types, _INT_OR_FLOAT, dict, list)
    )


def _get_list_element_type(typ: Optional[type]) -> Optional[type]:
    if typ is None:
        return None

    # Annotations not specifying the element type are assumed by mypy
    # to signify Any element type. Follow suit here.
    if typ in [list, List, Sequence, abc.Sequence]:
        return cast(type, Any)

    # If typ is a list, get the type for its values, to pass
    # along for each item.
    origin = get_origin(typ)
    if typ is list or origin in [list, List, Sequence, abc.Sequence]:
        args = get_args(typ)
        if len(args) == 1:
            return args[0]

    raise AssertionError(f"Unexpected type. Expected 'list' got '{typ}'")


async def serialize_properties(
    inputs: "Inputs",
    property_deps: Dict[str, List["Resource"]],
    resource_obj: Optional["Resource"] = None,
    input_transformer: Optional[Callable[[str], str]] = None,
    typ: Optional[type] = None,
    keep_output_values: Optional[bool] = None,
) -> struct_pb2.Struct:
    """
    Serializes an arbitrary Input bag into a Protobuf structure, keeping track of the list
    of dependent resources in the `deps` list. Serializing properties is inherently async
    because it awaits any futures that are contained transitively within the input bag.

    When `typ` is an input type, the metadata from the type is used to translate Python snake_case
    names to Pulumi camelCase names, rather than using the `input_transformer`.

    Modifies given property_deps dict to collect discovered dependencies by property name.

    :param Inputs inputs: The bag to serialize.

    :param Dict[str, List[Resource]] property_deps: Dependencies are set here.

    :param input_transfomer: Optional name translator.
    """

    # Default implementation of get_type that always returns None.
    get_type: Callable[[str], Optional[type]] = lambda k: None
    # Key translator.
    translate = input_transformer

    # If we have type information, we'll use it to do name translations rather than using
    # any passed-in input_transformer.
    if typ is not None:
        py_name_to_pulumi_name = _types.input_type_py_to_pulumi_names(typ)
        types = _types.input_type_types(typ)
        # pylint: disable=C3001
        translate = lambda k: py_name_to_pulumi_name.get(k) or k
        # pylint: disable=C3001
        get_type = lambda k: types.get(translate(k))  # type: ignore

    struct = struct_pb2.Struct()
    # We're deliberately not using `inputs.items()` here in case inputs is a subclass of `dict` that redefines items.
    for k in inputs:
        v = inputs[k]
        deps: List["Resource"] = []
        result = await serialize_property(
            v,
            deps,
            k,
            resource_obj,
            input_transformer,
            get_type(k),
            keep_output_values,
        )
        # We treat properties that serialize to None as if they don't exist.
        if result is not None:
            # While serializing to a pb struct, we must "translate" all key names to be what the
            # engine is going to expect. Resources provide the "transform" function for doing this.
            translated_name = k
            if translate is not None:
                translated_name = translate(k)
                if settings.excessive_debug_output:
                    log.debug(
                        f"top-level input property translated: {k} -> {translated_name}"
                    )
            # pylint: disable=unsupported-assignment-operation
            struct[translated_name] = result
            property_deps[translated_name] = deps

    return struct


async def _add_dependency(
    deps: Set[str], res: "Resource", from_resource: Optional["Resource"]
):
    """
    _add_dependency adds a dependency on the given resource to the set of deps.

    The behavior of this method depends on whether or not the resource is a custom resource, a local component resource,
    or a remote component resource:

    - Custom resources are added directly to the set, as they are "real" nodes in the dependency graph.
    - Local component resources act as aggregations of their descendents. Rather than adding the component resource
      itself, each child resource is added as a dependency.
    - Remote component resources are added directly to the set, as they naturally act as aggregations of their children
      with respect to dependencies: the construction of a remote component always waits on the construction of its
      children.

    In other words, if we had:

                     Comp1
                 |     |     |
             Cust1   Comp2  Remote1
                     |   |       |
                 Cust2   Cust3  Comp3
                 |                 |
             Cust4                Cust5

    Then the transitively reachable resources of Comp1 will be [Cust1, Cust2, Cust3, Remote1].
    It will *not* include:
    * Cust4 because it is a child of a custom resource
    * Comp2 because it is a non-remote component resoruce
    * Comp3 and Cust5 because Comp3 is a child of a remote component resource
    """
    from ..resource import Resource  # pylint: disable=import-outside-toplevel

    if not isinstance(res, Resource):
        raise TypeError(
            f"'depends_on' was passed a value {res} that was not a Resource."
        )

    # Exit early if there are cycles to avoid hangs.
    no_cycles = declare_dependency(from_resource, res) if from_resource else True
    if not no_cycles:
        error_on_cycles = (
            os.getenv(ERROR_ON_DEPENDENCY_CYCLES_VAR, "true").lower() == "true"
        )
        if not error_on_cycles:
            return
        raise RuntimeError(
            "We have detected a circular dependency involving a resource of type"
            + f" {res._type} named {res._name}.\n"
            + "Please review any `depends_on`, `parent` or other dependency relationships between your resources to ensure "
            + "no cycles have been introduced in your program."
        )

    from .. import ComponentResource  # pylint: disable=import-outside-toplevel

    # Local component resources act as aggregations of their descendents.
    # Rather than adding the component resource itself, each child resource
    # is added as a dependency.
    if isinstance(res, ComponentResource) and not res._remote:
        # Copy the set before iterating so that any concurrent child additions during
        # the dependency computation (which is async, so can be interleaved with other
        # operations including child resource construction which adds children to this
        # resource) do not trigger modification during iteration errors.
        child_resources = res._childResources.copy()
        for child in child_resources:
            await _add_dependency(deps, child, from_resource)
        return

    urn = await res.urn.future()
    if urn:
        deps.add(urn)


async def _expand_dependencies(
    deps: Iterable["Resource"], from_resource: Optional["Resource"]
) -> Set[str]:
    """
    _expand_dependencies expands the given iterable of Resources into a set of URNs.
    """

    urns: Set[str] = set()
    for d in deps:
        await _add_dependency(urns, d, from_resource)

    return urns


# pylint: disable=too-many-return-statements, too-many-branches
async def serialize_property(
    value: "Input[Any]",
    deps: List["Resource"],
    property_key: Optional[str],
    resource_obj: Optional["Resource"] = None,
    input_transformer: Optional[Callable[[str], str]] = None,
    typ: Optional[type] = None,
    keep_output_values: Optional[bool] = None,
) -> Any:
    """
    Serializes a single Input into a form suitable for remoting to the engine, awaiting
    any futures required to do so.

    When `typ` is specified, the metadata from the type is used to translate Python snake_case
    names to Pulumi camelCase names, rather than using the `input_transformer`.

    If `keep_output_values` is true and the monitor supports output values, they will be kept.
    """

    # Set typ to T if it's Optional[T], Input[T], or InputType[T].
    typ = _types.unwrap_type(typ) if typ else typ

    # If the typ is Any, set it to None to treat it as if we don't have any type information,
    # to avoid raising errors about unexpected types, since it could be any type.
    if typ is Any:
        typ = None

    # Exclude some built-in types that are instances of Sequence that we don't want to treat as sequences here.
    # From: https://github.com/python/cpython/blob/master/Lib/_collections_abc.py
    if isinstance(value, abc.Sequence) and not isinstance(
        value, (str, range, memoryview, bytes, bytearray)
    ):
        element_type = _get_list_element_type(typ)
        props = []
        for elem in value:
            props.append(
                await serialize_property(
                    elem,
                    deps,
                    property_key,
                    resource_obj,
                    input_transformer,
                    element_type,
                    keep_output_values,
                )
            )

        return props

    if known_types.is_unknown(value):
        return UNKNOWN

    if known_types.is_resource(value):
        resource = cast("Resource", value)

        is_custom = known_types.is_custom_resource(value)
        resource_id = cast("CustomResource", value).id if is_custom else None

        # If we're retaining resources, serialize the resource as a reference.
        if await settings.monitor_supports_resource_references():
            res = {
                _special_sig_key: _special_resource_sig,
                "urn": await serialize_property(
                    resource.urn,
                    deps,
                    property_key,
                    resource_obj,
                    input_transformer,
                    keep_output_values=False,
                ),
            }
            if is_custom:
                res["id"] = await serialize_property(
                    resource_id,
                    deps,
                    property_key,
                    resource_obj,
                    input_transformer,
                    keep_output_values=False,
                )
            return res

        # Otherwise, serialize the resource as either its ID (for custom resources) or its URN (for component resources)
        return await serialize_property(
            resource_id if is_custom else resource.urn,
            deps,
            property_key,
            resource_obj,
            input_transformer,
            keep_output_values=False,
        )

    if known_types.is_asset(value):
        # Serializing an asset requires the use of a magical signature key, since otherwise it would
        # look like any old weakly typed object/map when received by the other side of the RPC
        # boundary.
        obj = {_special_sig_key: _special_asset_sig}

        if hasattr(value, "path"):
            file_asset = cast("FileAsset", value)
            obj["path"] = await serialize_property(
                file_asset.path,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        elif hasattr(value, "text"):
            str_asset = cast("StringAsset", value)
            obj["text"] = await serialize_property(
                str_asset.text,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        elif hasattr(value, "uri"):
            remote_asset = cast("RemoteAsset", value)
            obj["uri"] = await serialize_property(
                remote_asset.uri,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        else:
            raise AssertionError(f"unknown asset type: {value!r}")

        return obj

    if known_types.is_archive(value):
        # Serializing an archive requires the use of a magical signature key, since otherwise it
        # would look like any old weakly typed object/map when received by the other side of the RPC
        # boundary.
        obj = {_special_sig_key: _special_archive_sig}

        if hasattr(value, "assets"):
            asset_archive = cast("AssetArchive", value)
            obj["assets"] = await serialize_property(
                asset_archive.assets,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        elif hasattr(value, "path"):
            file_archive = cast("FileArchive", value)
            obj["path"] = await serialize_property(
                file_archive.path,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        elif hasattr(value, "uri"):
            remote_archive = cast("RemoteArchive", value)
            obj["uri"] = await serialize_property(
                remote_archive.uri,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                keep_output_values=False,
            )
        else:
            raise AssertionError(f"unknown archive type: {value!r}")

        return obj

    if inspect.isawaitable(value):
        # Coroutines and Futures are both awaitable. Coroutines need to be scheduled.
        # asyncio.ensure_future returns futures verbatim while converting coroutines into
        # futures by arranging for the execution on the event loop.
        #
        # The returned future can then be awaited to yield a value, which we'll continue
        # serializing.
        awaitable = cast("Any", value)
        future_return = await asyncio.ensure_future(awaitable)
        return await serialize_property(
            future_return,
            deps,
            property_key,
            resource_obj,
            input_transformer,
            typ,
            keep_output_values,
        )

    if known_types.is_output(value):
        output = cast("Output", value)
        value_resources: Set["Resource"] = await output.resources()
        deps.extend(value_resources)

        # When serializing an Output, we will either serialize it as its resolved value or the
        # "unknown value" sentinel. We will do the former for all outputs created directly by user
        # code (such outputs always resolve isKnown to true) and for any resource outputs that were
        # resolved with known values.
        is_known = await output._is_known
        is_secret = await output._is_secret
        promise_deps: List["Resource"] = []
        value = await serialize_property(
            output.future(),
            promise_deps,
            property_key,
            resource_obj,
            input_transformer,
            typ,
            keep_output_values=False,
        )
        deps.extend(promise_deps)
        value_resources.update(promise_deps)

        if keep_output_values and await settings.monitor_supports_output_values():
            urn_deps: List["Resource"] = []
            for resource in value_resources:
                await serialize_property(
                    resource.urn,
                    urn_deps,
                    property_key,
                    resource_obj,
                    input_transformer,
                    keep_output_values=False,
                )
            promise_deps.extend(set(urn_deps))
            value_resources.update(urn_deps)

            dependencies = await _expand_dependencies(value_resources, None)

            output_value: Dict[str, Any] = {_special_sig_key: _special_output_value_sig}

            if is_known:
                output_value["value"] = value
            if is_secret:
                output_value["secret"] = is_secret
            if dependencies:
                output_value["dependencies"] = sorted(dependencies)

            return output_value

        if not is_known:
            return UNKNOWN
        if is_secret and await settings.monitor_supports_secrets():
            # Serializing an output with a secret value requires the use of a magical signature key,
            # which the engine detects.
            return {_special_sig_key: _special_secret_sig, "value": value}
        return value

    # If value is an input type, convert it to a dict.
    value_cls = type(value)
    if _types.is_input_type(value_cls):
        value = _types.input_type_to_dict(value)
        types = _types.input_type_types(value_cls)

        return {
            k: await serialize_property(
                v,
                deps,
                property_key,
                resource_obj,
                input_transformer,
                types.get(k),
                keep_output_values,
            )
            for k, v in value.items()
        }

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
                # pylint: disable=C3001
                types = _types.input_type_types(typ)
                # pylint: disable=C3001
                translate = lambda k: py_name_to_pulumi_name.get(k) or k
                get_type = types.get
            else:
                # Otherwise, don't do any translation of user-defined dict keys.
                origin = get_origin(typ)
                if typ is dict or origin in [dict, Dict, Mapping, abc.Mapping]:
                    args = get_args(typ)
                    if len(args) == 2 and args[0] is str:
                        # pylint: disable=C3001
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
                    log.debug(f"transforming input property: {k} -> {transformed_key}")
            obj[transformed_key] = await serialize_property(
                value[k],
                deps,
                k,
                resource_obj,
                input_transformer,
                get_type(transformed_key),
                keep_output_values,
            )

        return obj

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

    return value


# pylint: disable=too-many-return-statements
def deserialize_properties(
    props_struct: struct_pb2.Struct,
    keep_unknowns: Optional[bool] = None,
    keep_internal: Optional[bool] = None,
) -> Any:
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
        from .. import (  # pylint: disable=import-outside-toplevel
            AssetArchive,
            FileArchive,
            FileAsset,
            RemoteArchive,
            RemoteAsset,
            StringAsset,
        )

        if props_struct[_special_sig_key] == _special_asset_sig:
            # This is an asset. Re-hydrate this object into an Asset.
            if "path" in props_struct:
                return FileAsset(str(props_struct["path"]))
            if "text" in props_struct:
                return StringAsset(str(props_struct["text"]))
            if "uri" in props_struct:
                return RemoteAsset(str(props_struct["uri"]))
            raise AssertionError(
                "Invalid asset encountered when unmarshalling resource property"
            )
        if props_struct[_special_sig_key] == _special_archive_sig:
            # This is an archive. Re-hydrate this object into an Archive.
            if "assets" in props_struct:
                return AssetArchive(deserialize_property(props_struct["assets"]))
            if "path" in props_struct:
                return FileArchive(str(props_struct["path"]))
            if "uri" in props_struct:
                return RemoteArchive(str(props_struct["uri"]))
            raise AssertionError(
                "Invalid archive encountered when unmarshalling resource property"
            )
        if props_struct[_special_sig_key] == _special_secret_sig:
            return wrap_rpc_secret(deserialize_property(props_struct["value"]))
        if props_struct[_special_sig_key] == _special_resource_sig:
            return deserialize_resource(props_struct, keep_unknowns)
        if props_struct[_special_sig_key] == _special_output_value_sig:
            return deserialize_output_value(props_struct)
        raise AssertionError(
            "Unrecognized signature when unmarshalling resource property"
        )

    # Struct is duck-typed like a dictionary, so we can iterate over it in the normal ways. Note
    # that if the struct had any secret properties, we push the secretness of the object up to us
    # since we can only set secret outputs on top level properties.
    output = {}
    for k, v in list(props_struct.items()):
        # Unilaterally skip properties considered internal by the Pulumi engine.
        # These don't actually contribute to the exposed shape of the object, do
        # not need to be passed back to the engine, and often will not match the
        # expected type we are deserializing into.
        # Keep "__provider" as it's the property name used by Python dynamic providers.
        if not keep_internal and k.startswith("__") and k != "__provider":
            continue

        value = deserialize_property(v, keep_unknowns)
        # We treat values that deserialize to "None" as if they don't exist.
        if value is not None:
            output[k] = value

    return output


def deserialize_resource(
    ref_struct: struct_pb2.Struct, keep_unknowns: Optional[bool] = None
) -> Union["Resource", str]:
    urn = str(ref_struct["urn"])
    version = (
        str(ref_struct["packageVersion"]) if "packageVersion" in ref_struct else ""
    )

    urn_parts = urn_util._parse_urn(urn)
    urn_name = urn_parts.urn_name
    typ = urn_parts.typ
    pkg_name = urn_parts.pkg_name
    mod_name = urn_parts.mod_name
    typ_name = urn_parts.typ_name

    is_provider = pkg_name == "pulumi" and mod_name == "providers"
    if is_provider:
        resource_package = get_resource_package(typ_name, version)
        if resource_package is not None:
            return cast(
                "Resource", resource_package.construct_provider(urn_name, typ, urn)
            )
    else:
        resource_module = get_resource_module(pkg_name, mod_name, version)
        if resource_module is not None:
            return cast("Resource", resource_module.construct(urn_name, typ, urn))

    # If we've made it here, deserialize the reference as either a URN or an ID (if present).
    if "id" in ref_struct:
        ref_id = ref_struct["id"]
        return deserialize_property(UNKNOWN if ref_id == "" else ref_id, keep_unknowns)

    return urn


def deserialize_output_value(ref_struct: struct_pb2.Struct) -> "Output[Any]":
    is_known = "value" in ref_struct
    is_known_future: "asyncio.Future" = asyncio.Future()
    is_known_future.set_result(is_known)

    value = None
    if is_known:
        value = deserialize_property(ref_struct["value"])
    value_future: "asyncio.Future" = asyncio.Future()
    value_future.set_result(value)

    is_secret = False
    if "secret" in ref_struct:
        is_secret = deserialize_property(ref_struct["secret"]) is True
    is_secret_future: "asyncio.Future" = asyncio.Future()
    is_secret_future.set_result(is_secret)

    resources: Set["Resource"] = set()
    if "dependencies" in ref_struct:
        from ..resource import (  # pylint: disable=import-outside-toplevel
            DependencyResource,
        )

        dependencies = cast(List[str], deserialize_property(ref_struct["dependencies"]))
        for urn in dependencies:
            resources.add(DependencyResource(urn))

    from .. import Output  # pylint: disable=import-outside-toplevel

    return Output(resources, value_future, is_known_future, is_secret_future)


def is_rpc_secret(value: Any) -> bool:
    """
    Returns if a given python value is actually a wrapped secret.
    """
    return (
        isinstance(value, dict)
        and _special_sig_key in value
        and value[_special_sig_key] == _special_secret_sig
    )


def wrap_rpc_secret(value: Any) -> Any:
    """
    Given a value, wrap it as a secret value if it isn't already a secret, otherwise return the value unmodified.
    """
    if is_rpc_secret(value):
        return value

    return {
        _special_sig_key: _special_secret_sig,
        "value": value,
    }


def unwrap_rpc_secret(value: Any) -> Any:
    """
    Given a value, if it is a wrapped secret value, return the underlying, otherwise return the value unmodified.
    """
    if is_rpc_secret(value):
        return value["value"]

    return value


def deserialize_property(value: Any, keep_unknowns: Optional[bool] = None) -> Any:
    """
    Deserializes a single protobuf value (either `Struct` or `ListValue`) into idiomatic
    Python values.
    """
    from ..output import Unknown  # pylint: disable=import-outside-toplevel

    if value == UNKNOWN:
        return Unknown() if settings.is_dry_run() or keep_unknowns else None

    # ListValues are projected to lists
    if isinstance(value, struct_pb2.ListValue):
        # values has no __iter__ defined but this works.
        values = [deserialize_property(v, keep_unknowns) for v in value]  # type: ignore
        # If there are any secret values in the list, push the secretness "up" a level by returning
        # an array that is marked as a secret with raw values inside.
        if any(is_rpc_secret(v) for v in values):
            return wrap_rpc_secret([unwrap_rpc_secret(v) for v in values])

        return values

    # Structs are projected to dictionaries
    if isinstance(value, struct_pb2.Struct):
        props = deserialize_properties(value, keep_unknowns)
        # If there are any secret values in the dictionary, push the secretness "up" a level by returning
        # a dictionary that is marked as a secret with raw values inside. Note: the isinstance check here is
        # important, since deserialize_properties will return either a dictionary or a concret type (in the case of
        # assets).
        if isinstance(props, dict) and any(is_rpc_secret(v) for v in props.values()):
            return wrap_rpc_secret({k: unwrap_rpc_secret(v) for k, v in props.items()})

        return props

    # Everything else is identity projected.
    return value


Resolver = Callable[
    [Any, bool, bool, Optional[Set["Resource"]], Optional[Exception]], None
]
"""
A Resolver is a function that takes four arguments:
    1. A value, which represents the "resolved" value of a particular output (from the engine)
    2. A boolean "is_known", which represents whether or not this value is known to have a particular value at this
       point in time (not always true for previews), and
    3. A boolean "is_secret", which represents whether or not this value is contains secret data, and
    4. An exception, which (if provided) is an exception that occured when attempting to create the resource to whom
       this resolver belongs.

If argument 4 is not none, this output is considered to be abnormally resolved and attempts to await its future will
result in the exception being re-thrown.
"""


def transfer_properties(
    res: "Resource", props: "Inputs", custom: bool
) -> Dict[str, Resolver]:
    from .. import Output  # pylint: disable=import-outside-toplevel

    resolvers: Dict[str, Resolver] = {}

    for name in props:
        if name == "urn" or (name == "id" and custom):
            # these properties are handled specially elsewhere.
            continue

        resolve_value: "asyncio.Future" = asyncio.Future()
        resolve_is_known: "asyncio.Future" = asyncio.Future()
        resolve_is_secret: "asyncio.Future" = asyncio.Future()
        resolve_deps: "asyncio.Future" = asyncio.Future()

        def do_resolve(
            r: "Resource",
            value_fut: "asyncio.Future",
            known_fut: "asyncio.Future[bool]",
            secret_fut: "asyncio.Future[bool]",
            deps_fut: "asyncio.Future[Set[Resource]]",
            value: Any,
            is_known: bool,
            is_secret: bool,
            deps: Set["Resource"],
            failed: Optional[Exception],
        ):
            # Create a union of deps and the resource.
            deps_union = set(deps) if deps else set()
            deps_union.add(r)
            deps_fut.set_result(deps_union)

            # Was an exception provided? If so, this is an abnormal (exceptional) resolution. Resolve the futures
            # using set_exception so that any attempts to wait for their resolution will also fail.
            if failed is not None:
                value_fut.set_exception(failed)
                known_fut.set_exception(failed)
                secret_fut.set_exception(failed)
            else:
                value_fut.set_result(value)
                known_fut.set_result(is_known)
                secret_fut.set_result(is_secret)

        # Important to note here is that the resolver's future is assigned to the resource object using the
        # name before translation. When properties are returned from the engine, we must first translate the name
        # from the Pulumi name to the Python name and then use *that* name to index into the resolvers table.
        resolvers[name] = functools.partial(
            do_resolve,
            res,
            resolve_value,
            resolve_is_known,
            resolve_is_secret,
            resolve_deps,
        )
        res.__dict__[name] = Output(
            resolve_deps, resolve_value, resolve_is_known, resolve_is_secret
        )

    return resolvers


def translate_output_properties(
    output: Any,
    output_transformer: Callable[[str], str],
    typ: Optional[type] = None,
    transform_using_type_metadata: bool = False,
    path: Optional["_Path"] = None,
    return_none_on_dict_type_mismatch: bool = False,
) -> Any:
    """
    Recursively rewrite keys of objects returned by the engine to conform with a naming
    convention specified by `output_transformer`. If `transform_using_type_metadata` is
    set to True, then the metadata from `typ` is used to do the translation, and `dict`
    values that are intended to be user-defined dicts aren't translated at all.

    Additionally, perform any type conversions as necessary, based on the optional `typ` parameter.

    If output is a `dict`, every key is translated (unless `transform_using_type_metadata is True,
    the dict isn't an output type, and it is intended to be a user-defined dict) while every value is
    transformed by recursing.

    If output is a `list`, every value is recursively transformed.

    If output is a `dict` and `typ` is an output type, instantiate the output type,
    passing the values in the dict to the output type's __init__() method.

    If output is a `float` and `typ` is `int`, the value is cast to `int`.

    If output is in [`str`, `int`, `float`] and `typ` is an enum type, instantiate the enum type.

    Otherwise, if output is a primitive (i.e. not a dict or list), the value is returned without modification.

    :param Any output: The output value.
    :param Callable[[str], str] output_transformer: The function used to translate.
    :param Optional[type] typ: The output's target type.
    :param bool transform_using_type_metadata: Set to True to use the metadata from `typ` to do name translation instead
                                               of using `output_transformer`.

    :param Optional[Any] path: Used internally to track recursive descent and enhance error messages.
    """

    # If it's a secret, unwrap the value so the output is in alignment with the expected type, call
    # translate_output_properties with the unwrapped value, and then rewrap the result as a secret.
    if is_rpc_secret(output):
        unwrapped = unwrap_rpc_secret(output)
        result = translate_output_properties(
            unwrapped,
            output_transformer,
            typ,
            transform_using_type_metadata,
            path,
            return_none_on_dict_type_mismatch,
        )
        return wrap_rpc_secret(result)

    # Unwrap optional types.
    typ = _types.unwrap_optional_type(typ) if typ else typ

    # If the typ is Any, set it to None to treat it as if we don't have any type information,
    # to avoid raising errors about unexpected types, since it could be any type.
    if typ is Any:
        typ = None

    if isinstance(output, dict):
        # Function called to lookup a type for a given key.
        # The default always returns None.
        get_type: Callable[[str], Optional[type]] = lambda k: None
        translate = output_transformer

        if typ is not None:
            # If typ is an output type, instantiate it. We do not translate the top-level keys,
            # as the output type will take care of doing that if it has a _translate_property()
            # method.
            if _types.is_output_type(typ):
                # If typ is an output type, get its types, so we can pass the type along for each property.
                types = _types.output_type_types(typ)
                get_type = types.get

                translated_values = {
                    k: translate_output_properties(
                        v,
                        output_transformer,
                        get_type(k),
                        transform_using_type_metadata,
                        _Path(k, parent=path),
                        return_none_on_dict_type_mismatch,
                    )
                    for k, v in output.items()
                }
                return _types.output_type_from_dict(typ, translated_values)

            # If typ is a dict, get the type for its values, to pass along for each key.
            origin = get_origin(typ)
            if typ is dict or origin in [dict, Dict, Mapping, abc.Mapping]:
                args = get_args(typ)
                if len(args) == 2 and args[0] is str:
                    # pylint: disable=C3001
                    get_type = lambda k: args[1]
                    # If transform_using_type_metadata is True, don't translate its keys because
                    # it is intended to be a user-defined dict.
                    if transform_using_type_metadata:
                        # pylint: disable=C3001
                        translate = lambda k: k
            elif return_none_on_dict_type_mismatch:
                return None
            else:
                raise AssertionError(
                    (
                        f"Unexpected type; expected a value of type `{typ}`"
                        f" but got a value of type `{dict}`{_Path.format(path)}:"
                        f" {output}"
                    )
                )

        return {
            translate(k): translate_output_properties(
                v,
                output_transformer,
                get_type(k),
                transform_using_type_metadata,
                _Path(k, parent=path),
                return_none_on_dict_type_mismatch,
            )
            for k, v in output.items()
        }

    if isinstance(output, list):
        element_type = _get_list_element_type(typ)
        return [
            translate_output_properties(
                v,
                output_transformer,
                element_type,
                transform_using_type_metadata,
                _Path(str(i), parent=path),
                return_none_on_dict_type_mismatch,
            )
            for i, v in enumerate(output)
        ]

    if (
        typ
        and isinstance(output, (int, float, str))
        and inspect.isclass(typ)
        and issubclass(typ, Enum)
    ):
        return typ(output)

    if isinstance(output, float) and typ is int:
        return int(output)

    return output


class _Path:
    """Internal helper for `translate_output_properties` error reporting,
    essentially an immutable linked list of prop names with an
    additional context resource name slot.

    """

    prop: str
    resource: Optional[str]
    parent: Optional["_Path"]

    def __init__(
        self,
        prop: str,
        parent: Optional["_Path"] = None,
        resource: Optional[str] = None,
    ) -> None:
        self.prop = prop
        self.parent = parent
        self.resource = resource

    @staticmethod
    def format(path: Optional["_Path"]) -> str:
        chain: List[str] = []
        p: Optional[_Path] = path
        resource: Optional[str] = None

        while p is not None:
            chain.append(p.prop)
            resource = p.resource or resource
            p = p.parent

        chain.reverse()

        coordinates = []

        if resource is not None:
            coordinates.append(f"resource `{resource}`")

        if chain:
            coordinates.append(f'property `{".".join(chain)}`')

        if coordinates:
            return f' at {", ".join(coordinates)}'

        return ""


def contains_unknowns(val: Any) -> bool:
    def impl(val: Any, stack: List[Any]) -> bool:
        if known_types.is_unknown(val):
            return True

        if not any((x is val for x in stack)):
            stack.append(val)
            if isinstance(val, dict):
                return any((impl(val[k], stack) for k in val))
            if isinstance(val, list):
                return any((impl(x, stack) for x in val))
        return False

    return impl(val, [])


def resolve_outputs(
    res: "Resource",
    serialized_props: struct_pb2.Struct,
    outputs: struct_pb2.Struct,
    deps: Mapping[str, Set["Resource"]],
    resolvers: Dict[str, Resolver],
    custom: bool,
    transform_using_type_metadata: bool = False,
    keep_unknowns: bool = False,
):
    # Produce a combined set of property states, starting with inputs and then applying
    # outputs.  If the same property exists in the inputs and outputs states, the output wins.
    all_properties = {}
    # Get the resource's output types, so we can convert dicts from the engine into actual
    # instantiated output types or primitive types into enums as needed.
    resource_cls = type(res)
    types = _types.resource_types(resource_cls)
    translate, translate_to_pass = (
        res.translate_output_property,
        res.translate_output_property,
    )
    if transform_using_type_metadata:
        pulumi_to_py_names = _types.resource_pulumi_to_py_names(resource_cls)
        # pylint: disable=C3001
        translate = lambda prop: pulumi_to_py_names.get(prop) or prop
        # pylint: disable=C3001
        translate_to_pass = lambda prop: prop

    for key, value in deserialize_properties(outputs, keep_unknowns).items():
        # Outputs coming from the provider are NOT translated. Do so here.
        translated_key = translate(key)

        translated_value = translate_output_properties(
            value,
            translate_to_pass,
            types.get(key),
            transform_using_type_metadata,
            path=_Path(translated_key, resource=f"{res._name}"),
        )

        if settings.excessive_debug_output:
            log.debug(f"incoming output property translated: {key} -> {translated_key}")
            log.debug(
                f"incoming output value translated: {value} -> {translated_value}"
            )

        all_properties[translated_key] = translated_value

    translated_deps = {}
    for key, property_deps in deps.items():
        translated_deps[translate(key)] = property_deps

    if not settings.is_dry_run() or settings.is_legacy_apply_enabled():
        for key, value in list(serialized_props.items()):
            translated_key = translate(key)
            if translated_key not in all_properties:
                # input prop the engine didn't give us a final value for.
                # Just use the value passed into the resource by the user.
                # Set `return_none_on_dict_type_mismatch` to return `None` rather than raising an error when the value
                # is a dict and the type doesn't match (which is what would happen if the value didn't exist as an
                # input prop). This allows `pulumi up` to work without erroring when there is an input and output prop
                # with the same name but different types.
                all_properties[translated_key] = translate_output_properties(
                    deserialize_property(value),
                    translate_to_pass,
                    types.get(key),
                    transform_using_type_metadata,
                    path=_Path(translated_key, resource=f"{res._name}"),
                    return_none_on_dict_type_mismatch=True,
                )

    resolve_properties(resolvers, all_properties, translated_deps, custom)


def resolve_properties(
    resolvers: Dict[str, Resolver],
    all_properties: Dict[str, Any],
    deps: Mapping[str, Set["Resource"]],
    custom: bool,
):
    for key, value in all_properties.items():
        # Skip "id" and "urn", since we handle those specially.
        # Only skip ID if this is a custom resource, meaning non-component resource.
        # For component resources, using ID as output property is allowed.
        if key == "urn" or (key == "id" and custom):
            continue

        # Otherwise, unmarshal the value, and store it on the resource object.
        if settings.excessive_debug_output:
            log.debug(f"looking for resolver using translated name {key}")
        resolve = resolvers.get(key)
        if resolve is None:
            # engine returned a property that was not in our initial property-map.  This can happen
            # for outputs that were registered through direct calls to 'registerOutputs'. We do
            # *not* want to do anything with these returned properties.  First, the component
            # resources that were calling 'registerOutputs' will have already assigned these fields
            # directly on them themselves.  Second, if we were to try to assign here we would have
            # an incredibly bad race condition for two reasons:
            #
            #  1. This call to 'resolveProperties' happens asynchronously at some point far after
            #     the resource was constructed.  So the user will have been able to observe the
            #     initial value up until we get to this point.
            #
            #  2. The component resource will have often assigned a value of some arbitrary type
            #     (say, a 'string').  If we overwrite this with an `Output<string>` we'll be changing
            #     the type at some non-deterministic point in the future.
            continue

        # If this value is a secret, unwrap its inner value.
        is_secret = is_rpc_secret(value)
        value = unwrap_rpc_secret(value)

        # If either we are performing a real deployment, or this is a stable property value, we
        # can propagate its final value.  Otherwise, it must be undefined, since we don't know
        # if it's final.
        if not settings.is_dry_run():
            # normal 'pulumi up'.  resolve the output with the value we got back
            # from the engine.  That output can always run its .apply calls.
            resolve(value, True, is_secret, deps.get(key), None)
        else:
            # We're previewing. If the engine was able to give us a reasonable value back,
            # then use it. Otherwise, inform the Output that the value isn't known.
            resolve(value, value is not None, is_secret, deps.get(key), None)

    # `allProps` may not have contained a value for every resolver: for example, optional outputs may not be present.
    # We will resolve all of these values as `None`, and will mark the value as known if we are not running a
    # preview.
    for key, resolve in resolvers.items():
        if key not in all_properties:
            resolve(None, not settings.is_dry_run(), False, deps.get(key), None)


def resolve_outputs_due_to_exception(resolvers: Dict[str, Resolver], exn: Exception):
    """
    Resolves all outputs with resolvers exceptionally, using the given exception as the reason why the resolver has
    failed to resolve.

    :param resolvers: Resolvers associated with a resource's outputs.
    :param exn: The exception that occurred when trying (and failing) to create this resource.
    """
    for key, resolve in resolvers.items():
        if settings.excessive_debug_output:
            log.debug(f"sending exception to resolver for {key}")
        resolve(None, False, False, None, exn)


def same_version(a: Optional[Version], b: Optional[Version]) -> bool:
    # We treat None as a wildcard, so it always equals every other version.
    return a is None or b is None or a == b


def check_version(want: Optional[Version], have: Optional[Version]) -> bool:
    if want is None or have is None:
        return True
    return (
        have.major == want.major
        and have.minor >= want.minor
        and have.patch >= want.patch
    )


class ResourcePackage(ABC):
    @abstractmethod
    def version(self) -> Optional[Version]:
        pass

    @abstractmethod
    def construct_provider(self, name: str, typ: str, urn: str) -> "ProviderResource":
        pass


_RESOURCE_PACKAGES: Dict[str, List[ResourcePackage]] = {}


def register_resource_package(pkg: str, package: ResourcePackage):
    resource_packages = _RESOURCE_PACKAGES.get(pkg, None)
    if resource_packages is not None:
        for existing in resource_packages:
            if same_version(existing.version(), package.version()):
                raise ValueError(
                    f"Cannot re-register package {pkg}@{package.version()}. Previous registration was {existing}, new registration was {package}."
                )
    else:
        resource_packages = []
        _RESOURCE_PACKAGES[pkg] = resource_packages

    if settings.excessive_debug_output:
        log.debug(f"registering package {pkg}@{package.version()}")
    resource_packages.append(package)


def get_resource_package(pkg: str, version: str) -> Optional[ResourcePackage]:
    ver = None if version == "" else Version.parse(version)

    best_package = None
    for package in _RESOURCE_PACKAGES.get(pkg, []):
        if not check_version(ver, package.version()):
            continue
        if best_package is None or package.version() > best_package.version():
            best_package = package

    return best_package


class ResourceModule(ABC):
    @abstractmethod
    def version(self) -> Optional[Version]:
        pass

    @abstractmethod
    def construct(self, name: str, typ: str, urn: str) -> "Resource":
        pass


_RESOURCE_MODULES: Dict[str, List[ResourceModule]] = {}


def _module_key(pkg: str, mod: str) -> str:
    return f"{pkg}:{mod}"


def register_resource_module(pkg: str, mod: str, module: ResourceModule):
    key = _module_key(pkg, mod)

    resource_modules = _RESOURCE_MODULES.get(key, None)
    if resource_modules is not None:
        for existing in resource_modules:
            if same_version(existing.version(), module.version()):
                raise ValueError(
                    f"Cannot re-register module {key}@{module.version()}. Previous registration was {existing}, new registration was {module}."
                )
    else:
        resource_modules = []
        _RESOURCE_MODULES[key] = resource_modules

    if settings.excessive_debug_output:
        log.debug(f"registering module {key}@{module.version()}")
    resource_modules.append(module)


def get_resource_module(pkg: str, mod: str, version: str) -> Optional[ResourceModule]:
    key = _module_key(pkg, mod)
    ver = None if version == "" else Version.parse(version)

    best_module = None
    for module in _RESOURCE_MODULES.get(key, []):
        if not check_version(ver, module.version()):
            continue
        if best_module is None or module.version() > best_module.version():
            best_module = module

    return best_module
