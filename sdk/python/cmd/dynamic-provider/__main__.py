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

import base64
import json
import asyncio
from concurrent import futures
import time

# from google.protobuf import struct_pb2
from pulumi.runtime import proto, rpc
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer
from pulumi.dynamic import ResourceProvider
from google.protobuf import empty_pb2
import grpc
import cloudpickle

_ONE_DAY_IN_SECONDS = 60 * 60 * 24

PROVIDER_KEY = "__provider"

def get_provider(props) -> ResourceProvider:
    byts = base64.b64decode(props[PROVIDER_KEY])
    return cloudpickle.loads(byts)()

class MyResourceProviderServicer(ResourceProviderServicer):
    def CheckConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('CheckConfig is not implemented by the dynamic provider')
        raise NotImplementedError('CheckConfig is not implemented by the dynamic provider')

    def DiffConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('DiffConfig is not implemented by the dynamic provider')
        raise NotImplementedError('DiffConfig is not implemented by the dynamic provider')

    def Invoke(self, request, context):
        raise NotImplementedError('unknown function ' % request.token)

    def Diff(self, request, context):
        """Diff checks what impacts a hypothetical update will have on the resource's properties.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Diff not implemented!')
        raise NotImplementedError('Diff not implemented!')

    def Update(self, request, context):
        """Update updates an existing resource with new values.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Update not implemented!')
        raise NotImplementedError('Update not implemented!')

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
        outs = result['outs']
        outs[PROVIDER_KEY] = props[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        outs_proto = loop.run_until_complete(rpc.serialize_properties(outs, {}))
        loop.close()

        fields = {'id': result['id'], 'properties': outs_proto}
        return proto.CreateResponse(**fields)
        
    def Check(self, request, context):
        olds = rpc.deserialize_properties(request.olds)
        news = rpc.deserialize_properties(request.news)
        if news[PROVIDER_KEY] == rpc.UNKNOWN:
            provider = get_provider(olds)
        else:
            provider = get_provider(news)

        result = provider.check(olds, news)
        inputs = result['inputs']
        # TODO failures
        # failures = result['failures']

        inputs[PROVIDER_KEY] = news[PROVIDER_KEY]
        
        loop = asyncio.new_event_loop()
        inputs_proto = loop.run_until_complete(rpc.serialize_properties(inputs, {}))
        loop.close()

        fields = {"inputs": inputs_proto}
        return proto.CheckResponse(**fields)

    def Configure(self, request, context):
        fields = {"acceptSecrets": False}
        return proto.ConfigureResponse(**fields)

    def GetPluginInfo(self, request, context):
        fields = {"version": "0.1.0"}
        return proto.PluginInfo(**fields)

    def Read(self, request, context):
        id_ = request.id
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props)
        result = provider.read(id_, props)
        outs = result['props']
        outs[PROVIDER_KEY] = props[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        outs_proto = loop.run_until_complete(rpc.serialize_properties(outs, {}))
        loop.close()

        fields = {'id': result['id'], 'properties': outs_proto}
        return proto.ReadResponse(**fields)

    def __init__(self):
        return

def main():
    monitor = MyResourceProviderServicer()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    provider_pb2_grpc.add_ResourceProviderServicer_to_server(monitor, server)
    port = server.add_insecure_port(address="0.0.0.0:0")
    server.start()
    print(port)
    try:
        while True:
            time.sleep(_ONE_DAY_IN_SECONDS)
    except KeyboardInterrupt:
        server.stop(0)

main()