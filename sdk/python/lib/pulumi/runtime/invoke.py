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
import sys
from typing import Any, Awaitable
import grpc

from ..output import Inputs
from ..invoke import InvokeOptions
from .. import log
from .settings import get_monitor
from ..runtime.proto import provider_pb2
from . import rpc
from .rpc_manager import RPC_MANAGER


def invoke(tok: str, props: Inputs, opts: InvokeOptions = None) -> Awaitable[Any]:
    """
    invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    log.debug(f"Invoking function: tok={tok}")
    if opts is None:
        opts = InvokeOptions()

    async def do_invoke():
        # If a parent was provided, but no provider was provided, use the parent's provider if one was specified.
        if opts.parent is not None and opts.provider is None:
            opts.provider = opts.parent.get_provider(tok)

        # Construct a provider reference from the given provider, if one was provided to us.
        provider_ref = None
        if opts.provider is not None:
            provider_urn = await opts.provider.urn.future()
            provider_id = (await opts.provider.id.future()) or rpc.UNKNOWN
            provider_ref = f"{provider_urn}::{provider_id}"
            log.debug(f"Invoke using provider {provider_ref}")

        monitor = get_monitor()
        inputs = await rpc.serialize_properties(props, {})
        version = opts.version or ""
        log.debug(f"Invoking function prepared: tok={tok}")
        req = provider_pb2.InvokeRequest(tok=tok, args=inputs, provider=provider_ref, version=version)

        def do_invoke():
            try:
                return monitor.Invoke(req)
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

                details = exn.details()
            raise Exception(details)

        resp = await asyncio.get_event_loop().run_in_executor(None, do_invoke)
        log.debug(f"Invoking function completed successfully: tok={tok}")
        # If the invoke failed, raise an error.
        if resp.failures:
            raise Exception(f"invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})")

        # Otherwise, return the output properties.
        ret_obj = getattr(resp, 'return')
        if ret_obj:
            return rpc.deserialize_properties(ret_obj)
        return {}

    return asyncio.ensure_future(RPC_MANAGER.do_rpc("invoke", do_invoke)())
