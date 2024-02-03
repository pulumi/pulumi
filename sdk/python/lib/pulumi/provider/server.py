# Copyright 2016-2021, Pulumi Corporation.
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

"""Define gRPC plumbing to expose a custom user-defined `Provider`
instance as a gRPC server so that it can be used as a Pulumi plugin.

"""

from typing import Dict, List, Set, Optional, TypeVar, Any, cast
import argparse
import asyncio
import sys

import grpc
import grpc.aio

from google.protobuf import struct_pb2
from pulumi.provider.provider import InvokeResult, Provider, CallResult, ConstructResult
from pulumi.resource import (
    ProviderResource,
    Resource,
    DependencyResource,
    DependencyProviderResource,
    _parse_resource_reference,
)
from pulumi.runtime import known_types, proto, rpc
from pulumi.runtime.proto import provider_pb2_grpc, ResourceProviderServicer
from pulumi.runtime.stack import wait_for_rpcs
import pulumi
import pulumi.resource
import pulumi.runtime.config
import pulumi.runtime.settings


# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE)]


class ProviderServicer(ResourceProviderServicer):
    """Implements a subset of `ResourceProvider` methods to support
    `Construct` and other methods invoked by the engine when the user
    program creates a remote `ComponentResource` (with `remote=true`
    in the constructor).

    See `ResourceProvider` defined in `provider.proto`.

    """

    engine_address: str
    provider: Provider
    args: List[str]
    lock: asyncio.Lock

    async def Construct(  # pylint: disable=invalid-overridden-method
        self, request: proto.ConstructRequest, context
    ) -> proto.ConstructResponse:
        # Calls to `Construct` and `Call` are serialized because they currently modify globals. When we are able to
        # avoid modifying globals, we can remove the locking.
        await self.lock.acquire()
        try:
            return await self._construct(request, context)
        finally:
            self.lock.release()

    async def _construct(
        self, request: proto.ConstructRequest, context
    ) -> proto.ConstructResponse:
        # pylint: disable=unused-argument
        assert isinstance(
            request, proto.ConstructRequest
        ), f"request is not ConstructRequest but is {type(request)} instead"

        organization = request.organization if request.organization else "organization"
        pulumi.runtime.settings.reset_options(
            organization=organization,
            project=_empty_as_none(request.project),
            stack=_empty_as_none(request.stack),
            parallel=_zero_as_none(request.parallel),
            engine_address=self.engine_address,
            monitor_address=_empty_as_none(request.monitorEndpoint),
            preview=request.dryRun,
        )

        pulumi.runtime.config.set_all_config(
            dict(request.config), list(request.configSecretKeys)
        )
        inputs = await self._construct_inputs(request.inputs, request.inputDependencies)

        result = self.provider.construct(
            name=request.name,
            resource_type=request.type,
            inputs=inputs,
            options=self._construct_options(request),
        )

        response = await self._construct_response(result)

        # Wait for outstanding RPCs such as more provider Construct
        # calls. This can happen if i.e. provider creates child
        # resources but does not await their URN promises.
        #
        # Do not await all tasks as that starts hanging waiting for
        # indefinite grpc.aio servier tasks.
        await wait_for_rpcs(await_all_outstanding_tasks=False)

        return response

    @staticmethod
    async def _construct_inputs(
        inputs: struct_pb2.Struct, input_dependencies: Any
    ) -> Dict[str, pulumi.Input[Any]]:
        def deps(key: str) -> Set[str]:
            return set(
                urn
                for urn in input_dependencies.get(
                    key, proto.ConstructRequest.PropertyDependencies()
                ).urns
            )

        return {
            k: await ProviderServicer._select_value(the_input, deps=deps(k))
            for k, the_input in rpc.deserialize_properties(
                inputs, keep_unknowns=True
            ).items()
        }

    @staticmethod
    async def _select_value(the_input: Any, deps: Set[str]) -> Any:
        is_secret = rpc.is_rpc_secret(the_input)

        # If the input isn't a secret and either doesn't have any dependencies, already contains Outputs (from
        # deserialized output values), or is a resource reference, then return it directly without wrapping it
        # as an output.
        if not is_secret and (
            len(deps) == 0
            or _contains_outputs(the_input)
            or await _is_resource_reference(the_input, deps)
        ):
            return the_input

        # Otherwise, wrap it as an output so we can handle secrets
        # and/or track dependencies.
        # Note: If the value is or contains an unknown value, the Output will mark its value as
        # unknown automatically, so we just pass true for is_known here.
        return pulumi.Output(
            resources=set(DependencyResource(urn) for urn in deps),
            future=_as_future(rpc.unwrap_rpc_secret(the_input)),
            is_known=_as_future(True),
            is_secret=_as_future(is_secret),
        )

    @staticmethod
    def _construct_options(request: proto.ConstructRequest) -> pulumi.ResourceOptions:
        parent = None
        if not _empty_as_none(request.parent):
            parent = DependencyResource(request.parent)
        return pulumi.ResourceOptions(
            aliases=list(request.aliases),
            depends_on=[DependencyResource(urn) for urn in request.dependencies],
            protect=request.protect,
            providers={
                pkg: _create_provider_resource(ref)
                for pkg, ref in request.providers.items()
            },
            parent=parent,
        )

    async def _construct_response(
        self, result: ConstructResult
    ) -> proto.ConstructResponse:
        urn = await pulumi.Output.from_input(result.urn).future()
        assert urn is not None

        # Note: property_deps is populated by rpc.serialize_properties.
        property_deps: Dict[str, List[pulumi.resource.Resource]] = {}
        state = await rpc.serialize_properties(
            inputs={k: v for k, v in result.state.items() if k not in ["id", "urn"]},
            property_deps=property_deps,
        )

        deps: Dict[str, proto.ConstructResponse.PropertyDependencies] = {}
        for k, resources in property_deps.items():
            urns = await asyncio.gather(*(r.urn.future() for r in resources))
            deps[k] = proto.ConstructResponse.PropertyDependencies(urns=urns)

        return proto.ConstructResponse(urn=urn, state=state, stateDependencies=deps)

    async def Call(
        self, request: proto.CallRequest, context
    ):  # pylint: disable=invalid-overridden-method
        # Calls to `Construct` and `Call` are serialized because they currently modify globals. When we are able to
        # avoid modifying globals, we can remove the locking.
        await self.lock.acquire()
        try:
            return await self._call(request, context)
        finally:
            self.lock.release()

    async def _call(self, request: proto.CallRequest, context):
        # pylint: disable=unused-argument
        assert isinstance(
            request, proto.CallRequest
        ), f"request is not CallRequest but is {type(request)} instead"

        organization = request.organization if request.organization else "organization"
        pulumi.runtime.settings.reset_options(
            organization=organization,
            project=_empty_as_none(request.project),
            stack=_empty_as_none(request.stack),
            parallel=_zero_as_none(request.parallel),
            engine_address=self.engine_address,
            monitor_address=_empty_as_none(request.monitorEndpoint),
            preview=request.dryRun,
        )

        pulumi.runtime.config.set_all_config(
            dict(request.config),
            list(request.configSecretKeys),
        )

        args = await self._call_args(request)

        result = self.provider.call(token=request.tok, args=args)

        response = await self._call_response(result)

        # Wait for outstanding RPCs such as more provider Construct
        # calls. This can happen if i.e. provider creates child
        # resources but does not await their URN promises.
        #
        # Do not await all tasks as that starts hanging waiting for
        # indefinite grpc.aio servier tasks.
        await wait_for_rpcs(await_all_outstanding_tasks=False)

        return response

    @staticmethod
    async def _call_args(request: proto.CallRequest) -> Dict[str, pulumi.Input[Any]]:
        def deps(key: str) -> Set[str]:
            return set(
                urn
                for urn in request.argDependencies.get(
                    key, proto.CallRequest.ArgumentDependencies()
                ).urns
            )

        return {
            k: await ProviderServicer._select_value(the_input, deps=deps(k))
            for k, the_input in
            # We need to keep_internal, to keep the `__self__` that would normally be filtered because
            # it starts with "__".
            rpc.deserialize_properties(
                request.args, keep_unknowns=True, keep_internal=True
            ).items()
        }

    async def _call_response(self, result: CallResult) -> proto.CallResponse:
        # Note: ret_deps is populated by rpc.serialize_properties.
        ret_deps: Dict[str, List[pulumi.resource.Resource]] = {}
        ret = await rpc.serialize_properties(
            inputs=result.outputs, property_deps=ret_deps
        )

        deps: Dict[str, proto.CallResponse.ReturnDependencies] = {}
        for k, resources in ret_deps.items():
            urns = await asyncio.gather(*(r.urn.future() for r in resources))
            deps[k] = proto.CallResponse.ReturnDependencies(urns=urns)

        failures = None
        if result.failures:
            failures = [
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in result.failures
            ]
        resp = proto.CallResponse(returnDependencies=deps, failures=failures)
        # Since `return` is a keyword, we need to use getattr: https://developers.google.com/protocol-buffers/docs/reference/python-generated#keyword-conflicts
        getattr(resp, "return").CopyFrom(ret)
        return resp

    async def _invoke_response(self, result: InvokeResult) -> proto.InvokeResponse:
        # Note: ret_deps is populated by rpc.serialize_properties but unused
        ret_deps: Dict[str, List[pulumi.resource.Resource]] = {}
        ret = await rpc.serialize_properties(
            inputs=result.outputs, property_deps=ret_deps
        )
        # Since `return` is a keyword, we need to pass the args to `InvokeResponse` using a dictionary.
        resp: Dict[str, Any] = {
            "return": ret,
        }
        if result.failures:
            resp["failures"] = [
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in result.failures
            ]
        return proto.InvokeResponse(**resp)

    async def Invoke(  # pylint: disable=invalid-overridden-method
        self, request: proto.InvokeRequest, context
    ) -> proto.InvokeResponse:
        args = rpc.deserialize_properties(
            request.args, keep_unknowns=False, keep_internal=False
        )
        result = self.provider.invoke(token=request.tok, args=args)
        response = await self._invoke_response(result)
        return response

    async def Configure(  # pylint: disable=invalid-overridden-method
        self, request, context
    ) -> proto.ConfigureResponse:
        return proto.ConfigureResponse(
            acceptSecrets=True, acceptResources=True, acceptOutputs=True
        )

    async def GetPluginInfo(  # pylint: disable=invalid-overridden-method
        self, request, context
    ) -> proto.PluginInfo:
        return proto.PluginInfo(version=self.provider.version)

    async def GetSchema(  # pylint: disable=invalid-overridden-method
        self, request: proto.GetSchemaRequest, context
    ) -> proto.GetSchemaResponse:
        if request.version != 0:
            raise Exception(f"unsupported schema version {request.version}")
        schema = self.provider.schema if self.provider.schema else "{}"
        return proto.GetSchemaResponse(schema=schema)

    def __init__(
        self, provider: Provider, args: List[str], engine_address: str
    ) -> None:
        super().__init__()
        self.provider = provider
        self.args = args
        self.engine_address = engine_address
        self.lock = asyncio.Lock()


def main(provider: Provider, args: List[str]) -> None:  # args not in use?
    """For use as the `main` in programs that wrap a custom Provider
    implementation into a Pulumi-compatible gRPC server.

    :param provider: an instance of a Provider subclass

    :args: command line arguiments such as os.argv[1:]

    """

    argp = argparse.ArgumentParser(description="Pulumi provider plugin (gRPC server)")
    argp.add_argument("engine", help="Pulumi engine address")
    argp.add_argument("--logflow", action="store_true", help="Currently ignored")
    argp.add_argument("--logtostderr", action="store_true", help="Currently ignored")

    known_args, _ = argp.parse_known_args()
    engine_address: str = known_args.engine

    async def serve() -> None:
        server = grpc.aio.server(options=_GRPC_CHANNEL_OPTIONS)
        servicer = ProviderServicer(provider, args, engine_address=engine_address)
        provider_pb2_grpc.add_ResourceProviderServicer_to_server(servicer, server)
        port = server.add_insecure_port(address="127.0.0.1:0")
        await server.start()
        sys.stdout.buffer.write(f"{port}\n".encode())
        sys.stdout.buffer.flush()
        await server.wait_for_termination()

    try:
        loop = asyncio.get_event_loop()
        try:
            loop.run_until_complete(serve())
        finally:
            loop.close()
    except KeyboardInterrupt:
        pass


T = TypeVar("T")  # pylint: disable=invalid-name


def _as_future(value: T) -> "asyncio.Future[T]":
    fut: "asyncio.Future[T]" = asyncio.Future()
    fut.set_result(value)
    return fut


def _empty_as_none(text: str) -> Optional[str]:
    return None if text == "" else text


def _zero_as_none(value: int) -> Optional[int]:
    return None if value == 0 else value


async def _is_resource_reference(the_input: Any, deps: Set[str]) -> bool:
    """
    Returns True if `the_input` is a Resource and only depends on itself.
    """
    return (
        known_types.is_resource(the_input)
        and len(deps) == 1
        and next(iter(deps)) == await cast(Resource, the_input).urn.future()
    )


def _contains_outputs(the_input: Any) -> bool:
    """
    Returns true if the input contains Outputs (deeply).
    """
    if known_types.is_output(the_input):
        return True

    if isinstance(the_input, list):
        for e in the_input:
            if _contains_outputs(e):
                return True
    elif isinstance(the_input, dict):
        for k in the_input:
            if _contains_outputs(the_input[k]):
                return True

    return False


def _create_provider_resource(ref: str) -> ProviderResource:
    """
    Rehydrate the provider reference into a registered ProviderResource,
    otherwise return an instance of DependencyProviderResource.
    """
    urn, _ = _parse_resource_reference(ref)
    urn_parts = pulumi.urn._parse_urn(urn)
    resource_package = rpc.get_resource_package(urn_parts.typ_name, version="")
    if resource_package is not None:
        return cast(
            ProviderResource,
            resource_package.construct_provider(urn_parts.urn_name, urn_parts.typ, urn),
        )

    return DependencyProviderResource(ref)
