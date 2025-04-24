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
from threading import Event, Lock
from typing import Any, Dict, Optional
import os
import sys
import dill
import grpc
from google.protobuf import empty_pb2
from pulumi.metadata import get_project
from pulumi.runtime._serialization import _deserialize
from pulumi.runtime import configure, proto, rpc, Settings
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer
from pulumi.dynamic import ResourceProvider, ConfigureRequest, Config

_ONE_DAY_IN_SECONDS = 60 * 60 * 24
PROVIDER_KEY = "__provider"

# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE)]


_PROVIDER_CACHE: Dict[str, ResourceProvider] = {}
_PROVIDER_LOCK = Lock()


# get_provider deserializes the provider from the string found in
# `props[PROVIDER_KEY]` and calls `provider.configure` with the config. The
# deserialized and configured provider is stored in `_PROVIDER_CACHE`. This
# guarantees that the provider is only deserialized and configured once per
# process.
def get_provider(props: Dict[str, Any], config: Dict[str, Any]) -> ResourceProvider:
    # Ensure Settings are configured in the thread that calls get_provider
    configure(
        Settings(
            project=os.environ.get("PULUMI_PROJECT", "project"),
            # `stack` and `organization` are the default values for Settings.
            # Ideally we'd like to get the actual values here and set them.
            stack="stack",
            organization="organization",
        )
    )
    providerStr = props[PROVIDER_KEY]
    provider: Optional[ResourceProvider] = _PROVIDER_CACHE.get(providerStr)
    if provider is None:
        # This is pesimistic locking, because if two different providers try to fetch at the same time they
        # serialise. But it means we don't create two instances of the same provider. Also looking at issues
        # like https://github.com/pulumi/pulumi/issues/14159 there may be resource contention in dill.loads,
        # that this locking strategy will reduce.
        with _PROVIDER_LOCK:
            provider = _PROVIDER_CACHE.get(providerStr)
            if provider is None:

                def deserialize() -> ResourceProvider:
                    byts = base64.b64decode(providerStr)
                    return dill.loads(byts)

                provider = _deserialize(deserialize)
                dyn_config = Config(raw_config=config, project_name=get_project())
                req = ConfigureRequest(config=dyn_config)
                provider.configure(req)
                _PROVIDER_CACHE[providerStr] = provider

    return provider


class DynamicResourceProviderServicer(ResourceProviderServicer):
    _config: Dict[str, Any] = {}

    def CheckConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("CheckConfig is not implemented by the dynamic provider")
        raise NotImplementedError(
            "CheckConfig is not implemented by the dynamic provider"
        )

    def DiffConfig(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("DiffConfig is not implemented by the dynamic provider")
        raise NotImplementedError(
            "DiffConfig is not implemented by the dynamic provider"
        )

    def Invoke(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Invoke is not implemented by the dynamic provider")
        raise NotImplementedError(f"unknown function {request.token}")

    def Diff(self, request, context):
        olds = rpc.deserialize_properties(request.olds, True)
        news = rpc.deserialize_properties(request.news, True)
        if news[PROVIDER_KEY] == rpc.UNKNOWN:
            provider = get_provider(olds, self._config)
        else:
            provider = get_provider(news, self._config)
        result = provider.diff(request.id, olds, news)
        fields = {}
        if result.changes is not None:
            if result.changes:
                fields["changes"] = proto.DiffResponse.DIFF_SOME
            else:
                fields["changes"] = proto.DiffResponse.DIFF_NONE
        else:
            fields["changes"] = proto.DiffResponse.DIFF_UNKNOWN
        if result.replaces is not None:
            fields["replaces"] = result.replaces
        if result.delete_before_replace is not None:
            fields["deleteBeforeReplace"] = result.delete_before_replace
        return proto.DiffResponse(**fields)

    def Update(self, request, context):
        olds = rpc.deserialize_properties(request.olds)
        news = rpc.deserialize_properties(request.news)
        provider = get_provider(news, self._config)

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
        provider = get_provider(props, self._config)
        provider.delete(id_, props)
        return empty_pb2.Empty()

    def Cancel(self, request, context):
        return empty_pb2.Empty()

    def Create(self, request, context):
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props, self._config)
        result = provider.create(props)
        outs = result.outs if result.outs is not None else {}
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
            provider = get_provider(olds, self._config)
        else:
            provider = get_provider(news, self._config)

        result = provider.check(olds, news)
        inputs = result.inputs
        failures = result.failures

        inputs[PROVIDER_KEY] = news[PROVIDER_KEY]

        loop = asyncio.new_event_loop()
        inputs_proto = loop.run_until_complete(rpc.serialize_properties(inputs, {}))
        loop.close()

        failures_proto = [
            proto.CheckFailure(property=f.property, reason=f.reason) for f in failures
        ]

        fields = {"inputs": inputs_proto, "failures": failures_proto}
        return proto.CheckResponse(**fields)

    def Configure(self, request, context):
        # Get the configuration from the request and store it. When
        # deserializing dynamic providers, we will call the provider's
        # `configure` method with this configuration.
        config = rpc.deserialize_properties(request.args)
        config = {k: rpc.unwrap_rpc_secret(v) for k, v in config.items()}
        self._config = config
        fields = {"acceptSecrets": False}
        return proto.ConfigureResponse(**fields)

    def GetPluginInfo(self, request, context):
        fields = {"version": "0.1.0"}
        return proto.PluginInfo(**fields)

    def GetSchema(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("GetSchema is not implemented by the dynamic provider")
        raise NotImplementedError(
            "GetSchema is not implemented by the dynamic provider"
        )

    def Read(self, request, context):
        id_ = request.id
        props = rpc.deserialize_properties(request.properties)
        provider = get_provider(props, self._config)
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
        options=_GRPC_CHANNEL_OPTIONS,
    )
    provider_pb2_grpc.add_ResourceProviderServicer_to_server(monitor, server)
    port = server.add_insecure_port(address="127.0.0.1:0")
    server.start()
    sys.stdout.buffer.write(f"{port}\n".encode())
    try:
        Event().wait()
    except KeyboardInterrupt:
        server.stop(0)


main()
