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
from __future__ import absolute_import

from ..errors import RunError
from google.protobuf import struct_pb2
from .proto import provider_pb2, resource_pb2
from .settings import get_monitor
from . import rpc
import six
import sys
import grpc

def invoke(tok, args):
    """
    Dynamically invokes the function identified by tok, which is implemented by a provider plugin.  The input args
    is a dictionary of arbitrary values, and the return value contains a similar dictionary returned by the function.
    """

    # Ensure we have flushed all stdout/stderr, in case the RPC fails.
    sys.stdout.flush()
    sys.stderr.flush()

    # Now perform the invocation.  This is synchronous and will return only after the operation completes.
    # TODO[pulumi/pulumi#1063]: asynchronous registration to support parallelism.
    monitor = get_monitor()
    try:
        resp = monitor.Invoke(provider_pb2.InvokeRequest(
            tok=tok,
            args=rpc.serialize_resource_props(args)))
    except grpc.RpcError as exn:
        # gRPC-python gets creative with their exceptions. grpc.RpcError as a type is useless;
        # the usefullness come from the fact that it is polymorphically also a grpc.Call and thus has
        # the .code() member. Pylint doesn't know this because it's not known statically.
        #
        # Neither pylint nor I are the only ones who find this confusing:
        # https://github.com/grpc/grpc/issues/10885#issuecomment-302581315
        # pylint: disable=no-member
        if exn.code() == grpc.StatusCode.UNAVAILABLE:
            wait_for_death()

        # If the RPC otherwise failed, re-throw an exception with the message details - the contents
        # are suitable for user presentation.
        raise Exception(exn.details())


    # If the invoke failed, raise an error.
    if resp.failures:
        raise Exception('invoke of %s failed: %s (%s)' % (tok, resp.failures[0].reason, resp.failures[0].property))

    # Otherwise, return the output properties.
    retobj = getattr(resp, 'return')
    if retobj:
        return rpc.deserialize_resource_props(retobj)

    return {}

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
            wait_for_death()

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
            wait_for_death()
            
        # If the RPC otherwise failed, re-throw an exception with the message details - the contents
        # are suitable for user presentation.
        raise Exception(exn.details())



# wait_for_death loops forever. This is a hack.
#
# The purpose of this hack is to deal with graceful termination of the resource monitor.
# When the engine decides that it needs to terminate, it shuts down the Log and ResourceMonitor RPC
# endpoints. Shutting down RPC endpoints involves draining all outstanding RPCs and denying new connections.
#
# This is all fine for us as the language host, but we need to 1) not let the RPC that just failed due to
# the ResourceMonitor server shutdown get displayed as an error and 2) not do any more RPCs, since they'll fail.
#
# We can accomplish both by just doing spinning forever until the engine kills us. It's ugly, but it works.
def wait_for_death():
    while True:
        pass
