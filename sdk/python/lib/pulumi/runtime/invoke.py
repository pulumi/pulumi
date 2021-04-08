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
import os
from typing import Optional, TYPE_CHECKING
import grpc

from .. import log
from .. import _types
from ..invoke import InvokeOptions
from ..runtime.proto import provider_pb2
from . import rpc
from .rpc_manager import RPC_MANAGER
from .settings import get_monitor, grpc_error_to_exception
from .sync_await import _sync_await

if TYPE_CHECKING:
    from .. import Inputs

# This setting overrides a hardcoded maximum protobuf size in the python protobuf bindings. This avoids deserialization
# exceptions on large gRPC payloads, but makes it possible to use enough memory to cause an OOM error instead [1].
# Note: We hit the default maximum protobuf size in practice when processing Kubernetes CRDs [2]. If this setting ends
# up causing problems, it should be possible to work around it with more intelligent resource chunking in the k8s
# provider.
#
# [1] https://github.com/protocolbuffers/protobuf/blob/0a59054c30e4f0ba10f10acfc1d7f3814c63e1a7/python/google/protobuf/pyext/message.cc#L2017-L2024
# [2] https://github.com/pulumi/pulumi-kubernetes/issues/984
#
# This setting requires a platform-specific and python version-specific .so file called
# `_message.cpython-[py-version]-[platform].so`, which is not present in situations when a new python version is
# released but the corresponding dist wheel has not been. So, we wrap the import in a try/except to avoid breaking all
# python programs using a new version.
try:
    from google.protobuf.pyext._message import SetAllowOversizeProtos  # pylint: disable-msg=E0611
    SetAllowOversizeProtos(True)
except ImportError:
    pass


class InvokeResult:
    """
    InvokeResult is a helper type that wraps a prompt value in an Awaitable.
    """
    def __init__(self, value):
        self.value = value

    # pylint: disable=using-constant-test
    def __await__(self):
        # We need __await__ to be an iterator, but we only want it to return one value. As such, we use
        # `if False: yield` to construct this.
        if False:
            yield self.value
        return self.value

    __iter__ = __await__


def invoke(tok: str, props: 'Inputs', opts: Optional[InvokeOptions] = None, typ: Optional[type] = None) -> InvokeResult:
    """
    invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    log.debug(f"Invoking function: tok={tok}")
    if opts is None:
        opts = InvokeOptions()

    if typ and not _types.is_output_type(typ):
        raise TypeError("Expected typ to be decorated with @output_type")

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
        accept_resources = not (os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper() in {"TRUE", "1"})
        log.debug(f"Invoking function prepared: tok={tok}")
        req = provider_pb2.InvokeRequest(
            tok=tok,
            args=inputs,
            provider=provider_ref,
            version=version,
            acceptResources=accept_resources,
        )

        def do_invoke():
            try:
                return monitor.Invoke(req), None
            except grpc.RpcError as exn:
                return None, grpc_error_to_exception(exn)

        resp, error = await asyncio.get_event_loop().run_in_executor(None, do_invoke)
        log.debug(f"Invoking function completed: tok={tok}, error={error}")

        # If the invoke failed, raise an error.
        if error is not None:
            return None, Exception(f"invoke of {tok} failed: {error}")
        if resp.failures:
            return None, Exception(f"invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})")

        # Otherwise, return the output properties.
        ret_obj = getattr(resp, 'return')
        if ret_obj:
            deserialized = rpc.deserialize_properties(ret_obj)
            # If typ is not None, call translate_output_properties to instantiate any output types.
            return rpc.translate_output_properties(deserialized, lambda prop: prop, typ) if typ else deserialized, None
        return None, None

    async def do_rpc():
        resp, exn = await RPC_MANAGER.do_rpc("invoke", do_invoke)()
        # If there was an RPC level exception, we will raise it. Note that this will also crash the
        # process because it will have been considered "unhandled". For semantic level errors, such
        # as errors from the data source itself, we return that as part of the returned tuple instead.
        if exn is not None:
            raise exn
        return resp

    # Run the RPC callback asynchronously and then immediately await it.
    # If there was a semantic error, raise it now, otherwise return the resulting value.
    invoke_result, invoke_error = _sync_await(asyncio.ensure_future(do_rpc()))
    if invoke_error is not None:
        raise invoke_error
    return InvokeResult(invoke_result)
