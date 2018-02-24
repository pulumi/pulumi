# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

"""
Resource-related runtime functions.  These are not designed for external use.
"""

from ..errors import RunError
from google.protobuf import struct_pb2
from proto import resource_pb2
from settings import get_monitor
import six
import sys

def register_resource(res, typ, name, custom, props, opts):
    """
    Registers a new resource object with a given type and name.  This call is synchronous while the resource is
    created and All properties will be initialized to real property values once it completes.
    """

    # Serialize all properties.  This just translates known types into the gRPC marshalable equivalents.
    objprops = serialize_resource_props(props)

    # Ensure we have flushed all stdout/stderr, in case the RPC fails.
    sys.stdout.flush()
    sys.stderr.flush()

    # Now perform the resource registration.  This is synchronous and will return only after the operation completes.
    # TODO(joe): asynchronous registration to support parallelism.
    monitor = get_monitor()
    resp = monitor.RegisterResource(resource_pb2.RegisterResourceRequest(
        type=typ,
        name=name,
        parent=opts.parent.urn if opts and opts.parent else None,
        custom=custom,
        object=objprops,
        protect=opts.protect if opts else None))

    # Now copy the URN and ID properties back onto the resource object.
    res.urn = resp.urn
    if custom:
        if resp.id is None or resp.id == "":
            res.id = UNKNOWN
        else:
            res.id = resp.id

    # Now let the class itself decide how to accept output properties, if desired.
    if resp.object:
        outs = dict()
        for k, v in resp.object.items():
            outs[k] = v
        res.set_outputs(outs)

def register_resource_outputs(res, outputs):
    """
    Registers custom resource output properties.  This call is serial and blocks until the registration completes.
    """

    # Serialize all properties.  This just translates known types into the gRPC marshalable equivalents.
    objouts = serialize_resource_props(outputs)

    # Ensure we have flushed all stdout/stderr, in case the RPC fails.
    sys.stdout.flush()
    sys.stderr.flush()

    # Now perform the output registration.  This is synchronous and will return only after the operation completes.
    # TODO(joe): asynchronous registration to support parallelism.
    monitor = get_monitor()
    monitor.RegisterResourceOutputs(resource_pb2.RegisterResourceOutputsRequest(
        urn=res.urn,
        outputs=objouts))

def serialize_resource_props(props):
    """
    Serializes resource properties so that they are ready for marshaling to the gRPC endpoint.
    """
    struct = struct_pb2.Struct()
    for k, v in props.items():
        struct[k] = serialize_resource_value(v) # pylint: disable=unsupported-assignment-operation
    return struct

from ..resource import CustomResource

UNKNOWN = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
"""If a value is None, we serialize as UNKNOWN, which tells the engine that it may be computed later."""

def serialize_resource_value(value):
    """
    Seralizes a resource property value so that it's ready for marshaling to the gRPC endpoint.
    """
    if isinstance(value, CustomResource):
        # Resource objects aren't serializable.  Instead, serialize them as references to their IDs.
        return serialize_resource_value(value.id)
    elif isinstance(value, dict):
        # Deeply serialize dictionaries.
        d = dict()
        for k, v in value.items():
            d[k] = serialize_resource_value(v)
        return d
    elif isinstance(value, list):
        # Deeply serialize lists.
        a = []
        for e in value:
            a.append(serialize_resource_value(e))
        return a
    else:
        # All other values are directly serializable.
        # TODO(joe): eventually, we want to think about Output, Properties, and so on.
        return value
