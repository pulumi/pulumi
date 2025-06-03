# Copyright 2016-2025, Pulumi Corporation.
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
from typing import (
    TYPE_CHECKING,
    Any,
    Awaitable,
    Dict,
    List,
    Optional,
    Set,
    Tuple,
    Union,
)

import grpc
from google.protobuf import struct_pb2

from semver import VersionInfo

from .. import _types, log
from ..invoke import InvokeOptions, InvokeOutputOptions
from ..runtime.proto import resource_pb2
from ..resource import CustomResource
from . import rpc
from ._depends_on import _resolve_depends_on_urns, _resolve_depends_on
from .settings import (
    _get_rpc_manager,
    get_monitor,
    grpc_error_to_exception,
    handle_grpc_error,
)
from .sync_await import _sync_await

if TYPE_CHECKING:
    from .. import Inputs, Output, Resource


def _requires_legacy_none_return_for_empty_struct(
    ret_obj: struct_pb2.Struct, tok: str, version: str
) -> bool:
    """
    To avoid breaking older versions of the Kubernetes Python SDK, we maintain the legacy behavior of
    returning None instead of an empty dict for empty results if the invoke is for a version of the
    Pulumi Kubernetes SDK that can't handle empty dicts.
    See https://github.com/pulumi/pulumi/issues/14508.
    """

    # If re_obj is truthy or the token is not one of the known Kubernetes tokens, we don't need the legacy
    # behavior.
    if ret_obj or tok not in {
        "kubernetes:yaml:decode",
        "kubernetes:helm:template",
        "kubernetes:kustomize:directory",
    }:
        return False

    # If we have a version and it's less than or equal to pulumi-kubernetes 4.5.4, we need the legacy behavior;
    # otherwise, we don't because later versions can handle the new behavior correctly.
    if version:
        k8s_ver = VersionInfo(4, 5, 4)
        try:
            ver = VersionInfo.parse(version)
        except Exception as ex:  # noqa: BLE001 catch blind exception
            log.debug(f"Failed to parse version {version} as semver: {ex}")
            ver = k8s_ver
        return ver <= k8s_ver

    # We don't have a version, default to the legacy behavior.
    return True


class InvokeResult:
    """
    InvokeResult is a helper type that wraps a prompt value in an Awaitable.
    """

    def __init__(
        self,
        value: Any,
        is_secret: bool = False,
        is_known: bool = True,
        dependencies: Optional[List["Resource"]] = None,
    ):
        self.value = value
        self.is_secret = is_secret
        self.is_known = is_known
        self.dependencies = dependencies or []

    def __await__(self):
        # We need __await__ to be an iterator, but we only want it to return one value. As such, we use
        # `if False: yield` to construct this.
        if False:
            yield self.value
        return self.value

    __iter__ = __await__


def invoke(
    tok: str,
    props: "Inputs",
    opts: Optional[InvokeOptions] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> InvokeResult:
    """
    invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    # Run the RPC callback asynchronously and then immediately await it.
    awaitableInvokeResult = _invoke(tok, props, opts, typ, package_ref=package_ref)
    return _sync_await(awaitableInvokeResult)


def invoke_single(
    tok: str,
    props: "Inputs",
    opts: Optional[InvokeOptions] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> Any:
    """
    Similar to `invoke`, but extracts a singular value from the result (eg { "foo": "bar" } => "bar").
    """
    result = invoke(tok, props, opts, typ, package_ref)
    return _extract_single_value(result)


def invoke_output(
    tok: str,
    props: "Inputs",
    opts: Optional[Union[InvokeOptions, InvokeOutputOptions]] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> "Output[Any]":
    """
    invoke_output dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is an Output[T] that
    resolves when the invoke finishes.
    """

    # Setup the futures for the output.
    resolve_value: "asyncio.Future" = asyncio.Future()
    resolve_is_known: "asyncio.Future[bool]" = asyncio.Future()
    resolve_is_secret: "asyncio.Future[bool]" = asyncio.Future()
    resolve_deps: "asyncio.Future[Set[Resource]]" = asyncio.Future()

    from .. import Output

    out = Output(resolve_deps, resolve_value, resolve_is_known, resolve_is_secret)

    async def do_invoke_output() -> None:
        try:
            invoke_result = await _invoke(
                tok, props, opts, typ, package_ref=package_ref, check_dependencies=True
            )

            resolve_value.set_result(
                invoke_result.value if invoke_result.is_known else None
            )
            resolve_is_known.set_result(invoke_result.is_known)
            resolve_is_secret.set_result(
                invoke_result.is_secret and invoke_result.is_known
            )
            resolve_deps.set_result(set(invoke_result.dependencies))
        except Exception as exn:  # noqa: BLE001 catch blind exception
            resolve_value.set_exception(exn)
            resolve_is_known.set_exception(exn)
            resolve_is_secret.set_exception(exn)
            resolve_deps.set_exception(exn)

    asyncio.ensure_future(_get_rpc_manager().do_rpc("invoke", do_invoke_output)())
    return out


def invoke_output_single(
    tok: str,
    props: "Inputs",
    opts: Optional[Union[InvokeOptions, InvokeOutputOptions]] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> "Output[Any]":
    """
    invoke_output_single performs the same action as invoke_output, but expects the function tok to return
    an object containing a single value, which it then extracts and returns.
    """
    return invoke_output(tok, props, opts, typ, package_ref).apply(
        _extract_single_value
    )


async def invoke_async(
    tok: str,
    props: "Inputs",
    opts: Optional[InvokeOptions] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> Any:
    """
    invoke_async dynamically asynchronously invokes the function, tok, which is offered by a provider plugin.
    the inputs can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    invokeResult = await _invoke(tok, props, opts, typ, package_ref=package_ref)
    return invokeResult.value


def _invoke(
    tok: str,
    props: "Inputs",
    opts: Optional[Union[InvokeOptions, InvokeOutputOptions]],
    typ: Optional[type],
    package_ref: Optional[Awaitable[Optional[str]]],
    check_dependencies: Optional[bool] = False,
) -> Awaitable[InvokeResult]:
    log.debug(f"Invoking function: tok={tok}")
    if opts is None:
        opts = InvokeOptions()

    if typ and not _types.is_output_type(typ):
        raise TypeError("Expected typ to be decorated with @output_type")

    async def do_invoke() -> Tuple[InvokeResult, Optional[Exception]]:
        # If a parent was provided, but no provider was provided, use the parent's provider if one was specified.
        if opts is not None and opts.parent is not None and opts.provider is None:
            opts.provider = opts.parent.get_provider(tok)

        # Construct a provider reference from the given provider, if one was provided to us.
        provider_ref = None
        if opts is not None and opts.provider is not None:
            provider_urn = await opts.provider.urn.future()
            provider_id = (await opts.provider.id.future()) or rpc.UNKNOWN
            provider_ref = f"{provider_urn}::{provider_id}"
            log.debug(f"Invoke using provider {provider_ref}")

        # If we have a package reference, we need to wait for it to resolve.
        package_ref_str = None
        if package_ref is not None:
            package_ref_str = await package_ref
            # If we have a package reference we can clear some of the invoke
            # options.
            if package_ref_str is not None and opts is not None:
                opts.plugin_download_url = None
                opts.version = None
                log.debug(f"Invoke using package reference {package_ref_str}")

        monitor = get_monitor()
        # keep track of the dependencies of the inputs
        property_dependencies: Dict[str, List["Resource"]] = {}
        inputs = await rpc.serialize_properties(props, property_dependencies)
        if rpc.struct_contains_unknowns(inputs):
            return (InvokeResult(None, is_secret=False, is_known=False), None)

        # keep track of secretness of inputs
        # if any of the inputs are secret OR the invoke response contains secrets
        # then we mark the invoke result as secret
        plain_inputs, inputs_contain_secrets = rpc._unwrap_rpc_secret_struct_properties(
            inputs
        )

        # The direct dependencies of the invoke.
        depends_on_dependencies: Set["Resource"] = set()
        # Only check the resource dependencies for output form invokes. For
        # plain invokes, we do not want to check the dependencies. Technically,
        # these should only receive plain arguments, but this is not strictly
        # enforced, and in practice people pass in outputs. This happens to work
        # because we serialize the arguments.
        if check_dependencies:
            depends_on_dependencies = await _resolve_depends_on(
                opts._depends_on_list() if isinstance(opts, InvokeOutputOptions) else []
            )
            # If we depend on any CustomResources, we need to ensure that their
            # ID is known before proceeding. If it is not known, we will return
            # an unknown result.
            resources_to_wait_for: Set["Resource"] = set(depends_on_dependencies)
            # Add the dependencies from the inputs to the set of resources to wait for.
            for deps in property_dependencies.values():
                resources_to_wait_for = resources_to_wait_for.union(deps)
            # The expanded set of dependencies, including children of components.
            expanded_deps = await rpc._expand_dependencies(resources_to_wait_for, None)
            # Ensure that all resource IDs are known before proceeding.
            for res in expanded_deps.values():
                # DependencyResources inherit from CustomResource, but they don't
                # set the id. Skip them.
                if isinstance(res, CustomResource) and res.__dict__.get("id", None):
                    if not await res.id.is_known():
                        return (
                            InvokeResult(None, is_secret=False, is_known=False),
                            None,
                        )

        version = opts.version or "" if opts is not None else ""
        plugin_download_url = opts.plugin_download_url or "" if opts is not None else ""
        accept_resources = os.getenv(
            "PULUMI_DISABLE_RESOURCE_REFERENCES", ""
        ).upper() not in {"TRUE", "1"}
        log.debug(f"Invoking function prepared: tok={tok}")
        req = resource_pb2.ResourceInvokeRequest(
            tok=tok,
            args=plain_inputs,
            provider=provider_ref or "",
            version=version,
            acceptResources=accept_resources,
            pluginDownloadURL=plugin_download_url,
            packageRef=package_ref_str or "",
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
            return (
                InvokeResult(None, is_secret=False),
                Exception(f"invoke of {tok} failed: {error}"),
            )

        if resp.failures:
            return (
                InvokeResult(None, is_secret=False),
                Exception(
                    f"invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})"
                ),
            )

        # Otherwise, return the output properties.
        ret_obj = getattr(resp, "return")

        # To avoid breaking older versions of the Kubernetes Python SDK, return None instead
        # of an empty dict for empty results if this invoke is for a version of the Pulumi
        # Kubernetes SDK that can't handle empty dicts.
        if _requires_legacy_none_return_for_empty_struct(ret_obj, tok, version):
            log.debug(f"Returning None for empty result for invoke of {tok}")
            return InvokeResult(value=None, is_secret=False), None

        deserialized, is_secret = rpc.deserialize_properties_unwrap_secrets(ret_obj)
        # If typ is not None, call translate_output_properties to instantiate any output types.
        result = deserialized
        if typ:
            result = rpc.translate_output_properties(
                deserialized, lambda prop: prop, typ
            )

        invoke_output_secret = is_secret or inputs_contain_secrets
        dependencies: Set["Resource"] = depends_on_dependencies
        for _, property_deps in property_dependencies.items():
            for dep in property_deps:
                dependencies.add(dep)
        return (
            InvokeResult(
                value=result,
                is_secret=invoke_output_secret,
                dependencies=list(dependencies),
            ),
            None,
        )

    async def do_rpc():
        # Await any dependencies before invoking our RPC.
        await _resolve_depends_on_urns(
            opts._depends_on_list() if isinstance(opts, InvokeOutputOptions) else []
        )

        resp, exn = await _get_rpc_manager().do_rpc("invoke", do_invoke)()
        # If there was an RPC level exception, we will raise it. Note that this will also crash the
        # process because it will have been considered "unhandled". For semantic level errors, such
        # as errors from the data source itself, we return that as part of the returned tuple instead.
        if exn is not None:
            raise exn
        invokeResult, error = resp
        if error is not None:
            raise error
        return invokeResult

    async def wait_for_fut():
        return await asyncio.ensure_future(do_rpc())

    return wait_for_fut()


def call(
    tok: str,
    props: "Inputs",
    res: Optional["Resource"] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> "Output[Any]":
    """
    call dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s).
    """
    log.debug(f"Calling function: tok={tok}")

    if typ and not _types.is_output_type(typ):
        raise TypeError("Expected typ to be decorated with @output_type")

    # Setup the futures for the output.
    resolve_value: "asyncio.Future" = asyncio.Future()
    resolve_is_known: "asyncio.Future[bool]" = asyncio.Future()
    resolve_is_secret: "asyncio.Future[bool]" = asyncio.Future()
    resolve_deps: "asyncio.Future[Set[Resource]]" = asyncio.Future()

    from .. import Output

    out = Output(resolve_deps, resolve_value, resolve_is_known, resolve_is_secret)

    async def do_call() -> None:
        try:
            # Construct a provider reference from the given provider, if one is available on the resource.
            provider_ref, version, plugin_download_url = None, "", ""
            if res is not None:
                if res._provider is not None:
                    provider_urn = await res._provider.urn.future()
                    provider_id = (await res._provider.id.future()) or rpc.UNKNOWN
                    provider_ref = f"{provider_urn}::{provider_id}"
                    log.debug(f"Call using provider {provider_ref}")
                version = res._version or ""
                plugin_download_url = res._plugin_download_url or ""

            monitor = get_monitor()

            # Serialize out all props to their final values. In doing so, we'll also collect all the Resources pointed to
            # by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
            property_dependencies_resources: Dict[str, List["Resource"]] = {}

            inputs = await rpc.serialize_properties(
                props,
                property_dependencies_resources,
                # We keep output values when serializing inputs for call.
                keep_output_values=True,
                # We exclude resource references from `argDependencies` when serializing inputs for call.
                # This way, providers creating output instances based on `argDependencies` won't create
                # outputs for properties that only contain resource references.
                exclude_resource_refs_from_deps=True,
            )

            property_dependencies = {}
            for key, property_deps in property_dependencies_resources.items():
                urns = set()
                for dep in property_deps:
                    urn = await dep.urn.future()
                    if urn is not None:
                        urns.add(urn)
                property_dependencies[key] = (
                    resource_pb2.ResourceCallRequest.ArgumentDependencies(
                        urns=list(urns)
                    )
                )

            # If we have a package reference, we need to wait for it to resolve.
            package_ref_str = None
            if package_ref is not None:
                package_ref_str = await package_ref
                # If we have a package reference we can clear some of the invoke
                # options.
                if package_ref_str is not None:
                    plugin_download_url = ""
                    version = ""
                    log.debug(f"Call using package reference {package_ref_str}")

            req = resource_pb2.ResourceCallRequest(
                tok=tok,
                args=inputs,
                argDependencies=property_dependencies,
                provider="" if provider_ref is None else provider_ref,
                version=version,
                pluginDownloadURL=plugin_download_url,
                packageRef=package_ref_str or "",
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
                raise Exception(
                    f"call of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})"
                )

            log.debug(f"Call successful: tok={tok}")

            value = None
            is_known = True
            is_secret = False
            deps: Set["Resource"] = set()
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
                deps_urns: Set[str] = (
                    {urn for v in rpc_deps.values() for urn in v.urns}
                    if rpc_deps
                    else set()
                )
                from ..resource import (
                    DependencyResource,
                )

                deps = set(map(DependencyResource, deps_urns))

                if is_known:
                    # If typ is not None, call translate_output_properties to instantiate any output types.
                    value = (
                        rpc.translate_output_properties(deserialized, lambda p: p, typ)
                        if typ
                        else deserialized
                    )

            resolve_value.set_result(value)
            resolve_is_known.set_result(is_known)
            resolve_is_secret.set_result(is_secret)
            resolve_deps.set_result(deps)

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}"
            )
            resolve_value.set_exception(exn)
            resolve_is_known.set_exception(exn)
            resolve_is_secret.set_exception(exn)
            resolve_deps.set_result(set())
            raise

    asyncio.ensure_future(_get_rpc_manager().do_rpc("call", do_call)())

    return out


def call_single(
    tok: str,
    props: "Inputs",
    res: Optional["Resource"] = None,
    typ: Optional[type] = None,
    package_ref: Optional[Awaitable[Optional[str]]] = None,
) -> "Output[Any]":
    """
    Similar to `call`, but extracts a singular value from the result (eg { "foo": "bar" } => "bar").
    """

    return call(tok, props, res, typ, package_ref).apply(_extract_single_value)


def _extract_single_value(out: Any) -> Any:
    """
    Extracts the first value from a result object.
    Does not check (throws) if the result is None or an empty.
    """

    if not isinstance(out, dict):
        return out

    return out[list(out.keys())[0]]
