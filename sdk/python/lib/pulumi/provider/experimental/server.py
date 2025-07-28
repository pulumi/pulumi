# Copyright 2025, Pulumi Corporation.
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
import traceback

import grpc
import grpc.aio

from google.protobuf import empty_pb2

from pulumi.resource import (
    ProviderResource,
    Resource,
    DependencyResource,
    DependencyProviderResource,
    _parse_resource_reference,
)
from pulumi.runtime import known_types, proto, rpc
from pulumi.runtime.proto import (
    provider_pb2_grpc,
    ResourceProviderServicer,
    status_pb2,
    errors_pb2,
)
import pulumi.runtime.proto
import pulumi.provider.experimental.provider as provider
from pulumi.provider.experimental.property_value import PropertyValue
from pulumi.runtime.stack import wait_for_rpcs
import pulumi
import pulumi.resource
import pulumi.runtime.config
import pulumi.runtime.settings
from pulumi.errors import (
    InputPropertiesError,
    InputPropertyError,
    InputPropertyErrorDetails,
)

# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE)]


class ComponentInitError(Exception):
    """
    ComponentInitError signals an error raised from within the __init__ method
    of a component. This allows us to distinguish between a user error and a
    system error.
    """

    def __init__(self, inner: Exception) -> None:
        super().__init__(str(inner))
        self.inner = inner


class ProviderServicer(ResourceProviderServicer):
    """Implements a subset of `ResourceProvider` methods to support
    `Construct` and other methods invoked by the engine when the user
    program creates a remote `ComponentResource` (with `remote=true`
    in the constructor).

    See `ResourceProvider` defined in `provider.proto`.

    """

    _version: str
    _args: List[str]
    _provider: provider.Provider
    _engine_address: str
    _lock: asyncio.Lock

    def __init__(
        self,
        args: List[str],
        version: str,
        provider: provider.Provider,
        engine_address: str,
    ):
        super().__init__()
        self._args = args
        self._version = version
        self._provider = provider
        self._engine_address = engine_address
        self._lock = asyncio.Lock()

    async def GetPluginInfo(self, request, context) -> proto.PluginInfo:
        return proto.PluginInfo(version=self._version)

    def create_grpc_invalid_properties_status(
        self, message: str, errors: Optional[List[InputPropertyErrorDetails]]
    ):
        status = grpc.Status()  # type: ignore[attr-defined]
        # We don't care about the exact status code here, since they are pretty web centric, and don't
        # necessarily make sense in this context.  Pick one that's close enough.
        # type: ignore
        status.code = grpc.StatusCode.INVALID_ARGUMENT.value[0]  # type: ignore[index]
        status.details = message

        if errors is not None:
            s = status_pb2.Status()  # type: ignore[attr-defined]
            # This code needs to match the code above.
            s.code = grpc.StatusCode.INVALID_ARGUMENT.value[0]  # type: ignore[index]
            s.message = message

            error_details = errors_pb2.InputPropertiesError()
            for error in errors:
                property_error = errors_pb2.InputPropertiesError.PropertyError()
                property_error.property_path = error["property_path"]
                property_error.reason = error["reason"]
                error_details.errors.append(property_error)

            details_container = s.details.add()
            details_container.Pack(error_details)

            status.trailing_metadata = (
                ("grpc-status-details-bin", s.SerializeToString()),
            )

        return status

    async def Construct(
        self, request: proto.ConstructRequest, context
    ) -> proto.ConstructResponse:
        # Calls to `Construct` and `Call` are serialized because they currently modify globals. When we are able to
        # avoid modifying globals, we can remove the locking.
        await self._lock.acquire()
        try:
            return await self._construct(request, context)
        except Exception as e:  # noqa
            if isinstance(e, InputPropertiesError):
                status = self.create_grpc_invalid_properties_status(e.message, e.errors)
                await context.abort_with_status(status)
                # We already aborted at this point
                raise
            elif isinstance(e, InputPropertyError):
                status = self.create_grpc_invalid_properties_status(
                    "", [{"property_path": e.property_path, "reason": e.reason}]
                )
                await context.abort_with_status(status)
                # We already aborted at this point
                raise
            else:
                if isinstance(e, ComponentInitError):
                    stack = traceback.extract_tb(e.inner.__traceback__)[:]
                    # Drop the internal frame for `self._construct`.
                    stack = stack[1:]
                else:
                    stack = traceback.extract_tb(e.__traceback__)[:]
                pretty_stack = "".join(traceback.format_list(stack))
                raise Exception(f"{str(e)}:\n{pretty_stack}")
        finally:
            self._lock.release()

    async def _construct(
        self, request: proto.ConstructRequest, context
    ) -> proto.ConstructResponse:
        assert isinstance(
            request, proto.ConstructRequest
        ), f"request is not ConstructRequest but is {type(request)} instead"

        organization = request.organization if request.organization else "organization"
        pulumi.runtime.settings.reset_options(
            organization=organization,
            project=_empty_as_none(request.project),
            stack=_empty_as_none(request.stack),
            parallel=_zero_as_none(request.parallel),
            engine_address=self._engine_address,
            monitor_address=_empty_as_none(request.monitorEndpoint),
            preview=request.dryRun,
        )
        await pulumi.runtime.settings._load_monitor_feature_support()

        pulumi.runtime.config.set_all_config(
            dict(request.config), list(request.configSecretKeys)
        )
        inputs = PropertyValue.unmarshal_map(request.inputs)
        # Add the input dependencies to the inputs property values
        for k, v in inputs.items():
            deps = request.inputDependencies.get(k)
            if deps is not None:
                inputs[k] = v.with_dependencies(v.dependencies.union(deps.urns))

        result = await self._provider.construct(
            provider.ConstructRequest(
                name=request.name,
                resource_type=request.type,
                inputs=inputs,
                options=self._construct_options(request),
            )
        )

        response = self._construct_response(result)

        # Wait for outstanding RPCs such as more provider Construct
        # calls. This can happen if i.e. provider creates child
        # resources but does not await their URN promises.
        #
        # Do not await all tasks as that starts hanging waiting for
        # indefinite grpc.aio servier tasks.
        await wait_for_rpcs(await_all_outstanding_tasks=False)

        return response

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

    def _construct_response(
        self, result: provider.ConstructResponse
    ) -> proto.ConstructResponse:
        property_deps: Dict[str, Set[str]] = {}
        # Get the property dependencies from the result
        for k, v in result.state.items():
            if k in ["id", "urn"]:
                continue
            property_deps[k] = result.state[k].all_dependencies()

        state = PropertyValue.marshal_map(
            {k: v for k, v in result.state.items() if k not in ["id", "urn"]}
        )

        deps: Dict[str, proto.ConstructResponse.PropertyDependencies] = {}
        for k, urns in property_deps.items():
            deps[k] = proto.ConstructResponse.PropertyDependencies(urns=urns)

        return proto.ConstructResponse(
            urn=result.urn, state=state, stateDependencies=deps
        )

    async def Call(self, request: proto.CallRequest, context):
        # Calls to `Construct` and `Call` are serialized because they currently modify globals. When we are able to
        # avoid modifying globals, we can remove the locking.
        await self._lock.acquire()
        try:
            return await self._call(request, context)
        except InputPropertiesError as e:
            status = self.create_grpc_invalid_properties_status(e.message, e.errors)
            await context.abort_with_status(status)
            # We already aborted at this point
            raise
        except InputPropertyError as e:
            status = self.create_grpc_invalid_properties_status(
                "", [{"property_path": e.property_path, "reason": e.reason}]
            )
            await context.abort_with_status(status)
            # We already aborted at this point
            raise
        finally:
            self._lock.release()

    async def _call(self, request: proto.CallRequest, context):
        assert isinstance(
            request, proto.CallRequest
        ), f"request is not CallRequest but is {type(request)} instead"

        organization = request.organization if request.organization else "organization"
        pulumi.runtime.settings.reset_options(
            organization=organization,
            project=_empty_as_none(request.project),
            stack=_empty_as_none(request.stack),
            parallel=_zero_as_none(request.parallel),
            engine_address=self._engine_address,
            monitor_address=_empty_as_none(request.monitorEndpoint),
            preview=request.dryRun,
        )

        pulumi.runtime.config.set_all_config(
            dict(request.config),
            list(request.configSecretKeys),
        )

        args = PropertyValue.unmarshal_map(request.args)
        # Add the input dependencies to the args property values
        for k, v in args.items():
            deps = request.argDependencies.get(k)
            if deps is not None:
                args[k] = v.with_dependencies(v.dependencies.union(deps.urns))

        result = await self._provider.call(
            provider.CallRequest(tok=request.tok, args=args)
        )

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

    async def _call_response(self, result: provider.CallResponse) -> proto.CallResponse:
        property_deps: Dict[str, Set[str]] = {}
        # Get the property dependencies from the result
        for k, v in result.return_value.items():
            property_deps[k] = v.all_dependencies()

        return_value = PropertyValue.marshal_map(result.return_value)

        deps: Dict[str, proto.CallResponse.ReturnDependencies] = {}
        for k, urns in property_deps.items():
            deps[k] = proto.CallResponse.ReturnDependencies(urns=urns)

        failures = None
        if result.failures:
            failures = [
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in result.failures
            ]
        resp = proto.CallResponse(returnDependencies=deps, failures=failures)
        # Since `return` is a keyword, we need to use getattr: https://developers.google.com/protocol-buffers/docs/reference/python-generated#keyword-conflicts
        getattr(resp, "return").CopyFrom(return_value)
        return resp

    async def Parameterize(
        self, request: proto.ParameterizeRequest, context
    ) -> proto.ParameterizeResponse:
        which = request.WhichOneof("parameters")
        parameters: provider.Parameters
        if which == "args":
            parameters = provider.ParametersArgs(list(request.args.args))
        elif which == "value":
            parameters = provider.ParametersValue(
                name=request.value.name,
                version=request.value.version,
                value=request.value.value,
            )
        else:
            raise ValueError("ParameterizeRequest must contain either args or value")

        resp = await self._provider.parameterize(
            provider.ParameterizeRequest(parameters=parameters)
        )

        return proto.ParameterizeResponse(
            name=resp.name,
            version=resp.version,
        )

    async def Invoke(
        self, request: proto.InvokeRequest, context
    ) -> proto.InvokeResponse:
        resp = await self._provider.invoke(
            provider.InvokeRequest(
                request.tok,
                PropertyValue.unmarshal_map(request.args),
            )
        )
        # Since `return` is a keyword, we need to pass the args to `InvokeResponse` using a dictionary.
        ret: Dict[str, Any] = {
            "return": PropertyValue.marshal_map(resp.return_value)
            if resp.return_value
            else None,
        }
        if resp.failures:
            ret["failures"] = [
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in resp.failures
            ]
        return proto.InvokeResponse(**ret)

    async def GetSchema(
        self, request: proto.GetSchemaRequest, context
    ) -> proto.GetSchemaResponse:
        resp = await self._provider.get_schema(
            provider.GetSchemaRequest(
                request.version,
                _empty_as_none(request.subpackage_name),
                _empty_as_none(request.subpackage_version),
            )
        )
        return proto.GetSchemaResponse(schema=resp.schema or "")

    async def CheckConfig(self, request: pulumi.runtime.proto.CheckRequest, context):
        resp = await self._provider.check_config(
            provider.CheckRequest(
                request.urn,
                PropertyValue.unmarshal_map(request.olds),
                PropertyValue.unmarshal_map(request.news),
                request.randomSeed,
            )
        )
        return proto.CheckResponse(
            inputs=PropertyValue.marshal_map(resp.inputs)
            if resp.inputs is not None
            else None,
            failures=[
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in resp.failures
            ]
            if resp.failures is not None
            else None,
        )

    async def DiffConfig(self, request, context):
        resp = await self._provider.diff_config(
            provider.DiffRequest(
                request.urn,
                request.id,
                PropertyValue.unmarshal_map(request.olds),
                PropertyValue.unmarshal_map(request.news),
                request.ignoreChanges,
            )
        )
        return proto.DiffResponse(
            replaces=resp.replaces,
            stables=resp.stables,
            deleteBeforeReplace=resp.delete_before_replace,
            changes=resp.changes,
            diffs=resp.diffs,
            detailedDiff={
                k: proto.PropertyDiff(kind=v.kind, inputDiff=v.inputDiff)
                for k, v in resp.detailed_diff.items()
            },
            hasDetailedDiff=True,
        )

    async def Configure(
        self, request: proto.ConfigureRequest, context
    ) -> proto.ConfigureResponse:
        resp = await self._provider.configure(
            provider.ConfigureRequest(
                dict(request.variables),
                PropertyValue.unmarshal_map(request.args),
                request.acceptSecrets,
                request.acceptResources,
            )
        )
        return proto.ConfigureResponse(
            acceptSecrets=resp.accept_secrets,
            acceptResources=resp.accept_resources,
            acceptOutputs=resp.accept_outputs,
            supportsPreview=resp.supports_preview,
        )

    async def Check(self, request: proto.CheckRequest, context):
        resp = await self._provider.check(
            provider.CheckRequest(
                request.urn,
                PropertyValue.unmarshal_map(request.olds),
                PropertyValue.unmarshal_map(request.news),
                request.randomSeed,
            )
        )

        failures = None
        if resp.failures:
            failures = [
                proto.CheckFailure(property=f.property, reason=f.reason)
                for f in resp.failures
            ]

        return proto.CheckResponse(
            inputs=PropertyValue.marshal_map(resp.inputs)
            if resp.inputs is not None
            else None,
            failures=failures,
        )

    async def Diff(self, request: proto.DiffRequest, context):
        resp = await self._provider.diff(
            provider.DiffRequest(
                request.urn,
                request.id,
                PropertyValue.unmarshal_map(request.olds),
                PropertyValue.unmarshal_map(request.news),
                list(request.ignoreChanges),
            )
        )

        changes = proto.DiffResponse.DIFF_UNKNOWN
        if resp.changes is not None:
            if resp.changes:
                changes = proto.DiffResponse.DIFF_SOME
            else:
                changes = proto.DiffResponse.DIFF_NONE

        tokind = {
            provider.PropertyDiffKind.ADD: proto.PropertyDiff.ADD,
            provider.PropertyDiffKind.ADD_REPLACE: proto.PropertyDiff.ADD_REPLACE,
            provider.PropertyDiffKind.DELETE: proto.PropertyDiff.DELETE,
            provider.PropertyDiffKind.DELETE_REPLACE: proto.PropertyDiff.DELETE_REPLACE,
            provider.PropertyDiffKind.UPDATE: proto.PropertyDiff.UPDATE,
            provider.PropertyDiffKind.UPDATE_REPLACE: proto.PropertyDiff.UPDATE_REPLACE,
        }

        return proto.DiffResponse(
            replaces=resp.replaces,
            stables=resp.stables,
            deleteBeforeReplace=resp.delete_before_replace,
            changes=changes,
            diffs=resp.diffs,
            detailedDiff={
                k: proto.PropertyDiff(kind=tokind[v.kind], inputDiff=v.input_diff)
                for k, v in resp.detailed_diff.items()
            },
            hasDetailedDiff=True,
        )

    async def Create(self, request, context):
        resp = await self._provider.create(
            provider.CreateRequest(
                request.urn,
                PropertyValue.unmarshal_map(request.properties),
                request.timeout,
                request.preview,
            )
        )
        return proto.CreateResponse(
            id=resp.resource_id,
            properties=PropertyValue.marshal_map(resp.properties),
        )

    async def Update(self, request, context):
        resp = await self._provider.update(
            provider.UpdateRequest(
                request.urn,
                request.id,
                PropertyValue.unmarshal_map(request.olds),
                PropertyValue.unmarshal_map(request.news),
                timeout=request.timeout,
                ignore_changes=request.ignoreChanges,
                preview=request.preview,
            )
        )
        return proto.UpdateResponse(
            properties=PropertyValue.marshal_map(resp.properties),
        )

    async def Delete(self, request, context):
        await self._provider.delete(
            provider.DeleteRequest(
                request.urn,
                request.id,
                PropertyValue.unmarshal_map(request.properties),
                timeout=request.timeout,
            )
        )
        return empty_pb2.Empty()

    async def Read(self, request, context):
        resp = await self._provider.read(
            provider.ReadRequest(
                request.urn,
                request.id,
                PropertyValue.unmarshal_map(request.properties),
                PropertyValue.unmarshal_map(request.inputs),
            )
        )
        return proto.ReadResponse(
            id=resp.resource_id,
            properties=PropertyValue.marshal_map(resp.properties),
            inputs=PropertyValue.marshal_map(resp.inputs),
        )


def main(args: List[str], version: str, provider: provider.Provider) -> None:
    """For use as the `main` in programs that wrap a custom Provider
    implementation into a Pulumi-compatible gRPC server.

    :param provider: an instance of a Provider subclass

    :args: command line arguments such as os.argv[1:]

    """

    argp = argparse.ArgumentParser(description="Pulumi provider plugin (gRPC server)")
    argp.add_argument("engine", help="Pulumi engine address")
    argp.add_argument("--logflow", action="store_true", help="Currently ignored")
    argp.add_argument("--logtostderr", action="store_true", help="Currently ignored")

    known_args, _ = argp.parse_known_args()
    engine_address: str = known_args.engine

    async def serve() -> None:
        server = grpc.aio.server(options=_GRPC_CHANNEL_OPTIONS)
        servicer = ProviderServicer(
            args, version, provider, engine_address=engine_address
        )
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


T = TypeVar("T")


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
