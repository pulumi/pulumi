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
from typing import Optional, Any, Callable, List, NamedTuple, Dict, Set, TYPE_CHECKING
import grpc

from . import rpc, settings, known_types
from .. import log
from ..runtime.proto import resource_pb2
from .rpc_manager import RPC_MANAGER

if TYPE_CHECKING:
    from .. import Resource, ResourceOptions
    from ..output import Output, Inputs


class ResourceResolverOperations(NamedTuple): #TODO(sean) rename
    parent_urn: Optional[str]
    serialized_props: Dict[str, Any]
    dependencies: Set[str]


# Prepares for an RPC that will manufacture a resource, and hence deals with input and output properties.
async def prepare_resource(res: 'Resource', ty: str, props: 'Inputs', opts: Optional['ResourceOptions']) -> ResourceResolverOperations:
    log.debug(f"resource {props} preparing to wait for dependencies")
    # Before we can proceed, all our dependencies must be finished.
    explicit_urn_dependencies = []
    if opts is not None and opts.depends_on is not None:
        dependent_urns = list(map(lambda r: r.urn.future(), opts.depends_on))
        explicit_urn_dependencies = await asyncio.gather(*dependent_urns)

    # Serialize out all our props to their final values.  In doing so, we'll also collect all
    # the Resources pointed to by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
    implicit_dependencies: List[Resource] = []
    serialized_props = await rpc.serialize_properties(props, implicit_dependencies, res.translate_input_property)

    # Wait for our parent to resolve
    parent_urn = ""
    if opts is not None and opts.parent is not None:
        parent_urn = await opts.parent.urn.future()
    elif ty != "pulumi:pulumi:Stack": # TODO(sean) is it necessary to check the type here?
        # If no parent was provided, parent to the root resource.
        parent = settings.get_root_resource()
        if parent is not None:
            parent_urn = await parent.urn.future()

    # TODO(swgillespie, first class providers) here (pulumi/pulumi#1713)
    dependencies = set(explicit_urn_dependencies)
    for implicit_dep in implicit_dependencies:
        dependencies.add(await implicit_dep.urn.future())

    log.debug(f"resource {props} prepared")
    return ResourceResolverOperations(
        parent_urn,
        serialized_props,
        dependencies
    )


# pylint: disable=too-many-locals
def register_resource(res: 'Resource', ty: str, name: str, custom: bool, props: 'Inputs', opts: Optional['ResourceOptions']):
    """
    registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
    URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
    objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
    """
    log.debug(f"registering resource: ty={ty}, name={name}, custom={custom}")
    monitor = settings.get_monitor()

    # Prepare the resource.

    # Simply initialize the URN property and get prepared to resolve it later on.
    # Note: a resource urn will always get a value, and thus the output property
    # for it can always run .apply calls.
    log.debug(f"preparing resource for RPC")
    urn_future = asyncio.Future()
    urn_known = asyncio.Future()
    urn_known.set_result(True)
    resolve_urn: Callable[[str], None] = urn_future.set_result
    res.urn = known_types.new_output({res}, urn_future, urn_known)

    # If a custom resource, make room for the ID property.
    resolve_id: Optional[Callable[[Any, str], None]] = None
    if custom:
        resolve_value = asyncio.Future()
        resolve_perform_apply = asyncio.Future()
        res.id = known_types.new_output({res}, resolve_value, resolve_perform_apply)

        def do_resolve(value: Any, perform_apply: bool):
            resolve_value.set_result(value)
            resolve_perform_apply.set_result(perform_apply)

        resolve_id = do_resolve

    # Now "transfer" all input properties into unresolved futures on res.  This way,
    # this resource will look like it has all its output properties to anyone it is
    # passed to.  However, those futures won't actually resolve until the RPC returns
    resolvers = rpc.transfer_properties(res, props)

    async def do_register():
        log.debug(f"preparing resource registration: ty={ty}, name={name}")
        resolver = await prepare_resource(res, ty, props, opts)
        log.debug(f"resource registration prepared: ty={ty}, name={name}")
        req = resource_pb2.RegisterResourceRequest(
            type=ty,
            name=name,
            parent=resolver.parent_urn,
            custom=custom,
            object=resolver.serialized_props,
            protect=False,
            dependencies=resolver.dependencies
        )

        def do_rpc_call():
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
        log.debug(f"resource registration successful: ty={ty}, urn={resp.urn}")
        resolve_urn(resp.urn)
        if resolve_id:
            is_known = resp.id is not None
            resolve_id(resp.id, is_known)

        await rpc.resolve_outputs(res, props, resp.object, resolvers)

    asyncio.ensure_future(RPC_MANAGER.do_rpc("register resource", do_register)())

def register_resource_outputs(res: 'Resource', outputs: 'Union[Inputs, Awaitable[Inputs], Output[Inputs]]'):
    async def do_register_resource_outputs():
        urn = await res.urn.future()
        serialized_props = await rpc.serialize_properties(outputs, [])
        log.debug(f"register resource outputs prepared: urn={urn}, props={serialized_props}")
        monitor = settings.get_monitor()
        req = resource_pb2.RegisterResourceOutputsRequest(urn=urn, outputs=serialized_props)

        def do_rpc_call():
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
        log.debug(f"resource registration successful: urn={urn}, props={serialized_props}")

    asyncio.ensure_future(RPC_MANAGER.do_rpc("register resource outputs", do_register_resource_outputs)())
