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
import asyncio
from collections import abc
import functools
import inspect
from abc import ABC, abstractmethod
from typing import List, Any, Callable, Dict, Mapping, Optional, Sequence, Set, TYPE_CHECKING, cast
from enum import Enum

from google.protobuf import struct_pb2
from semver import VersionInfo as Version  # type:ignore
import six
from . import known_types, settings
from .. import log
from .. import _types

if TYPE_CHECKING:
    from ..output import Inputs, Input, Output
    from ..resource import Resource, ProviderResource
    from ..asset import FileAsset, RemoteAsset, StringAsset, FileArchive, RemoteArchive, AssetArchive

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

_special_sig_key = "4dabf18193072939515e22adb298388d"
"""_special_sig_key is sometimes used to encode type identity inside of a map. See pkg/resource/properties.go."""

_special_asset_sig = "c44067f5952c0a294b673a41bacd8c17"
"""special_asset_sig is a randomly assigned hash used to identify assets in maps. See pkg/resource/asset.go."""

_special_archive_sig = "0def7320c3a5731c473e5ecbe6d01bc7"
"""special_archive_sig is a randomly assigned hash used to identify assets in maps. See pkg/resource/asset.go."""

_special_secret_sig = "1b47061264138c4ac30d75fd1eb44270"
"""special_secret_sig is a randomly assigned hash used to identify secrets in maps. See pkg/resource/properties.go"""

_special_resource_sig = "5cf8f73096256a8f31e491e813e4eb8e"
"""special_resource_sig is a randomly assigned hash used to identify resources in maps. See pkg/resource/properties.go"""

_INT_OR_FLOAT = six.integer_types + (float,)


def isLegalProtobufValue(value: Any) -> bool:
    """
    Returns True if the given value is a legal Protobuf value as per the source at
    https://github.com/protocolbuffers/protobuf/blob/master/python/google/protobuf/internal/well_known_types.py#L714-L732
    """
    return value is None or isinstance(value, (bool, six.string_types, _INT_OR_FLOAT, dict, list))


async def serialize_properties(inputs: 'Inputs',
                               property_deps: Dict[str, List['Resource']],
                               input_transformer: Optional[Callable[[str], str]] = None) -> struct_pb2.Struct:
    """
    Serializes an arbitrary Input bag into a Protobuf structure, keeping track of the list
    of dependent resources in the `deps` list. Serializing properties is inherently async
    because it awaits any futures that are contained transitively within the input bag.
    """
    struct = struct_pb2.Struct()
    for k, v in inputs.items():
        deps: List['Resource'] = []
        result = await serialize_property(v, deps, input_transformer)
        # We treat properties that serialize to None as if they don't exist.
        if result is not None:
            # While serializing to a pb struct, we must "translate" all key names to be what the
            # engine is going to expect. Resources provide the "transform" function for doing this.
            translated_name = k
            if input_transformer is not None:
                translated_name = input_transformer(k)
                log.debug(f"top-level input property translated: {k} -> {translated_name}")
            # pylint: disable=unsupported-assignment-operation
            struct[translated_name] = result
            property_deps[translated_name] = deps

    return struct


# pylint: disable=too-many-return-statements, too-many-branches
async def serialize_property(value: 'Input[Any]',
                             deps: List['Resource'],
                             input_transformer: Optional[Callable[[str], str]] = None) -> Any:
    """
    Serializes a single Input into a form suitable for remoting to the engine, awaiting
    any futures required to do so.
    """
    # Exclude some built-in types that are instances of Sequence that we don't want to treat as sequences here.
    # From: https://github.com/python/cpython/blob/master/Lib/_collections_abc.py
    if isinstance(value, abc.Sequence) and not isinstance(value, (tuple, str, range, memoryview, bytes, bytearray)):
        props = []
        for elem in value:
            props.append(await serialize_property(elem, deps, input_transformer))

        return props

    if known_types.is_unknown(value):
        return UNKNOWN

    if known_types.is_resource(value):
        resource = cast('Resource', value)

        is_custom = known_types.is_custom_resource(value)
        resource_id = cast('CustomResource', value).id if is_custom else None

        # If we're retaining resources, serialize the resource as a reference.
        if await settings.monitor_supports_resource_references():
            res = {
                _special_sig_key: _special_resource_sig,
                "urn": await serialize_property(resource.urn, deps, input_transformer)
            }
            if is_custom:
                res["id"] = await serialize_property(resource_id, deps, input_transformer)
            return res

        # Otherwise, serialize the resource as either its ID (for custom resources) or its URN (for component resources)
        return await serialize_property(resource_id if is_custom else resource.urn, deps, input_transformer)

    if known_types.is_asset(value):
        # Serializing an asset requires the use of a magical signature key, since otherwise it would
        # look like any old weakly typed object/map when received by the other side of the RPC
        # boundary.
        obj = {
            _special_sig_key: _special_asset_sig
        }

        if hasattr(value, "path"):
            file_asset = cast('FileAsset', value)
            obj["path"] = await serialize_property(file_asset.path, deps, input_transformer)
        elif hasattr(value, "text"):
            str_asset = cast('StringAsset', value)
            obj["text"] = await serialize_property(str_asset.text, deps, input_transformer)
        elif hasattr(value, "uri"):
            remote_asset = cast('RemoteAsset', value)
            obj["uri"] = await serialize_property(remote_asset.uri, deps, input_transformer)
        else:
            raise AssertionError(f"unknown asset type: {value!r}")

        return obj

    if known_types.is_archive(value):
        # Serializing an archive requires the use of a magical signature key, since otherwise it
        # would look like any old weakly typed object/map when received by the other side of the RPC
        # boundary.
        obj = {
            _special_sig_key: _special_archive_sig
        }

        if hasattr(value, "assets"):
            asset_archive = cast('AssetArchive', value)
            obj["assets"] = await serialize_property(asset_archive.assets, deps, input_transformer)
        elif hasattr(value, "path"):
            file_archive = cast('FileArchive', value)
            obj["path"] = await serialize_property(file_archive.path, deps, input_transformer)
        elif hasattr(value, "uri"):
            remote_archive = cast('RemoteArchive', value)
            obj["uri"] = await serialize_property(remote_archive.uri, deps, input_transformer)
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
        awaitable = cast('Any', value)
        future_return = await asyncio.ensure_future(awaitable)
        return await serialize_property(future_return, deps, input_transformer)

    if known_types.is_output(value):
        output = cast('Output', value)
        value_resources = await output.resources()
        deps.extend(value_resources)

        # When serializing an Output, we will either serialize it as its resolved value or the
        # "unknown value" sentinel. We will do the former for all outputs created directly by user
        # code (such outputs always resolve isKnown to true) and for any resource outputs that were
        # resolved with known values.
        is_known = await output._is_known
        is_secret = await output._is_secret
        value = await serialize_property(output.future(), deps, input_transformer)
        if not is_known:
            return UNKNOWN
        if is_secret and await settings.monitor_supports_secrets():
            # Serializing an output with a secret value requires the use of a magical signature key,
            # which the engine detects.
            return {
                _special_sig_key: _special_secret_sig,
                "value": value
            }
        return value

    transform_keys = True

    # If value is an input type, convert it to a dict, and set transform_keys to False to prevent
    # transforming the keys of the resulting dict as the keys should already be the final names.
    value_cls = type(value)
    if _types.is_input_type(value_cls):
        value = _types.input_type_to_dict(value)
        transform_keys = False

    if isinstance(value, abc.Mapping):
        obj = {}
        for k, v in value.items():
            transformed_key = k
            if transform_keys and input_transformer is not None:
                transformed_key = input_transformer(k)
                log.debug(f"transforming input property: {k} -> {transformed_key}")
            obj[transformed_key] = await serialize_property(v, deps, input_transformer)

        return obj

    # Ensure that we have a value that Protobuf understands.
    if not isLegalProtobufValue(value):
        raise ValueError(f"unexpected input of type {type(value).__name__}")

    return value


# pylint: disable=too-many-return-statements
def deserialize_properties(props_struct: struct_pb2.Struct, keep_unknowns: Optional[bool] = None) -> Any:
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
        from .. import FileAsset, StringAsset, RemoteAsset, AssetArchive, FileArchive, RemoteArchive  # pylint: disable=import-outside-toplevel
        if props_struct[_special_sig_key] == _special_asset_sig:
            # This is an asset. Re-hydrate this object into an Asset.
            if "path" in props_struct:
                return FileAsset(props_struct["path"])
            if "text" in props_struct:
                return StringAsset(props_struct["text"])
            if "uri" in props_struct:
                return RemoteAsset(props_struct["uri"])
            raise AssertionError("Invalid asset encountered when unmarshalling resource property")
        if props_struct[_special_sig_key] == _special_archive_sig:
            # This is an archive. Re-hydrate this object into an Archive.
            if "assets" in props_struct:
                return AssetArchive(deserialize_property(props_struct["assets"]))
            if "path" in props_struct:
                return FileArchive(props_struct["path"])
            if "uri" in props_struct:
                return RemoteArchive(props_struct["uri"])
            raise AssertionError("Invalid archive encountered when unmarshalling resource property")
        if props_struct[_special_sig_key] == _special_secret_sig:
            return wrap_rpc_secret(deserialize_property(props_struct["value"]))
        if props_struct[_special_sig_key] == _special_resource_sig:
            return deserialize_resource(props_struct, keep_unknowns)
        raise AssertionError("Unrecognized signature when unmarshalling resource property")

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
        if k.startswith("__") and k != "__provider":
            continue

        value = deserialize_property(v, keep_unknowns)
        # We treat values that deserialize to "None" as if they don't exist.
        if value is not None:
            output[k] = value

    return output


def deserialize_resource(ref_struct: struct_pb2.Struct, keep_unknowns: Optional[bool] = None) -> 'Resource':
    urn = ref_struct["urn"]
    version = ref_struct["packageVersion"] if "packageVersion" in ref_struct else ""

    urn_parts = urn.split("::")
    urn_name = urn_parts[3]
    qualified_type = urn_parts[2]
    typ = qualified_type.split("$")[-1]

    typ_parts = typ.split(":")
    pkg_name = typ_parts[0]
    mod_name = typ_parts[1] if len(typ_parts) > 1 else ""
    typ_name = typ_parts[2] if len(typ_parts) > 2 else ""

    is_provider = pkg_name == "pulumi" and mod_name == "providers"
    if is_provider:
        resource_package = get_resource_package(typ_name, version)
        if resource_package is not None:
            return cast('Resource', resource_package.construct_provider(urn_name, typ, urn))
    else:
        resource_module = get_resource_module(pkg_name, mod_name, version)
        if resource_module is not None:
            return cast('Resource', resource_module.construct(urn_name, typ, urn))

    # If we've made it here, deserialize the reference as either a URN or an ID (if present).
    if "id" in ref_struct:
        id = ref_struct["id"]
        return deserialize_property(UNKNOWN if id == "" else id, keep_unknowns)

    return urn


def is_rpc_secret(value: Any) -> bool:
    """
    Returns if a given python value is actually a wrapped secret.
    """
    return isinstance(value, dict) and _special_sig_key in value and value[_special_sig_key] == _special_secret_sig


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
        values = [deserialize_property(v, keep_unknowns) for v in value] # type: ignore
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


Resolver = Callable[[Any, bool, bool, Optional[Set['Resource']], Optional[Exception]], None]
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


def transfer_properties(res: 'Resource', props: 'Inputs') -> Dict[str, Resolver]:
    from .. import Output  # pylint: disable=import-outside-toplevel
    resolvers: Dict[str, Resolver] = {}
    for name in props.keys():
        if name in ["id", "urn"]:
            # these properties are handled specially elsewhere.
            continue

        resolve_value: 'asyncio.Future' = asyncio.Future()
        resolve_is_known: 'asyncio.Future' = asyncio.Future()
        resolve_is_secret: 'asyncio.Future' = asyncio.Future()
        resolve_deps: 'asyncio.Future' = asyncio.Future()

        def do_resolve(r: 'Resource',
                       value_fut: 'asyncio.Future',
                       known_fut: 'asyncio.Future[bool]',
                       secret_fut: 'asyncio.Future[bool]',
                       deps_fut: 'asyncio.Future[Set[Resource]]',
                       value: Any,
                       is_known: bool,
                       is_secret: bool,
                       deps: Set['Resource'],
                       failed: Optional[Exception]):

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
        # using res.translate_output_property and then use *that* name to index into the resolvers table.
        log.debug(f"adding resolver {name}")
        resolvers[name] = functools.partial(do_resolve, res, resolve_value, resolve_is_known, resolve_is_secret, resolve_deps)
        res.__dict__[name] = Output(resolve_deps, resolve_value, resolve_is_known, resolve_is_secret)

    return resolvers


def translate_output_properties(output: Any,
                                output_transformer: Callable[[str], str],
                                typ: Optional[type] = None) -> Any:
    """
    Recursively rewrite keys of objects returned by the engine to conform with a naming
    convention specified by `output_transformer`.

    Additionally, perform any type conversions as necessary, based on the optional `typ` parameter.

    If output is a `dict`, every key is translated using `translate_output_property` while every value is transformed
    by recursing.

    If output is a `list`, every value is recursively transformed.

    If output is a `dict` and `typ` is an output type, instantiate the output type,
    passing the values in the dict to the output type's __init__() method.

    If output is a `float` and `typ` is `int`, the value is cast to `int`.

    If output is in [`str`, `int`, `float`] and `typ` is an enum type, instantiate the enum type.

    Otherwise, if output is a primitive (i.e. not a dict or list), the value is returned without modification.

    :param Optional[type] typ: The output's target type.
    """

    # If it's a secret, unwrap the value so the output is in alignment with the expected type, call
    # translate_output_properties with the unwrapped value, and then rewrap the result as a secret.
    if is_rpc_secret(output):
        unwrapped = unwrap_rpc_secret(output)
        result = translate_output_properties(unwrapped, output_transformer, typ)
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

        if typ and _types.is_output_type(typ):
            # If typ is an output type, get its types, so we can pass
            # the type along for each property.
            types = _types.output_type_types(typ)
            get_type = lambda k: types.get(k)  # pylint: disable=unnecessary-lambda
        elif typ:
            # If typ is a dict, get the type for its values, to pass
            # along for each key.
            origin = _types.get_origin(typ)
            if typ is dict or origin in {dict, Dict, Mapping, abc.Mapping}:
                args = _types.get_args(typ)
                if len(args) == 2 and args[0] is str:
                    get_type = lambda k: args[1]
            else:
                raise AssertionError(f"Unexpected type; expected 'dict' got '{typ}'")

        # If typ is an output type, instantiate it. We do not translate the top-level keys,
        # as the output type will take care of doing that if it has a _translate_property()
        # method.
        if typ and _types.is_output_type(typ):
            translated_values = {
                k: translate_output_properties(v, output_transformer, get_type(k))
                for k, v in output.items()
            }
            return _types.output_type_from_dict(typ, translated_values)

        # Otherwise, return the fully translated dict.
        return {
            output_transformer(k):
                translate_output_properties(v, output_transformer, get_type(k))
            for k, v in output.items()
        }

    if isinstance(output, list):
        element_type: Optional[type] = None
        if typ:
            # If typ is a list, get the type for its values, to pass
            # along for each item.
            origin = _types.get_origin(typ)
            if typ is list or origin in {list, List, Sequence, abc.Sequence}:
                args = _types.get_args(typ)
                if len(args) == 1:
                    element_type = args[0]
            else:
                raise AssertionError(f"Unexpected type. Expected 'list' got '{typ}'")
        return [translate_output_properties(v, output_transformer, element_type) for v in output]

    if typ and isinstance(output, (int, float, str)) and inspect.isclass(typ) and issubclass(typ, Enum):
        return typ(output)

    if isinstance(output, float) and typ is int:
        return int(output)

    return output


def contains_unknowns(val: Any) -> bool:
    def impl(val: Any, stack: List[Any]) -> bool:
        if known_types.is_unknown(val):
            return True

        if not any([x is val for x in stack]):
            stack.append(val)
            if isinstance(val, dict):
                return any([impl(val[k], stack) for k in val])
            if isinstance(val, list):
                return any([impl(x, stack) for x in val])
        return False

    return impl(val, [])


def resolve_outputs(res: 'Resource',
                    serialized_props: struct_pb2.Struct,
                    outputs: struct_pb2.Struct,
                    deps: Mapping[str, Set['Resource']],
                    resolvers: Dict[str, Resolver]):

    # Produce a combined set of property states, starting with inputs and then applying
    # outputs.  If the same property exists in the inputs and outputs states, the output wins.
    all_properties = {}
    # Get the resource's output types, so we can convert dicts from the engine into actual
    # instantiated output types or primitive types into enums as needed.
    types = _types.resource_types(type(res))
    for key, value in deserialize_properties(outputs).items():
        # Outputs coming from the provider are NOT translated. Do so here.
        translated_key = res.translate_output_property(key)
        translated_value = translate_output_properties(value, res.translate_output_property, types.get(key))
        log.debug(f"incoming output property translated: {key} -> {translated_key}")
        log.debug(f"incoming output value translated: {value} -> {translated_value}")
        all_properties[translated_key] = translated_value

    if not settings.is_dry_run() or settings.is_legacy_apply_enabled():
        for key, value in list(serialized_props.items()):
            translated_key = res.translate_output_property(key)
            if translated_key not in all_properties:
                # input prop the engine didn't give us a final value for.Just use the value passed into the resource by
                # the user.
                all_properties[translated_key] = translate_output_properties(deserialize_property(value), res.translate_output_property, types.get(key))

    resolve_properties(resolvers, all_properties, deps)


def resolve_properties(resolvers: Dict[str, Resolver], all_properties: Dict[str, Any], deps: Mapping[str, Set['Resource']]):
    for key, value in all_properties.items():
        # Skip "id" and "urn", since we handle those specially.
        if key in ["id", "urn"]:
            continue

        # Otherwise, unmarshal the value, and store it on the resource object.
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
        log.debug(f"sending exception to resolver for {key}")
        resolve(None, False, False, None, exn)


def same_version(a: Optional[Version], b: Optional[Version]) -> bool:
    # We treat None as a wildcard, so it always equals every other version.
    return a is None or b is None or a == b


def check_version(want: Optional[Version], have: Optional[Version]) -> bool:
    if want is None or have is None:
        return True
    return have.major == want.major() and have.minor() >= want.minor() and have.patch() >= want.patch()


class ResourcePackage(ABC):
    @abstractmethod
    def version(self) -> Optional[Version]:
        pass

    @abstractmethod
    def construct_provider(self, name: str, typ: str, urn: str) -> 'ProviderResource':
        pass


_RESOURCE_PACKAGES: Dict[str, List[ResourcePackage]] = dict()


def register_resource_package(pkg: str, package: ResourcePackage):
    resource_packages = _RESOURCE_PACKAGES.get(pkg, None)
    if resource_packages is not None:
        for existing in resource_packages:
            if same_version(existing.version(), package.version()):
                raise ValueError(f"Cannot re-register package {pkg}@{package.version()}. Previous registration was {existing}, new registration was {package}.")
    else:
        resource_packages = []
        _RESOURCE_PACKAGES[pkg] = resource_packages

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
    def construct(self, name: str, typ: str, urn: str) -> 'Resource':
        pass


_RESOURCE_MODULES: Dict[str, List[ResourceModule]] = dict()


def _module_key(pkg: str, mod: str) -> str:
    return f"{pkg}:{mod}"


def register_resource_module(pkg: str, mod: str, module: ResourceModule):
    key = _module_key(pkg, mod)

    resource_modules = _RESOURCE_MODULES.get(key, None)
    if resource_modules is not None:
        for existing in resource_modules:
            if same_version(existing.version(), module.version()):
                raise ValueError(f"Cannot re-register module {key}@{module.version()}. Previous registration was {existing}, new registration was {module}.")
    else:
        resource_modules = []
        _RESOURCE_MODULES[key] = resource_modules

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
