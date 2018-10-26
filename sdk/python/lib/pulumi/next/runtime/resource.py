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
Resource-related runtime functions.  These are not designed for external use.
"""
import asyncio
from typing import NamedTuple, Optional

from ...runtime.proto import provider_pb2, resource_pb2
from .settings import get_monitor
from . import rpc
import sys
import grpc

from ..output import Output, Inputs
from .. import log
from ..resource import Resource, ResourceOptions


async def invoke(tok: str, args: Inputs) -> dict:
    """
    Dynamically invokes the function identified by tok, which is implemented by a provider plugin.  The input args
    is a dictionary of arbitrary values, and the return value contains a similar dictionary returned by the function.
    """
    log.debug(f"Invoking function tok={tok}")
    props = await rpc.serialize_resource_props(args)  # TODO(sean, rpc)
    log.debug(f"Invoke RPC prepared: tok={tok}")

    # TODO(sean, providers) - first class provider reference stuff goes here

    # Set up our RPC call. Before we get started we allocate a future
    loop = asyncio.get_event_loop()
    fut: asyncio.Future = loop.create_future()
    req = provider_pb2.InvokeRequest(tok=tok, args=props)
    monitor = get_monitor()

    def do_invoke():
        try:
            resp = monitor.Invoke(req)
            log.debug(f"Invoke of {tok} finished")
            if resp.failures:
                fail_msg = f"Invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})"
                fut.set_exception(Exception(fail_msg))
                return

            ret_obj = getattr(resp, 'return')
            if ret_obj:
                fut.set_result(rpc.deserialize_resource_props(ret_obj))
            else:
                fut.set_result({})
        except grpc.RpcError as exn:
            # gRPC-python gets creative with their exceptions. grpc.RpcError as a type is useless;
            # the usefullness come from the fact that it is polymorphically also a grpc.Call and thus has
            # the .code() member. Pylint doesn't know this because it's not known statically.
            #
            # Neither pylint nor I are the only ones who find this confusing:
            # https://github.com/grpc/grpc/issues/10885#issuecomment-302581315
            # pylint: disable=no-member
            if exn.code() == grpc.StatusCode.UNAVAILABLE:
                sys.exit(0)

            # If the RPC otherwise failed, re-throw an exception with the message details - the contents
            # are suitable for user presentation.
            fut.set_exception(Exception(exn.details()))

    loop.call_soon(do_invoke)
    return await fut


class RegisterResourceResult(object):
    """
    RegisterResourceResult contains the assigned URN, the ID -- if applicable -- and the resulting resource
    output properties, representing a resource's state after registration has completed.
    """
    def __init__(self, urn, id, outputs):
        self.urn = urn
        self.id = id
        self.outputs = outputs

def register_resource(typ, name, custom, props, opts):
    """
    Registers a new resource object with a given type and name.  This call is synchronous while the resource is
    created and All properties will be initialized to real property values once it completes.
    """

    # Serialize all properties.  This just translates known types into the gRPC marshalable equivalents.
    objprops = rpc.serialize_resource_props(props)

    # Ensure we have flushed all stdout/stderr, in case the RPC fails.
    sys.stdout.flush()
    sys.stderr.flush()

    # Now perform the resource registration.  This is synchronous and will return only after the operation completes.
    # TODO[pulumi/pulumi#1063]: asynchronous registration to support parallelism.
    monitor = get_monitor()
    try:
        resp = monitor.RegisterResource(resource_pb2.RegisterResourceRequest(
            type=typ,
            name=name,
            parent=opts.parent.urn if opts and opts.parent else None,
            custom=custom,
            object=objprops,
            protect=opts.protect if opts else None))
    except grpc.RpcError as exn:
        # See the above comment on invoke for the justification for disabling
        # this warning
        # pylint: disable=no-member
        if exn.code() == grpc.StatusCode.UNAVAILABLE:
            sys.exit(0)

        # If the RPC otherwise failed, re-throw an exception with the message details - the contents
        # are suitable for user presentation.
        raise Exception(exn.details())


    # Return the URN, ID, and output properties.
    urn = resp.urn
    if custom:
        if resp.id:
            id = resp.id
        else:
            id = None
    else:
        id = None
    outputs = dict()
    if resp.object:
        outputs = rpc.deserialize_resource_props(resp.object)

    return RegisterResourceResult(urn, id, outputs)

def register_resource_outputs(res, outputs):
    """
    Registers custom resource output properties.  This call is serial and blocks until the registration completes.
    """

    # Serialize all properties.  This just translates known types into the gRPC marshalable equivalents.
    objouts = rpc.serialize_resource_props(outputs)

    # Ensure we have flushed all stdout/stderr, in case the RPC fails.
    sys.stdout.flush()
    sys.stderr.flush()

    # Now perform the output registration.  This is synchronous and will return only after the operation completes.
    # TODO[pulumi/pulumi#1063]: asynchronous registration to support parallelism.
    monitor = get_monitor()
    try:
        monitor.RegisterResourceOutputs(resource_pb2.RegisterResourceOutputsRequest(
            urn=res.urn,
            outputs=objouts))
    except grpc.RpcError as exn:
        # See the above comment on invoke for the justification for disabling
        # this warning
        # pylint: disable=no-member
        if exn.code() == grpc.StatusCode.UNAVAILABLE:
            sys.exit(0)
            
        # If the RPC otherwise failed, re-throw an exception with the message details - the contents
        # are suitable for user presentation.
        raise Exception(exn.details())


class PreparedResourceOp(NamedTuple):
    urn_future: asyncio.Future
    id_future: Optional[asyncio.Future]


async def prepare_resource(res: Resource, custom: bool, props: Inputs, opts: ResourceOptions):
    loop: asyncio.AbstractEventLoop = asyncio.get_event_loop()

    # Set up a future to resolve the URN of this resource.
    urn_future = loop.create_future()
    urn_known_future = loop.create_future()
    urn_known_future.set_result(True)
    res.urn = Output({res}, urn_future, urn_known_future)

    # If this is a custom resource, set up a future for the ID.
    id_future = None
    if custom:
        resolve_value_future = loop.create_future()
        resolve_perform_apply = loop.create_future()
        id_future = loop.create_future()

        async def resolve_id():
            (v, perform_apply) = await id_future
            resolve_value_future.set_result(v)
            resolve_perform_apply.set_result(perform_apply)
        loop.call_soon(resolve_id)

        res.id = Output({res}, resolve_value_future, resolve_perform_apply)

