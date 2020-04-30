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
import base64
from concurrent import futures
import sys
import time

import dill
import grpc
from google.protobuf import empty_pb2
from pulumi.runtime import proto, rpc
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer
from pulumi.dynamic import ResourceProvider

_ONE_DAY_IN_SECONDS = 60 * 60 * 24
PROVIDER_KEY = "__provider"

# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [('grpc.max_receive_message_length', _MAX_RPC_MESSAGE_SIZE)]


def get_provider(props) -> ResourceProvider:
    byts = base64.b64decode(props[PROVIDER_KEY])
    return dill.loads(byts)

class DynamicResourceProviderServicer(ResourceProviderServicer):
    def CheckConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("CheckConfig is not implemented by the dynamic provider")
        raise NotImplementedError("CheckConfig is not implemented by the dynamic provider")

    def DiffConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("DiffConfig is not implemented by the dynamic provider")
        raise NotImplementedError("DiffConfig is not implemented by the dynamic provider")

    def Invoke(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Invoke is not implemented by the dynamic provider")
        raise NotImplementedError("unknown function %s" % request.token)

    def Diff(self, request, context):
        olds = rpc.deserialize_properties(request.olds, True)
        news = rpc.deserialize_properties(request.news, True)
        if news[PROVIDER_KEY] == rpc.UNKNOWN:
            provider = get_provider(olds)
        else:
            provider = get_provider(news)
        result = provider.diff(request.id, olds, news)
        fields = {}
        if result.changes is not None:
            if result.changes:
                fields["changes"] = proto.DiffResponse.DIFF_SOME # pylint: disable=no-member
            else:
                fields["changes"] = proto.DiffResponse.DIFF_NONE # pylint: disable=no-member
        else:
            fields["changes"] = proto.DiffResponse.DIFF_UNKNOWN # pylint: disable=no-member
        if result.replaces is not None:
            fields["replaces"] = result.replaces
        if result.delete_before_replace is not None:
            fields["deleteBeforeReplace"] = result.delete_before_replace
        return proto.DiffResponse(**fields)

    def Update(self, request, context):
        olds = rpc.deserialize_properties(request.olds)
        news = rpc.deserialize_properties(request.news)
        provider = get_provider(news)

        result = provider.update(request.id, olds, news)
        outs = {}
        if result.outs is not None:
            outs = result.outs
        outs[PROVIDER_KEY] = news[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        outs_proto = loop.run_until_complete(rpc.serialize_properties(outs, {}))
        loop.close()

        fields = {"properties": outs_proto}
        return proto.UpdateResponse(**fields)

    def Delete(self, request, context):
        id_ = request.id
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props)
        provider.delete(id_, props)
        return empty_pb2.Empty()

    def Cancel(self, request, context):
        return empty_pb2.Empty()

    def Create(self, request, context):
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props)
        result = provider.create(props)
        outs = result.outs
        outs[PROVIDER_KEY] = props[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        outs_proto = loop.run_until_complete(rpc.serialize_properties(outs, {}))
        loop.close()

        fields = {"id": result.id, "properties": outs_proto}
        return proto.CreateResponse(**fields)

    def Check(self, request, context):
        olds = rpc.deserialize_properties(request.olds, True)
        news = rpc.deserialize_properties(request.news, True)
        if news[PROVIDER_KEY] == rpc.UNKNOWN:
            provider = get_provider(olds)
        else:
            provider = get_provider(news)

        result = provider.check(olds, news)
        inputs = result.inputs
        failures = result.failures

        inputs[PROVIDER_KEY] = news[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        inputs_proto = loop.run_until_complete(rpc.serialize_properties(inputs, {}))
        loop.close()

        failures_proto = [proto.CheckFailure(f.property, f.reason) for f in failures]

        fields = {"inputs": inputs_proto, "failures": failures_proto}
        return proto.CheckResponse(**fields)

    def Configure(self, request, context):
        fields = {"acceptSecrets": False}
        return proto.ConfigureResponse(**fields)

    def GetPluginInfo(self, request, context):
        fields = {"version": "0.1.0"}
        return proto.PluginInfo(**fields)

    def GetSchema(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("GetSchema is not implemented by the dynamic provider")
        raise NotImplementedError("GetSchema is not implemented by the dynamic provider")

    def Read(self, request, context):
        id_ = request.id
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props)
        result = provider.read(id_, props)
        outs = result.outs
        outs[PROVIDER_KEY] = props[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        outs_proto = loop.run_until_complete(rpc.serialize_properties(outs, {}))
        loop.close()

        fields = {"id": result.id, "properties": outs_proto}
        return proto.ReadResponse(**fields)

    def __init__(self):
        pass

def main():
    monitor = DynamicResourceProviderServicer()
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=4),
        options=_GRPC_CHANNEL_OPTIONS
    )
    provider_pb2_grpc.add_ResourceProviderServicer_to_server(monitor, server)
    port = server.add_insecure_port(address="0.0.0.0:0")
    server.start()
    sys.stdout.buffer.write(f"{port}\n".encode())
    try:
        while True:
            time.sleep(_ONE_DAY_IN_SECONDS)
    except KeyboardInterrupt:
        server.stop(0)

main()
