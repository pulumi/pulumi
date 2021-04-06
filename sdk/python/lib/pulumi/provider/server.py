from concurrent import futures
from typing import List
import datetime
import sys
import time

import grpc
from pulumi.runtime import proto
from pulumi.provider.provider import Provider
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer

# import dill
# from google.protobuf import empty_pb2
# from pulumi.dynamic import ResourceProvider


# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [('grpc.max_receive_message_length', _MAX_RPC_MESSAGE_SIZE)]
_GRPC_WORKERS = 4


class ProviderServicer(ResourceProviderServicer):
    provider: Provider
    args: List[str]

    def Construct(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented: Construct - OVERRIDE!')
        raise NotImplementedError('Method not implemented: Construct - OVERRIDE!')

    def GetPluginInfo(self, request, context):
        return proto.PluginInfo(version=self.provider.version)

    def __init__(self, provider: Provider, args: List[str]):
        super(ProviderServicer, self).__init__()
        self.provider = provider
        self.args = args


def main(provider: Provider, args: List[str]):
    """For use as the `main` in programs that wrap a custom Provider
    implementation into a Pulumi-compatible gRPC server.

    :param provider: an instance of a Provider subclass

    :args: command line arguiments such as os.argv[2:]

    """

    servicer = ProviderServicer(provider, args)
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=_GRPC_WORKERS),
        options=_GRPC_CHANNEL_OPTIONS
    )
    provider_pb2_grpc.add_ResourceProviderServicer_to_server(servicer, server)
    port = server.add_insecure_port(address='0.0.0.0:0')
    server.start()
    sys.stdout.buffer.write(f'{port}\n'.encode())
    sys.stdout.buffer.flush()
    try:
        while True:
            time.sleep(datetime.timedelta(days=1).total_seconds())
    except KeyboardInterrupt:
        server.stop(0)
