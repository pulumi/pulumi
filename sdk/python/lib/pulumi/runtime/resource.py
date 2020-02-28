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
import asyncio
import sys
import traceback

from typing import Optional, Any, Callable, List, NamedTuple, Dict, Set, Union, TYPE_CHECKING, cast
from google.protobuf import struct_pb2
import grpc

from . import rpc, settings, known_types
from .. import log
from ..runtime.proto import resource_pb2
from .rpc_manager import RPC_MANAGER
from ..metadata import get_project, get_stack

from ..output import Output

if TYPE_CHECKING:
    from .. import Resource, ResourceOptions, CustomResource
    from ..output import Inputs


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
                           props: 'Inputs',
                           opts: Optional['ResourceOptions']) -> ResourceResolverOperations:
    log.debug(f"resource {props} preparing to wait for dependencies")
    # Before we can proceed, all our dependencies must be finished.
    explicit_urn_dependencies = []
    if opts is not None and opts.depends_on is not None:
        dependent_urns = list(map(lambda r: r.urn.future(), opts.depends_on))
        explicit_urn_dependencies = await asyncio.gather(*dependent_urns)

    # Serialize out all our props to their final values.  In doing so, we'll also collect all
    # the Resources pointed to by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
    property_dependencies_resources: Dict[str, List['Resource']] = {}
    serialized_props = await rpc.serialize_properties(props, property_dependencies_resources, res.translate_input_property)

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

    dependencies = set(explicit_urn_dependencies)
    property_dependencies: Dict[str, List[Optional[str]]] = {}
    for key, deps in property_dependencies_resources.items():
        urns = set()
        for dep in deps:
            urn = await dep.urn.future()
            urns.add(urn)
            dependencies.add(urn)
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

    log.debug(f"resource {props} prepared")
    return ResourceResolverOperations(
        parent_urn,
        serialized_props,
        dependencies,
        provider_ref,
        property_dependencies,
        aliases,
    )

# pylint: disable=too-many-locals,too-many-statements


def read_resource(res: 'CustomResource', ty: str, name: str, props: 'Inputs', opts: 'ResourceOptions'):
    if opts.id is None:
        raise Exception(
            "Cannot read resource whose options are lacking an ID value")

    log.debug(f"reading resource: ty={ty}, name={name}, id={opts.id}")
    monitor = settings.get_monitor()

    # Prepare the resource, similar to a RegisterResource. Reads are deliberately similar to RegisterResource except
    # that we are populating the Resource object with properties associated with an already-live resource.
    #
    # Same as below, we initialize the URN property on the resource, which will always be resolved.
    log.debug(f"preparing read resource for RPC")
    urn_future: asyncio.Future[Any] = asyncio.Future()
    urn_known: asyncio.Future[bool] = asyncio.Future()
    urn_secret: asyncio.Future[bool] = asyncio.Future()
    urn_known.set_result(True)
    urn_secret.set_result(False)
    resolve_urn = urn_future.set_result
    resolve_urn_exn = urn_future.set_exception
    res.urn = known_types.new_output({res}, urn_future, urn_known, urn_secret)

    # Furthermore, since resources being Read must always be custom resources (enforced in the
    # Resource constructor), we'll need to set up the ID field which will be populated at the end of
    # the ReadResource call.
    #
    # Note that we technically already have the ID (opts.id), but it's more consistent with the rest
    # of the model to resolve it asynchronously along with all of the other resources.

    resolve_value: asyncio.Future[Any] = asyncio.Future()
    resolve_perform_apply: asyncio.Future[bool] = asyncio.Future()
    resolve_secret: asyncio.Future[bool] = asyncio.Future()
    res.id = known_types.new_output(
        {res}, resolve_value, resolve_perform_apply, resolve_secret)

    def do_resolve(value: Any, perform_apply: bool, exn: Optional[Exception]):
        if exn is not None:
            resolve_value.set_exception(exn)
            resolve_perform_apply.set_exception(exn)
            resolve_secret.set_exception(exn)
        else:
            resolve_value.set_result(value)
            resolve_perform_apply.set_result(perform_apply)
            resolve_secret.set_result(False)

    resolve_id = do_resolve

    # Like below, "transfer" all input properties onto unresolved futures on res.
    resolvers = rpc.transfer_properties(res, props)

    async def do_read():
        try:
            log.debug(f"preparing read: ty={ty}, name={name}, id={opts.id}")
            resolver = await prepare_resource(res, ty, True, props, opts)

            # Resolve the ID that we were given. Note that we are explicitly discarding the list of
            # dependencies returned to us from "serialize_property" (the second argument). This is
            # because a "read" resource does not actually have any dependencies at all in the cloud
            # provider sense, because a read resource already exists. We do not need to track this
            # dependency.
            resolved_id = await rpc.serialize_property(opts.id, [])
            log.debug(f"read prepared: ty={ty}, name={name}, id={opts.id}")

            # These inputs will end up in the snapshot, so if there are any additional secret
            # outputs, record them here.
            additional_secret_outputs = opts.additional_secret_outputs
            if res.translate_input_property is not None and opts.additional_secret_outputs is not None:
                additional_secret_outputs = map(
                    res.translate_input_property, opts.additional_secret_outputs)

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
                additionalSecretOutputs=additional_secret_outputs,
            )

            from ..resource import create_urn # pylint: disable=import-outside-toplevel
            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call():
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(mock_urn, None, resolver.serialized_props)

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.ReadResource(req)
                except grpc.RpcError as exn:
                    # See the comment on invoke for the justification for disabling
                    # this warning
                    # pylint: disable=no-member
                    if exn.code() == grpc.StatusCode.UNAVAILABLE:
                        sys.exit(0)

                    details = exn.details()
                raise Exception(details)

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}")
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn_exn(exn)
            resolve_id(None, False, exn)
            raise

        log.debug(f"resource read successful: ty={ty}, urn={resp.urn}")
        resolve_urn(resp.urn)
        resolve_id(resolved_id, True, None)  # Read IDs are always known.
        await rpc.resolve_outputs(res, resolver.serialized_props, resp.properties, resolvers)

    asyncio.ensure_future(RPC_MANAGER.do_rpc("read resource", do_read)())

# pylint: disable=too-many-locals,too-many-statements


def register_resource(res: 'Resource', ty: str, name: str, custom: bool, props: 'Inputs', opts: Optional['ResourceOptions']):
    """
    registerResource registers a new resource object with a given type t and name.  It returns the
    auto-generated URN and the ID that will resolve after the deployment has completed.  All
    properties will be initialized to property objects that the registration operation will resolve
    at the right time (or remain unresolved for deployments).
    """
    log.debug(f"registering resource: ty={ty}, name={name}, custom={custom}")
    monitor = settings.get_monitor()

    # Prepare the resource.

    # Simply initialize the URN property and get prepared to resolve it later on.
    # Note: a resource urn will always get a value, and thus the output property
    # for it can always run .apply calls.
    log.debug(f"preparing resource for RPC")
    urn_future: asyncio.Future[Any] = asyncio.Future()
    urn_known: asyncio.Future[bool] = asyncio.Future()
    urn_secret: asyncio.Future[bool] = asyncio.Future()
    urn_known.set_result(True)
    urn_secret.set_result(False)
    resolve_urn = urn_future.set_result
    resolve_urn_exn = urn_future.set_exception
    res.urn = known_types.new_output({res}, urn_future, urn_known, urn_secret)

    # If a custom resource, make room for the ID property.
    resolve_id: Optional[Callable[[
        Any, bool, Optional[Exception]], None]] = None
    if custom:
        res = cast('CustomResource', res)
        resolve_value: asyncio.Future[Any] = asyncio.Future()
        resolve_perform_apply: asyncio.Future[bool] = asyncio.Future()
        resolve_secret: asyncio.Future[bool] = asyncio.Future()
        res.id = known_types.new_output(
            {res}, resolve_value, resolve_perform_apply, resolve_secret)

        def do_resolve(value: Any, perform_apply: bool, exn: Optional[Exception]):
            if exn is not None:
                resolve_value.set_exception(exn)
                resolve_perform_apply.set_exception(exn)
                resolve_secret.set_exception(exn)
            else:
                resolve_value.set_result(value)
                resolve_perform_apply.set_result(perform_apply)
                resolve_secret.set_result(False)

        resolve_id = do_resolve

    # Now "transfer" all input properties into unresolved futures on res.  This way,
    # this resource will look like it has all its output properties to anyone it is
    # passed to.  However, those futures won't actually resolve until the RPC returns
    resolvers = rpc.transfer_properties(res, props)

    async def do_register():
        try:
            log.debug(f"preparing resource registration: ty={ty}, name={name}")
            resolver = await prepare_resource(res, ty, custom, props, opts)
            log.debug(f"resource registration prepared: ty={ty}, name={name}")

            property_dependencies = {}
            for key, deps in resolver.property_dependencies.items():
                property_dependencies[key] = resource_pb2.RegisterResourceRequest.PropertyDependencies(
                    urns=deps)

            ignore_changes = opts.ignore_changes
            if res.translate_input_property is not None and opts.ignore_changes is not None:
                ignore_changes = map(
                    res.translate_input_property, opts.ignore_changes)

            # Note that while `additional_secret_outputs` lists property names that are outputs, we
            # call `translate_input_property` because it is the method that converts from the
            # language projection name to the provider name, which is what we want.
            additional_secret_outputs = opts.additional_secret_outputs
            if res.translate_input_property is not None and opts.additional_secret_outputs is not None:
                additional_secret_outputs = map(
                    res.translate_input_property, opts.additional_secret_outputs)

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

            req = resource_pb2.RegisterResourceRequest(
                type=ty,
                name=name,
                parent=resolver.parent_urn,
                custom=custom,
                object=resolver.serialized_props,
                protect=opts.protect,
                provider=resolver.provider_ref,
                dependencies=resolver.dependencies,
                propertyDependencies=property_dependencies,
                deleteBeforeReplace=opts.delete_before_replace,
                deleteBeforeReplaceDefined=opts.delete_before_replace is not None,
                ignoreChanges=ignore_changes,
                version=opts.version or "",
                acceptSecrets=True,
                additionalSecretOutputs=additional_secret_outputs,
                importId=opts.import_,
                customTimeouts=custom_timeouts,
                aliases=resolver.aliases,
                supportsPartialValues=True,
            )

            from ..resource import create_urn # pylint: disable=import-outside-toplevel
            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call():
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(mock_urn, None, resolver.serialized_props)

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.RegisterResource(req)
                except grpc.RpcError as exn:
                    # See the comment on invoke for the justification for disabling
                    # this warning
                    # pylint: disable=no-member
                    if exn.code() == grpc.StatusCode.UNAVAILABLE:
                        sys.exit(0)

                    details = exn.details()
                raise Exception(details)

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}")
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn_exn(exn)
            if resolve_id is not None:
                resolve_id(None, False, exn)
            raise

        log.debug(f"resource registration successful: ty={ty}, urn={resp.urn}")
        resolve_urn(resp.urn)
        if resolve_id:
            # The ID is known if (and only if) it is a non-empty string. If it's either None or an
            # empty string, we should treat it as unknown. TFBridge in particular is known to send
            # the empty string as an ID when doing a preview.
            is_known = bool(resp.id)
            resolve_id(resp.id, is_known, None)

        await rpc.resolve_outputs(res, resolver.serialized_props, resp.object, resolvers)

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
                # See the comment on invoke for the justification for disabling
                # this warning
                # pylint: disable=no-member
                if exn.code() == grpc.StatusCode.UNAVAILABLE:
                    sys.exit(0)

                details = exn.details()
            raise Exception(details)

        await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        log.debug(
            f"resource registration successful: urn={urn}, props={serialized_props}")

    asyncio.ensure_future(RPC_MANAGER.do_rpc(
        "register resource outputs", do_register_resource_outputs)())


class RegisterResponse:
    urn: str
    id: str
    object: struct_pb2.Struct

    # pylint: disable=redefined-builtin
    def __init__(self, urn: str, id: str, object: struct_pb2.Struct):
        self.urn = urn
        self.id = id
        self.object = object
