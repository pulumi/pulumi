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
from typing import Any

from ..output import Inputs
from .. import log
from .settings import get_monitor
from ...runtime.proto import provider_pb2
from . import rpc


async def invoke(tok: str, props: Inputs) -> Any:
    """
    invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    log.debug(f"Invoking function: tok={tok}")

    # TODO(swgillespie, first class providers) here
    monitor = get_monitor()
    inputs = await rpc.serialize_properties(props, [])
    log.debug(f"Invoking function prepared: tok={tok}")
    req = provider_pb2.InvokeRequest(tok=tok, args=inputs)
    future: asyncio.Future = asyncio.Future()

    def do_invoke():
        log.debug(f"Invoking function beginning: tok={tok}")
        try:
            resp = monitor.Invoke(req)
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

            future.set_exception(Exception(exn.details()))
            return

        log.debug(f"Invoking function completed: tok={tok}")
        # If the invoke failed, raise an error.
        if resp.failures:
            exn = Exception(f"invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})")
            future.set_exception(exn)
            return

        # Otherwise, return the output properties.
        ret_obj = getattr(resp, 'return')
        if ret_obj:
            result = rpc.deserialize_properties(ret_obj)
            future.set_result(result)
            return

        future.set_result({})

    asyncio.get_event_loop().call_soon(do_invoke)
    return future
