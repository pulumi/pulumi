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

"""
Runtime settings and configuration.
"""
from __future__ import annotations

import asyncio
import os
import threading
from collections import deque
from contextvars import ContextVar
from typing import TYPE_CHECKING, Any, Optional, Union

import grpc

from .._utils import contextproperty
from ..errors import RunError
from ..runtime.proto import engine_pb2_grpc, resource_pb2, resource_pb2_grpc
from ._callbacks import _CallbackServicer
from .rpc_manager import RPCManager

if TYPE_CHECKING:
    from ..resource import Resource

# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE)]


# excessive_debug_output enables, well, pretty excessive debug output pertaining to resources and properties.
excessive_debug_output = False


class Settings:
    """
    A bag of properties for configuring the Pulumi Python language runtime.
    """

    def __init__(
        self,
        project: Optional[str],
        stack: Optional[str],
        monitor: Optional[Union[str, Any]] = None,
        engine: Optional[Union[str, Any]] = None,
        parallel: Optional[int] = None,
        dry_run: Optional[bool] = None,
        legacy_apply_enabled: Optional[bool] = None,
        organization: Optional[str] = None,
    ):
        self.rpc_manager = RPCManager()
        self.outputs = deque()
        self.lock = threading.Lock()

        # Save the metadata information.
        self.project = project
        self.stack = stack
        self.parallel = parallel
        self.dry_run = dry_run
        self.legacy_apply_enabled = legacy_apply_enabled
        self.feature_support = {}
        self.organization = organization

        if self.legacy_apply_enabled is None:
            self.legacy_apply_enabled = (
                os.getenv("PULUMI_ENABLE_LEGACY_APPLY", "false") == "true"
            )

        # Actually connect to the monitor/engine over gRPC.
        if monitor is not None:
            if isinstance(monitor, str):
                self.monitor = resource_pb2_grpc.ResourceMonitorStub(
                    grpc.insecure_channel(monitor, options=_GRPC_CHANNEL_OPTIONS),
                )
            else:
                self.monitor = monitor
        else:
            self.monitor = None
        if engine:
            if isinstance(engine, str):
                self.engine = engine_pb2_grpc.EngineStub(
                    grpc.insecure_channel(engine, options=_GRPC_CHANNEL_OPTIONS),
                )
            else:
                self.engine = engine
        else:
            self.engine = None

        self.callbacks = None

    @contextproperty
    def rpc_manager(self) -> RPCManager:  # type: ignore
        # The contextproperty decorator will fill the body of this method in, but mypy doesn't know that.
        ...

    @contextproperty
    def lock(self) -> threading.Lock: ...  # type: ignore

    @contextproperty
    def outputs(self) -> deque[asyncio.Task]: ...  # type: ignore

    @contextproperty
    def monitor(self) -> Optional[resource_pb2_grpc.ResourceMonitorStub]: ...

    @contextproperty
    def engine(self) -> Optional[engine_pb2_grpc.EngineStub]: ...

    @contextproperty
    def organization(self) -> Optional[str]: ...

    @contextproperty
    def project(self) -> Optional[str]: ...

    @contextproperty
    def stack(self) -> Optional[str]: ...

    @contextproperty
    def parallel(self) -> Optional[bool]: ...

    @contextproperty
    def dry_run(self) -> Optional[bool]: ...

    @contextproperty
    def legacy_apply_enabled(self) -> Optional[bool]: ...

    @contextproperty
    def feature_support(self) -> Optional[dict]: ...

    @contextproperty
    def callbacks(self) -> Optional[_CallbackServicer]: ...

    def __repr__(self):
        return f"<class Settings[engine={self.engine.__repr__()} monitor={self.monitor.__repr__()} project={self.project.__repr__()} stack={self.stack.__repr__()}>"


# default to "empty" settings.
SETTINGS = Settings(stack="stack", project="project", organization="organization")


def configure(settings: Settings):
    """
    Configure sets the current ambient settings bag to the one given.
    """
    if not settings or not isinstance(settings, Settings):
        raise TypeError("Settings is expected to be non-None and of type Settings")
    # The properties of SETTINGS are contextvars but SETTINGS itself isn't.
    for key, value in settings.__dict__.items():
        setattr(SETTINGS, key, value)


def is_dry_run() -> bool:
    """
    Returns whether or not we are currently doing a preview.

    When writing unit tests, you can set this flag via `pulumi.runtime.set_mocks` by supplying a value
    for the argument `preview`.
    """
    return bool(SETTINGS.dry_run)


def is_legacy_apply_enabled():
    return bool(SETTINGS.legacy_apply_enabled)


def get_organization() -> str:
    """
    Returns the current organization name.

    When writing unit tests, you can set this flag via `pulumi.runtime.set_mocks` by supplying a value
    for the argument `organization`.
    """
    return SETTINGS.organization


def get_project() -> str:
    """
    Returns the current project name.
    """
    return SETTINGS.project


def _set_project(v: Optional[str]):
    """
    Set the project name programmatically -- meant for testing only.
    """
    SETTINGS.project = v


def get_stack() -> str:
    """
    Returns the current stack name.
    """
    return SETTINGS.stack


def _set_stack(v: Optional[str]):
    """
    Set the stack name programmatically -- meant for testing only.
    """
    SETTINGS.stack = v


def _get_rpc_manager() -> RPCManager:
    """
    Returns the current rpc manager.
    """
    return SETTINGS.rpc_manager


def get_monitor() -> Optional[Union[resource_pb2_grpc.ResourceMonitorStub, Any]]:
    """
    Returns the current resource monitoring service client for RPC communications.
    """
    return SETTINGS.monitor


def get_engine() -> Optional[Union[engine_pb2_grpc.EngineStub, Any]]:
    """
    Returns the current engine service client for RPC communications.
    """
    return SETTINGS.engine


async def _get_callbacks() -> Optional[_CallbackServicer]:
    """
    Returns the current callbacks for RPC communications.
    """
    callbacks = SETTINGS.callbacks
    if callbacks is not None:
        return callbacks

    monitor = SETTINGS.monitor
    if monitor is None or not isinstance(
        monitor, resource_pb2_grpc.ResourceMonitorStub
    ):
        return None

    callbacks = _CallbackServicer(monitor)
    await callbacks.serve()
    SETTINGS.callbacks = callbacks
    return callbacks


async def _shutdown_callbacks():
    await _CallbackServicer.shutdown()


def get_root_resource() -> Optional["Resource"]:
    """
    Returns the implicit root stack resource for all resources created in this program.
    """
    return ROOT.get()


def set_root_resource(root: "Resource"):
    """
    Sets the current root stack resource for all resources subsequently to be created in this program.
    """
    ROOT.set(root)


ROOT: ContextVar[Optional[Resource]] = ContextVar("root_resource", default=None)


async def monitor_supports_feature(feature: str) -> bool:
    if feature not in SETTINGS.feature_support:
        monitor = SETTINGS.monitor
        if not monitor:
            return False

        result = await _monitor_supports_feature(monitor, feature)
        SETTINGS.feature_support[feature] = result

    return SETTINGS.feature_support[feature]


def grpc_error_to_exception(exn: grpc.RpcError) -> Exception:
    # gRPC-python gets creative with their exceptions. grpc.RpcError as a type is useless;
    # the usefulness come from the fact that it is polymorphically also a grpc.Call and thus has
    # the .code() member. Pylint doesn't know this because it's not known statically.
    #
    # Neither pylint nor I are the only ones who find this confusing:
    # https://github.com/grpc/grpc/issues/10885#issuecomment-302581315
    # pylint: disable=no-member
    if exn.code() == grpc.StatusCode.UNAVAILABLE:
        # If the monitor is unavailable, it is in the process of
        # shutting down or has already shut down.
        return RunError("Resource monitor has terminated, shutting down")

    details = exn.details()
    return Exception(details)


def handle_grpc_error(exn: grpc.RpcError) -> None:
    raise grpc_error_to_exception(exn)


async def monitor_supports_secrets() -> bool:
    return await monitor_supports_feature("secrets")


async def monitor_supports_resource_references() -> bool:
    return await monitor_supports_feature("resourceReferences")


async def monitor_supports_output_values() -> bool:
    return await monitor_supports_feature("outputValues")


async def monitor_supports_deleted_with() -> bool:
    return await monitor_supports_feature("deletedWith")


async def monitor_supports_alias_specs() -> bool:
    return await monitor_supports_feature("aliasSpecs")


def _sync_monitor_supports_transforms() -> bool:
    if "transforms" not in SETTINGS.feature_support:
        return False
    return SETTINGS.feature_support["transforms"]


def _sync_monitor_supports_invoke_transforms() -> bool:
    if "invokeTransforms" not in SETTINGS.feature_support:
        return False
    return SETTINGS.feature_support["invokeTransforms"]


def reset_options(
    project: Optional[str] = None,
    stack: Optional[str] = None,
    parallel: Optional[int] = None,
    engine_address: Optional[str] = None,
    monitor_address: Optional[str] = None,
    preview: Optional[bool] = None,
    organization: Optional[str] = None,
):
    """Resets globals to the values provided."""

    ROOT.set(None)

    configure(
        Settings(
            project=project,
            monitor=monitor_address,
            engine=engine_address,
            stack=stack,
            parallel=parallel,
            dry_run=preview,
            organization=organization,
        )
    )


async def _monitor_supports_feature(
    monitor: resource_pb2_grpc.ResourceMonitorStub, feature: str
) -> bool:
    req = resource_pb2.SupportsFeatureRequest(id=feature)

    def do_rpc_call():
        try:
            resp = monitor.SupportsFeature(req)
            return resp.hasSupport
        except grpc.RpcError as exn:
            if exn.code() != grpc.StatusCode.UNIMPLEMENTED:  # pylint: disable=no-member
                handle_grpc_error(exn)
            return False

    return await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)


async def _load_monitor_feature_support():
    # Prime the feature support cache.
    await asyncio.gather(
        monitor_supports_feature("secrets"),
        monitor_supports_feature("resourceReferences"),
        monitor_supports_feature("outputValues"),
        monitor_supports_feature("deletedWith"),
        monitor_supports_feature("aliasSpecs"),
        monitor_supports_feature("transforms"),
        monitor_supports_feature("invokeTransforms"),
    )
