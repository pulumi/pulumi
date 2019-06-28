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
import asyncio
from concurrent import futures
import time

# from google.protobuf import struct_pb2
from pulumi.runtime import proto, rpc
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer
from pulumi.dynamic import ResourceProvider
import grpc
import cloudpickle

_ONE_DAY_IN_SECONDS = 60 * 60 * 24

PROVIDER_KEY = "__provider"

def get_provider(props) -> ResourceProvider:
    byts = base64.b64decode(props[PROVIDER_KEY])
    return cloudpickle.loads(byts)()

class MyResourceProviderServicer(ResourceProviderServicer):
    def CheckConfig(self, request, context):
        """CheckConfig validates the configuration for this resource provider.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('CheckConfig not implemented!')
        raise NotImplementedError('CheckConfig not implemented!')

    def DiffConfig(self, request, context):
        """DiffConfig checks the impact a hypothetical change to this provider's configuration will have on the provider.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('DiffConfig not implemented!')
        raise NotImplementedError('DiffConfig not implemented!')

    def Invoke(self, request, context):
        """Invoke dynamically executes a built-in function in the provider.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Invoke not implemented!')
        raise NotImplementedError('Invoke not implemented!')

    def Diff(self, request, context):
        """Diff checks what impacts a hypothetical update will have on the resource's properties.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Diff not implemented!')
        raise NotImplementedError('Diff not implemented!')

    def Create(self, request, context):
        """Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
        must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Create not implemented!')
        raise NotImplementedError('Create not implemented!')

    def Read(self, request, context):
        """Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
        identify the resource; this is typically just the resource ID, but may also include some properties.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Read not implemented!')
        raise NotImplementedError('Read not implemented!')

    def Update(self, request, context):
        """Update updates an existing resource with new values.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Update not implemented!')
        raise NotImplementedError('Update not implemented!')

    def Delete(self, request, context):
        """Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Delete not implemented!')
        raise NotImplementedError('Delete not implemented!')

    def Cancel(self, request, context):
        """Cancel signals the provider to abort all outstanding resource operations.
        """
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Cancel not implemented!')
        raise NotImplementedError('Cancel not implemented!')

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