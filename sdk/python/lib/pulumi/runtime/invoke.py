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
import traceback

from typing import Any, Dict, List, NamedTuple, Optional, Set, TYPE_CHECKING
import grpc

from .. import log
from .. import _types
from ..invoke import InvokeOptions
from ..runtime.proto import provider_pb2
from . import rpc
from .rpc_manager import RPC_MANAGER
from .settings import get_monitor, grpc_error_to_exception, handle_grpc_error
from .sync_await import _sync_await

if TYPE_CHECKING:
    from .. import Resource, Inputs, Output

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


def call(tok: str, props: 'Inputs', res: Optional['Resource'] = None, typ: Optional[type] = None) -> 'Output[Any]':
    """
    call dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s).
    """
    log.debug(f"Calling function: tok={tok}")

    if typ and not _types.is_output_type(typ):
        raise TypeError("Expected typ to be decorated with @output_type")

    # Setup the futures for the output.
    resolve_value: 'asyncio.Future' = asyncio.Future()
    resolve_is_known: 'asyncio.Future[bool]' = asyncio.Future()
    resolve_is_secret: 'asyncio.Future[bool]' = asyncio.Future()
    resolve_deps: 'asyncio.Future[Set[Resource]]' = asyncio.Future()

    from .. import Output  # pylint: disable=import-outside-toplevel
    out = Output(resolve_deps, resolve_value, resolve_is_known, resolve_is_secret)

    async def do_call():
        try:
            # Construct a provider reference from the given provider, if one is available on the resource.
            provider_ref, version = None, ""
            if res is not None:
                if res._provider is not None:
                    provider_urn = await res._provider.urn.future()
                    provider_id = (await res._provider.id.future()) or rpc.UNKNOWN
                    provider_ref = f"{provider_urn}::{provider_id}"
                    log.debug(f"Call using provider {provider_ref}")
                version = res._version or ""

            monitor = get_monitor()

            # Serialize out all props to their final values. In doing so, we'll also collect all the Resources pointed to
            # by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
            property_dependencies_resources: Dict[str, List['Resource']] = {}
            # We keep output values when serializing inputs for call.
            inputs = await rpc.serialize_properties(props, property_dependencies_resources, keep_output_values=True)

            property_dependencies = {}
            for key, property_deps in property_dependencies_resources.items():
                urns = set()
                for dep in property_deps:
                    urn = await dep.urn.future()
                    urns.add(urn)
                property_dependencies[key] = provider_pb2.CallRequest.ArgumentDependencies(urns=list(urns))

            req = provider_pb2.CallRequest(
                tok=tok,
                args=inputs,
                argDependencies=property_dependencies,
                provider=provider_ref,
                version=version,
            )

            def do_rpc_call():
                try:
                    return monitor.Call(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
            if resp is None:
                return

            if resp.failures:
                raise Exception(f"call of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})")

            log.debug(f"Call successful: tok={tok}")

            value = None
            is_known = True
            is_secret = False
            deps: Set['Resource'] = set()
            ret_obj = getattr(resp, "return")
            if ret_obj:
                deserialized = rpc.deserialize_properties(ret_obj)
                is_known = not rpc.contains_unknowns(deserialized)

                # Keep track of whether we need to mark the resulting output a secret,
                # and unwrap each individual value.
                for k, v in deserialized.items():
                    if rpc.is_rpc_secret(v):
                        is_secret = True
                        deserialized[k] = rpc.unwrap_rpc_secret(v)

                # Combine the individual dependencies into a single set of dependency resources.
                rpc_deps = resp.returnDependencies
                deps_urns: Set[str] = {urn for v in rpc_deps.values() for urn in v.urns} if rpc_deps else set()
                from ..resource import DependencyResource  # pylint: disable=import-outside-toplevel
                deps = set(map(DependencyResource, deps_urns))

                if is_known:
                    # If typ is not None, call translate_output_properties to instantiate any output types.
                    value = rpc.translate_output_properties(deserialized, lambda p: p, typ) if typ else deserialized

            resolve_value.set_result(value)
            resolve_is_known.set_result(is_known)
            resolve_is_secret.set_result(is_secret)
            resolve_deps.set_result(deps)

        except Exception as exn:
            log.debug(f"exception when preparing or executing rpc: {traceback.format_exc()}")
            resolve_value.set_exception(exn)
            resolve_is_known.set_exception(exn)
            resolve_is_secret.set_exception(exn)
            resolve_deps.set_result(set())
            raise

    asyncio.ensure_future(RPC_MANAGER.do_rpc("call", do_call)())

    return out
