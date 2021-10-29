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
import asyncio
import os
import traceback

from typing import Optional, Any, Callable, List, NamedTuple, Dict, Set, Tuple, Union, TYPE_CHECKING, cast, Mapping, Sequence, Iterable
from google.protobuf import struct_pb2
import grpc

from . import rpc, settings, known_types
from .. import log
from ..runtime.proto import provider_pb2, resource_pb2
from .rpc_manager import RPC_MANAGER
from .settings import handle_grpc_error
from .resource_cycle_breaker import declare_dependency
from ..output import Output
from .. import _types
from .. import urn as urn_util


if TYPE_CHECKING:
    from .. import Resource, ComponentResource, CustomResource, Inputs, ProviderResource
    from ..resource import ResourceOptions


class ResourceResolverOperations(NamedTuple):
    """
    The set of properties resulting from a successful call to prepare_resource.
    """

    parent_urn: Optional[str]
    """
    This resource's parent URN.
    """

    serialized_props: struct_pb2.Struct
    """
    This resource's input properties, serialized into protobuf structures.
    """

    dependencies: Set[str]
    """
    The set of URNs, corresponding to the resources that this resource depends on.
    """

    provider_ref: Optional[str]
    """
    An optional reference to a provider that should be used for this resource's CRUD operations.
    """

    provider_refs: Dict[str, Optional[str]]
    """
    An optional dict of references to providers that should be used for this resource's CRUD operations.
    """

    property_dependencies: Dict[str, List[Optional[str]]]
    """
    A map from property name to the URNs of the resources the property depends on.
    """

    aliases: List[Optional[str]]
    """
    A list of aliases applied to this resource.
    """


# Prepares for an RPC that will manufacture a resource, and hence deals with input and output properties.
# pylint: disable=too-many-locals
async def prepare_resource(res: 'Resource',
                           ty: str,
                           custom: bool,
                           remote: bool,
                           props: 'Inputs',
                           opts: Optional['ResourceOptions'],
                           typ: Optional[type] = None) -> ResourceResolverOperations:

    # Before we can proceed, all our dependencies must be finished.
    explicit_urn_dependencies: Set[str] = set()
    if opts is not None and opts.depends_on is not None:
        explicit_urn_dependencies = await _resolve_depends_on_urns(opts, from_resource=res)

    # Serialize out all our props to their final values.  In doing so, we'll also collect all
    # the Resources pointed to by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
    property_dependencies_resources: Dict[str, List['Resource']] = {}

    # If we have type information, we'll use it for translations rather than the resource's translate_input_property.
    translate: Optional[Callable[[str], str]] = res.translate_input_property
    if typ is not None:
        translate = None

    # To initially scope the use of this new feature, we only keep output values when
    # remote is true (for multi-lang components).
    serialized_props = await rpc.serialize_properties(props, property_dependencies_resources, translate, typ,
        keep_output_values=remote)

    # Wait for our parent to resolve
    parent_urn: Optional[str] = ""
    if opts is not None and opts.parent is not None:
        parent_urn = await opts.parent.urn.future()
    # TODO(sean) is it necessary to check the type here?
    elif ty != "pulumi:pulumi:Stack":
        # If no parent was provided, parent to the root resource.
        parent = settings.get_root_resource()
        if parent is not None:
            parent_urn = await parent.urn.future()

    # Construct the provider reference, if we were given a provider to use.
    provider_ref = None
    if custom and opts is not None and opts.provider is not None:
        provider = opts.provider

        # If we were given a provider, wait for it to resolve and construct a provider reference from it.
        # A provider reference is a well-known string (two ::-separated values) that the engine interprets.
        provider_urn = await provider.urn.future()
        provider_id = await provider.id.future() or rpc.UNKNOWN
        provider_ref = f"{provider_urn}::{provider_id}"

    # For remote resources, merge any provider opts into a single dict, and then create a new dict with all of the
    # resolved provider refs.
    provider_refs: Dict[str, Optional[str]] = {}
    if remote and opts is not None:
        providers = convert_providers(opts.provider, opts.providers)
        for name, provider in providers.items():
            # If we were given providers, wait for them to resolve and construct provider references from them.
            # A provider reference is a well-known string (two ::-separated values) that the engine interprets.
            urn = await provider.urn.future()
            id_ = await provider.id.future() or rpc.UNKNOWN
            ref = f"{urn}::{id_}"
            provider_refs[name] = ref

    dependencies: Set[str] = set(explicit_urn_dependencies)
    property_dependencies: Dict[str, List[Optional[str]]] = {}
    for key, deps in property_dependencies_resources.items():
        urns = await _expand_dependencies(deps, from_resource=res)
        dependencies |= urns
        property_dependencies[key] = list(urns)

    # Wait for all aliases. Note that we use `res._aliases` instead of `opts.aliases` as the
    # former has been processed in the Resource constructor prior to calling
    # `register_resource` - both adding new inherited aliases and simplifying aliases down
    # to URNs.
    aliases: List[Optional[str]] = []
    for alias in res._aliases:
        alias_val = await Output.from_input(alias).future()
        if not alias_val in aliases:
            aliases.append(alias_val)

    return ResourceResolverOperations(
        parent_urn,
        serialized_props,
        dependencies,
        provider_ref,
        provider_refs,
        property_dependencies,
        aliases,
    )


def resource_output(res: 'Resource') -> Tuple[Callable[[Any, bool, bool, Optional[Exception]], None], 'Output']:

    value_future: asyncio.Future[Any] = asyncio.Future()
    known_future: asyncio.Future[bool] = asyncio.Future()
    secret_future: asyncio.Future[bool] = asyncio.Future()

    def resolve(value: Any, known: bool, secret: bool, exn: Optional[Exception]):
        if exn is not None:
            value_future.set_exception(exn)
            known_future.set_exception(exn)
            secret_future.set_exception(exn)
        else:
            value_future.set_result(value)
            known_future.set_result(known)
            secret_future.set_result(secret)

    return resolve, Output({res}, value_future, known_future, secret_future)


def get_resource(res: 'Resource',
                 props: 'Inputs',
                 custom: bool,
                 urn: str,
                 typ: Optional[type] = None) -> None:
    log.debug(f"getting resource: urn={urn}")

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Extract the resource type from the URN.
    urn_parts = urn_util._parse_urn(urn)
    ty = urn_parts.typ

    # Initialize the URN property on the resource.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # If this is a custom resource, initialize its ID property.
    resolve_id: Optional[Callable[[Any, bool, bool, Optional[Exception]], None]] = None
    if custom:
        (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Like the other resource functions, "transfer" all input properties onto unresolved futures on res.
    resolvers = rpc.transfer_properties(res, props)

    async def do_get():
        try:
            resolver = await prepare_resource(res, ty, custom, False, props, None, typ)

            monitor = settings.get_monitor()
            inputs = await rpc.serialize_properties({"urn": urn}, {})
            req = provider_pb2.InvokeRequest(tok="pulumi:pulumi:getResource", args=inputs, provider="", version="")

            def do_invoke():
                try:
                    return monitor.Invoke(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_invoke)

            # If the invoke failed, raise an error.
            if resp.failures:
                raise Exception(f"getResource failed: {resp.failures[0].reason} ({resp.failures[0].property})")

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}")
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            if resolve_id is not None:
                resolve_id(None, True, False, exn)
            raise

        # Otherwise, grab the URN, ID, and output properties and resolve all of them.
        resp = getattr(resp, 'return')

        log.debug(f"getResource completed successfully: ty={ty}, urn={resp['urn']}")
        resolve_urn(resp["urn"], True, False, None)
        if resolve_id:
            # The ID is known if (and only if) it is a non-empty string. If it's either None or an
            # empty string, we should treat it as unknown. TFBridge in particular is known to send
            # the empty string as an ID when doing a preview.
            is_known = bool(resp["id"])
            resolve_id(resp["id"], is_known, False, None)

        rpc.resolve_outputs(res, resolver.serialized_props, resp["state"], {}, resolvers, transform_using_type_metadata)

    asyncio.ensure_future(RPC_MANAGER.do_rpc("get resource", do_get)())


def _translate_ignore_changes(res: 'Resource',
                              typ: Optional[type],
                              ignore_changes: Optional[List[str]]) -> Optional[List[str]]:
    if ignore_changes is not None:
        if typ is not None:
            # If `typ` is specified, use its type/name metadata for translation.
            input_names = _types.input_type_py_to_pulumi_names(typ)
            ignore_changes = list(map(lambda k: input_names.get(k) or k, ignore_changes))
        elif res.translate_input_property is not None:
            ignore_changes = list(map(res.translate_input_property, ignore_changes))
    return ignore_changes


def _translate_additional_secret_outputs(res: 'Resource',
                                         typ: Optional[type],
                                         additional_secret_outputs: Optional[List[str]]) -> Optional[List[str]]:
    if additional_secret_outputs is not None:
        if typ is not None:
            # If a `typ` is specified, we've opt-ed in to doing translations using type/name metadata rather
            # than using the resource's tranlate_input_property. Use the resource's metadata to translate.
            output_names = _types.resource_py_to_pulumi_names(type(res))
            additional_secret_outputs = list(map(lambda k: output_names.get(k) or k, additional_secret_outputs))
        elif res.translate_input_property is not None:
            # Note that while `additional_secret_outputs` lists property names that are outputs, we
            # call `translate_input_property` because it is the method that converts from the
            # language projection name to the provider name, which is what we want.
            additional_secret_outputs = list(map(res.translate_input_property, additional_secret_outputs))
    return additional_secret_outputs


def _translate_replace_on_changes(res: 'Resource',
                                  typ: Optional[type],
                                  replace_on_changes: Optional[List[str]]) -> Optional[List[str]]:
    if replace_on_changes is not None:
        if typ is not None:
            # If `typ` is specified, use its type/name metadata for translation.
            input_names = _types.input_type_py_to_pulumi_names(typ)
            replace_on_changes = list(map(lambda k: input_names.get(k) or k, replace_on_changes))
        elif res.translate_input_property is not None:
            replace_on_changes = list(map(res.translate_input_property, replace_on_changes))
    return replace_on_changes


def read_resource(res: 'CustomResource',
                  ty: str,
                  name: str,
                  props: 'Inputs',
                  opts: 'ResourceOptions',
                  typ: Optional[type] = None) -> None:
    if opts.id is None:
        raise Exception(
            "Cannot read resource whose options are lacking an ID value")

    log.debug(f"reading resource: ty={ty}, name={name}, id={opts.id}")
    monitor = settings.get_monitor()

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Prepare the resource, similar to a RegisterResource. Reads are deliberately similar to RegisterResource except
    # that we are populating the Resource object with properties associated with an already-live resource.
    #
    # Same as below, we initialize the URN property on the resource, which will always be resolved.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # Furthermore, since resources being Read must always be custom resources (enforced in the
    # Resource constructor), we'll need to set up the ID field which will be populated at the end of
    # the ReadResource call.
    #
    # Note that we technically already have the ID (opts.id), but it's more consistent with the rest
    # of the model to resolve it asynchronously along with all of the other resources.

    (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Like below, "transfer" all input properties onto unresolved futures on res.
    resolvers = rpc.transfer_properties(res, props)

    async def do_read():
        try:
            resolver = await prepare_resource(res, ty, True, False, props, opts, typ)

            # Resolve the ID that we were given. Note that we are explicitly discarding the list of
            # dependencies returned to us from "serialize_property" (the second argument). This is
            # because a "read" resource does not actually have any dependencies at all in the cloud
            # provider sense, because a read resource already exists. We do not need to track this
            # dependency.
            resolved_id = await rpc.serialize_property(opts.id, [])
            log.debug(f"read prepared: ty={ty}, name={name}, id={opts.id}")

            # These inputs will end up in the snapshot, so if there are any additional secret
            # outputs, record them here.
            additional_secret_outputs = _translate_additional_secret_outputs(res, typ, opts.additional_secret_outputs)

            accept_resources = not (os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper() in {"TRUE", "1"})
            req = resource_pb2.ReadResourceRequest(
                type=ty,
                name=name,
                id=resolved_id,
                parent=resolver.parent_urn,
                provider=resolver.provider_ref,
                properties=resolver.serialized_props,
                dependencies=resolver.dependencies,
                version=opts.version or "",
                acceptSecrets=True,
                acceptResources=accept_resources,
                additionalSecretOutputs=additional_secret_outputs,
            )

            from ..resource import create_urn  # pylint: disable=import-outside-toplevel
            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call():
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(mock_urn, None, resolver.serialized_props, None)

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.ReadResource(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}")
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            resolve_id(None, True, False, exn)
            raise

        log.debug(f"resource read successful: ty={ty}, urn={resp.urn}")
        resolve_urn(resp.urn, True, False, None)
        resolve_id(resolved_id, True, False, None)  # Read IDs are always known.
        rpc.resolve_outputs(res, resolver.serialized_props, resp.properties, {}, resolvers,
                            transform_using_type_metadata)

    asyncio.ensure_future(RPC_MANAGER.do_rpc("read resource", do_read)())


def register_resource(res: 'Resource',
                      ty: str,
                      name: str,
                      custom: bool,
                      remote: bool,
                      new_dependency: Callable[[str], 'Resource'],
                      props: 'Inputs',
                      opts: Optional['ResourceOptions'],
                      typ: Optional[type] = None) -> None:
    """
    Registers a new resource object with a given type t and name.  It returns the
    auto-generated URN and the ID that will resolve after the deployment has completed.  All
    properties will be initialized to property objects that the registration operation will resolve
    at the right time (or remain unresolved for deployments).
    """
    log.debug(f"registering resource: ty={ty}, name={name}, custom={custom}, remote={remote}")
    monitor = settings.get_monitor()

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Prepare the resource.

    # Simply initialize the URN property and get prepared to resolve it later on.
    # Note: a resource urn will always get a value, and thus the output property
    # for it can always run .apply calls.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # If a custom resource, make room for the ID property.
    resolve_id: Optional[Callable[[Any, bool, bool, Optional[Exception]], None]] = None
    if custom:
        (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Now "transfer" all input properties into unresolved futures on res.  This way,
    # this resource will look like it has all its output properties to anyone it is
    # passed to.  However, those futures won't actually resolve until the RPC returns
    resolvers = rpc.transfer_properties(res, props)

    async def do_register():
        try:
            resolver = await prepare_resource(res, ty, custom, remote, props, opts, typ)
            log.debug(f"resource registration prepared: ty={ty}, name={name}")

            property_dependencies = {}
            for key, deps in resolver.property_dependencies.items():
                property_dependencies[key] = resource_pb2.RegisterResourceRequest.PropertyDependencies(
                    urns=deps)

            ignore_changes = _translate_ignore_changes(res, typ, opts.ignore_changes)
            additional_secret_outputs = _translate_additional_secret_outputs(res, typ, opts.additional_secret_outputs)
            replace_on_changes = _translate_replace_on_changes(res, typ, opts.replace_on_changes)

            # Translate the CustomTimeouts object.
            custom_timeouts = None
            if opts.custom_timeouts is not None:
                custom_timeouts = resource_pb2.RegisterResourceRequest.CustomTimeouts()
                # It could be an actual CustomTimeouts object.
                if known_types.is_custom_timeouts(opts.custom_timeouts):
                    if opts.custom_timeouts.create is not None:
                        custom_timeouts.create = opts.custom_timeouts.create
                    if opts.custom_timeouts.update is not None:
                        custom_timeouts.update = opts.custom_timeouts.update
                    if opts.custom_timeouts.delete is not None:
                        custom_timeouts.delete = opts.custom_timeouts.delete
                # Or, it could be a workaround passing in a dict.
                elif isinstance(opts.custom_timeouts, dict):
                    if 'create' in opts.custom_timeouts:
                        custom_timeouts.create = opts.custom_timeouts['create']
                    if 'update' in opts.custom_timeouts:
                        custom_timeouts.update = opts.custom_timeouts['update']
                    if 'delete' in opts.custom_timeouts:
                        custom_timeouts.delete = opts.custom_timeouts['delete']
                else:
                    raise Exception("Expected custom_timeouts to be a CustomTimeouts object")

            accept_resources = not (os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper() in {"TRUE", "1"})
            req = resource_pb2.RegisterResourceRequest(
                type=ty,
                name=name,
                parent=resolver.parent_urn,
                custom=custom,
                object=resolver.serialized_props,
                protect=opts.protect,
                provider=resolver.provider_ref,
                providers=resolver.provider_refs,
                dependencies=resolver.dependencies,
                propertyDependencies=property_dependencies,
                deleteBeforeReplace=opts.delete_before_replace,
                deleteBeforeReplaceDefined=opts.delete_before_replace is not None,
                ignoreChanges=ignore_changes,
                version=opts.version or "",
                acceptSecrets=True,
                acceptResources=accept_resources,
                additionalSecretOutputs=additional_secret_outputs,
                importId=opts.import_,
                customTimeouts=custom_timeouts,
                aliases=resolver.aliases,
                supportsPartialValues=True,
                remote=remote,
                replaceOnChanges=replace_on_changes,
            )

            from ..resource import create_urn  # pylint: disable=import-outside-toplevel
            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call():
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(mock_urn, None, resolver.serialized_props, None)

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.RegisterResource(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        except Exception as exn:
            log.debug(f"exception when preparing or executing rpc: {traceback.format_exc()}")
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            if resolve_id is not None:
                resolve_id(None, True, False, exn)
            raise

        if resp is None:
            return

        # At this point we would like to return successfully and call
        # `rpc.resolve_outputs`, but unfortunately that itself can
        # throw an exception sometimes. This was causing Pulumi
        # program to hang, so the additional try..except block is used
        # to propagate this exception into `rpc.resolve_outputs` which
        # causes it to display.

        resolve_outputs_called = False
        resolve_id_called = False
        resolve_urn_called = False

        try:
            log.debug(f"resource registration successful: ty={ty}, urn={resp.urn}")

            resolve_urn(resp.urn, True, False, None)
            resolve_urn_called = True

            if resolve_id is not None:
                # The ID is known if (and only if) it is a non-empty string. If it's either None or an
                # empty string, we should treat it as unknown. TFBridge in particular is known to send
                # the empty string as an ID when doing a preview.
                is_known = bool(resp.id)
                resolve_id(resp.id, is_known, False, None)
                resolve_id_called = True

            deps = {}
            rpc_deps = resp.propertyDependencies
            if rpc_deps:
                for k, v in rpc_deps.items():
                    urns = list(v.urns)
                    deps[k] = set(map(new_dependency, urns))

            rpc.resolve_outputs(res, resolver.serialized_props, resp.object, deps, resolvers, transform_using_type_metadata)
            resolve_outputs_called = True

        except Exception as exn:
            log.debug(f"exception after executing rpc: {traceback.format_exc()}")

            if not resolve_outputs_called:
                rpc.resolve_outputs_due_to_exception(resolvers, exn)

            if not resolve_urn_called:
                resolve_urn(None, True, False, exn)

            if resolve_id is not None and not resolve_id_called:
                resolve_id(None, True, False, exn)

            raise

    asyncio.ensure_future(RPC_MANAGER.do_rpc(
        "register resource", do_register)())


def register_resource_outputs(res: 'Resource', outputs: 'Union[Inputs, Output[Inputs]]'):
    async def do_register_resource_outputs():
        urn = await res.urn.future()
        serialized_props = await rpc.serialize_properties(outputs, {})
        log.debug(
            f"register resource outputs prepared: urn={urn}, props={serialized_props}")
        monitor = settings.get_monitor()
        req = resource_pb2.RegisterResourceOutputsRequest(
            urn=urn, outputs=serialized_props)

        def do_rpc_call():
            if monitor is None:
                # If there's no engine attached, simply ignore it.
                return None

            try:
                return monitor.RegisterResourceOutputs(req)
            except grpc.RpcError as exn:
                handle_grpc_error(exn)
                return None

        await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        log.debug(
            f"resource registration successful: urn={urn}, props={serialized_props}")

    asyncio.ensure_future(RPC_MANAGER.do_rpc(
        "register resource outputs", do_register_resource_outputs)())


class PropertyDependencies:
    urns: List[str]

    def __init__(self, urns: List[str]):
        self.urns = urns


class RegisterResponse:
    urn: str
    id: Optional[str]
    object: struct_pb2.Struct
    propertyDependencies: Optional[Dict[str, PropertyDependencies]]

    # pylint: disable=redefined-builtin
    def __init__(self,
                 urn: str,
                 id: Optional[str],
                 object: struct_pb2.Struct,
                 propertyDependencies: Optional[Dict[str, PropertyDependencies]]):
        self.urn = urn
        self.id = id
        self.object = object
        self.propertyDependencies = propertyDependencies


def convert_providers(
        provider: Optional['ProviderResource'],
        providers: Optional[Union[Mapping[str, 'ProviderResource'],
                                  Sequence['ProviderResource']]]) -> Mapping[str, 'ProviderResource']:
    """
    Merge all providers opts (opts.provider and both list and dict forms of opts.providers) into a single dict.
    """
    if provider is not None:
        return convert_providers(None, [provider])

    if providers is None:
        return {}

    if isinstance(providers, Mapping):
        return providers

    result = {}
    for p in providers:
        result[p.package] = p

    return result


async def _add_dependency(deps: Set[str], res: 'Resource', from_resource: Optional['Resource']):
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

    from .. import ComponentResource # pylint: disable=import-outside-toplevel

    if isinstance(res, ComponentResource):
        for child in res._childResources:
            await _add_dependency(deps, child, from_resource)
        if not res._remote:
            return

    no_cycles = declare_dependency(from_resource, res) if from_resource else True
    if no_cycles:
        urn = await res.urn.future()
        if urn:
            deps.add(urn)


async def _expand_dependencies(deps: Iterable['Resource'], from_resource: Optional['Resource']) -> Set[str]:
    """
    _expand_dependencies expands the given iterable of Resources into a set of URNs.
    """

    urns: Set[str] = set()
    for d in deps:
        await _add_dependency(urns, d, from_resource)

    return urns


async def _resolve_depends_on_urns(options: 'ResourceOptions', from_resource: 'Resource') -> Set[str]:
    """
    Resolves the set of all dependent resources implied by
    `depends_on`, either directly listed or implied in the Input
    layer. Returns a deduplicated URN list.
    """

    if options.depends_on is None:
        return set()

    outer = Output._from_input_shallow(options._depends_on_list())
    all_deps = await outer.resources()
    inner_list = await outer.future() or []

    for i in inner_list:
        inner = Output.from_input(i)
        more_deps = await inner.resources()
        all_deps = all_deps | more_deps
        direct_dep = await inner.future()
        if direct_dep is not None:
            all_deps.add(direct_dep)

    return await _expand_dependencies(all_deps, from_resource)
