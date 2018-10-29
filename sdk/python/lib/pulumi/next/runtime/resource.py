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
import grpc
import sys
from typing import Optional, Awaitable, Any, Callable, List, NamedTuple, Dict, Set, Union, TYPE_CHECKING

from . import rpc, settings, known_types
from .. import log
from ...runtime.proto import resource_pb2

if TYPE_CHECKING:
    from .. import Resource, ResourceOptions
    from ..output import Output, Inputs


class RPCCounter:
    mutex: asyncio.Lock
    zero_cond: asyncio.Condition
    count: int

    def __init__(self):
        self.zero_cond = asyncio.Condition()
        self.count = 0

    async def wait_for_outstanding_rpcs(self):
        async with self.zero_cond:
            self.zero_cond.wait()

    async def __aenter__(self):
        async with self.zero_cond:
            log.debug(f"recorded new RPC, {self.count} RPCs outstanding")
            self.count += 1

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        async with self.zero_cond:
            self.count -= 1
            if self.count == 0:
                self.zero_cond.notify_all()


_COUNTER: RPCCounter = RPCCounter()


async def do_rpc(func):
    async def wrapper(*args, **kwargs):
        log.debug("beginning RPC")
        async with _COUNTER:
            return await func(*args, **kwargs)

    return wrapper


class ResourceResolverOperations(NamedTuple):
    resolve_urn: Callable[[str], None]
    resolve_id: Callable[[Any, str], None]
    resolvers: Any
    parent_urn: Optional[str]
    serialized_props: Dict[str, Any]
    dependencies: Set[str]


# Prepares for an RPC that will manufacture a resource, and hence deals with input and output properties.
async def prepare_resource(res: 'Resource', custom: bool, props: 'Inputs', opts: 'ResourceOptions') -> ResourceResolverOperations:
    # Simply initialize the URN property and get prepared to resolve it later on.
    # Note: a resource urn will always get a value, and thus the output property
    # for it can always run .apply calls.
    log.debug(f"preparing resource for RPC", res)
    urn_future = asyncio.Future()
    urn_known = asyncio.Future()
    urn_known.set_result(True)
    resolve_urn: Callable[[str], None] = urn_future.set_result
    res.urn = known_types.new_output({res}, resolve_urn, urn_known)

    # If a custom resource, make room for the ID property.
    resolve_id: Callable[[Any, str], None]
    if custom:
        resolve_value = asyncio.Future()
        resolve_perform_apply = asyncio.Future()
        res.id = known_types.new_output({res}, resolve_value, resolve_perform_apply)

        def do_resolve(tup):
            (value, perform_apply) = tup
            resolve_value.set_result(value)
            resolve_perform_apply.set_result(perform_apply)

        resolve_id = do_resolve

    # Now "transfer" all input properties into unresolved futures on res.  This way,
    # this resource will look like it has all its output properties to anyone it is
    # passed to.  However, those futures won't actually resolve until the RPC returns
    resolvers = rpc.transfer_properties(res, props) # TODO(sean)

    # IMPORTANT!  We should never await prior to this line, otherwise the Resource will be partly uninitialized.

    log.debug(f"resource {props} preparing to wait for dependencies")
    # Before we can proceed, all our dependencies must be finished.
    explicit_urn_dependencies = []
    if opts.depends_on is not None:
        dependent_urns = list(map(lambda r: r.urn.future(), opts.depends_on))
        explicit_urn_dependencies = await asyncio.gather(*dependent_urns)

    # Serialize out all our props to their final values.  In doing so, we'll also collect all
    # the Resources pointed to by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
    implicit_dependencies: List[Resource] = []
    serialized_props = await rpc.serialize_properties(props, implicit_dependencies)

    # Wait for our parent to resolve
    if opts.parent:
        parent_urn = await opts.parent.urn.future()
    else:
        # If no parent was provided, parent to the root resource.
        parent_urn = settings.get_root_resource()


    # TODO(swgillespie, first class providers) here
    dependencies = set(explicit_urn_dependencies)
    for implicit_dep in implicit_dependencies:
        dependencies.add(await implicit_dep.urn.promise())

    log.debug(f"resource {props} prepared")
    return ResourceResolverOperations(
        resolve_urn,
        resolve_id,
        resolvers,
        parent_urn,
        serialized_props,
        dependencies
    )


def register_resource(res: 'Resource', ty: str, name: str, custom: bool, props: 'Inputs', opts: Optional['ResourceOptions']):
    """
    registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
    URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
    objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
    """
    log.debug(f"registering resource: ty={ty}, name={name}, custom={custom}")
    monitor = settings.get_monitor()

    async def do_register():
        log.debug(f"preparing resource registration: ty={ty}, name={name}")
        resolver = await prepare_resource(res, custom, props, opts)
        log.debug(f"resource registration prepared: ty={ty}, name={name}")

        req = resource_pb2.RegisterResourceRequest(
            type=ty,
            name=name,
            parent=resolver.parent_urn,
            custom=custom,
            object=resolver.serialized_props,
            protect=opts.protect if opts.protect is not None else False
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

                # If the RPC otherwise failed, re-throw an exception with the message details - the contents
                # are suitable for user presentation.
                raise Exception(exn.details())

        resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        log.debug(f"resource registration successful: ty={ty}, urn={resp.urn}")
        resolver.resolve_urn(resp.urn)
        if resolver.resolve_id:
            is_known = resp.id is not None
            resolver.resolve_id(resp.id, is_known)

        await rpc.resolve_outputs(props, resp.object, resolver.resolvers)

    def bootstrap_coro(loop: asyncio.AbstractEventLoop, coro):
        asyncio.run_coroutine_threadsafe(coro, loop)

    loop = asyncio.get_event_loop()
    loop.create_task(do_rpc(do_register))
    #asyncio.run_coroutine_threadsafe(do_rpc(do_register), asyncio.get_event_loop())
    #asyncio.ensure_future(do_rpc(do_register))

def register_resource_outputs(res: 'Resource', inputs: 'Union[Inputs, Awaitable[Inputs], Output[Inputs]]'):
    pass
