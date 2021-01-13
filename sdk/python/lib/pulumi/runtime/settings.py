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
import asyncio
import os
import sys
from typing import Optional, Awaitable, Union, Any, TYPE_CHECKING

import grpc
from ..runtime.proto import engine_pb2_grpc, resource_pb2, resource_pb2_grpc
from ..errors import RunError

if TYPE_CHECKING:
    from ..resource import Resource

# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [('grpc.max_receive_message_length', _MAX_RPC_MESSAGE_SIZE)]

class Settings:
    monitor: Optional[Union[resource_pb2_grpc.ResourceMonitorStub, Any]]
    engine: Optional[Union[engine_pb2_grpc.EngineStub, Any]]
    project: Optional[str]
    stack: Optional[str]
    parallel: Optional[str]
    dry_run: Optional[bool]
    test_mode_enabled: Optional[bool]
    legacy_apply_enabled: Optional[bool]
    feature_support: dict

    """
    A bag of properties for configuring the Pulumi Python language runtime.
    """
    def __init__(self,
                 monitor: Optional[Union[str, Any]] = None,
                 engine: Optional[Union[str, Any]] = None,
                 project: Optional[str] = None,
                 stack: Optional[str] = None,
                 parallel: Optional[str] = None,
                 dry_run: Optional[bool] = None,
                 test_mode_enabled: Optional[bool] = None,
                 legacy_apply_enabled: Optional[bool] = None):
        # Save the metadata information.
        self.project = project
        self.stack = stack
        self.parallel = parallel
        self.dry_run = dry_run
        self.test_mode_enabled = test_mode_enabled
        self.legacy_apply_enabled = legacy_apply_enabled
        self.feature_support = {}

        if self.test_mode_enabled is None:
            self.test_mode_enabled = os.getenv("PULUMI_TEST_MODE", "false") == "true"

        if self.legacy_apply_enabled is None:
            self.legacy_apply_enabled = os.getenv("PULUMI_ENABLE_LEGACY_APPLY", "false") == "true"


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

# default to "empty" settings.
SETTINGS = Settings()


def configure(settings: Settings):
    """
    Configure sets the current ambient settings bag to the one given.
    """
    if not settings or not isinstance(settings, Settings):
        raise TypeError('Settings is expected to be non-None and of type Settings')
    global SETTINGS  # pylint: disable=global-statement
    SETTINGS = settings


def is_dry_run() -> bool:
    """
    Returns whether or not we are currently doing a preview.
    """
    return bool(SETTINGS.dry_run)


def is_test_mode_enabled() -> bool:
    """
    Returns true if test mode is enabled (PULUMI_TEST_MODE).
    """
    return bool(SETTINGS.test_mode_enabled)


def _set_test_mode_enabled(v: Optional[bool]):
    """
    Enable or disable testing mode programmatically -- meant for testing only.
    """
    SETTINGS.test_mode_enabled = v


def require_test_mode_enabled():
    if not is_test_mode_enabled():
        raise RunError('Program run without the Pulumi engine available; re-run using the `pulumi` CLI')

def is_legacy_apply_enabled():
    return bool(SETTINGS.legacy_apply_enabled)


def get_project() -> str:
    """
    Returns the current project name.
    """
    project = SETTINGS.project
    if not project:
        require_test_mode_enabled()
        raise RunError('Missing project name; for test mode, please call `pulumi.runtime.set_mocks`')
    return project


def _set_project(v: Optional[str]):
    """
    Set the project name programmatically -- meant for testing only.
    """
    SETTINGS.project = v


def get_stack() -> str:
    """
    Returns the current stack name.
    """
    stack = SETTINGS.stack
    if not stack:
        require_test_mode_enabled()
        raise RunError('Missing stack name; for test mode, please set PULUMI_NODEJS_STACK')
    return stack


def _set_stack(v: Optional[str]):
    """
    Set the stack name programmatically -- meant for testing only.
    """
    SETTINGS.stack = v


def get_monitor() -> Optional[Union[resource_pb2_grpc.ResourceMonitorStub, Any]]:
    """
    Returns the current resource monitoring service client for RPC communications.
    """
    monitor = SETTINGS.monitor
    if not monitor:
        require_test_mode_enabled()
    return monitor


def get_engine() -> Optional[Union[engine_pb2_grpc.EngineStub, Any]]:
    """
    Returns the current engine service client for RPC communications.
    """
    return SETTINGS.engine


ROOT: Optional['Resource'] = None


def get_root_resource() -> Optional['Resource']:
    """
    Returns the implicit root stack resource for all resources created in this program.
    """
    global ROOT
    return ROOT


def set_root_resource(root: 'Resource'):
    """
    Sets the current root stack resource for all resources subsequently to be created in this program.
    """
    global ROOT
    ROOT = root


async def monitor_supports_feature(feature: str) -> bool:
    if feature not in SETTINGS.feature_support:
        monitor = SETTINGS.monitor
        if not monitor:
            return False

        req = resource_pb2.SupportsFeatureRequest(id=feature)
        def do_rpc_call():
            try:
                resp = monitor.SupportsFeature(req)
                return resp.hasSupport
            except grpc.RpcError as exn:
                # See the comment on invoke for the justification for disabling
                # this warning
                # pylint: disable=no-member
                if exn.code() == grpc.StatusCode.UNAVAILABLE:
                    sys.exit(0)
                if exn.code() == grpc.StatusCode.UNIMPLEMENTED:
                    return False
                details = exn.details()
            raise Exception(details)

        result = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        SETTINGS.feature_support[feature] = result

    return SETTINGS.feature_support[feature]

async def monitor_supports_secrets() -> bool:
    return await monitor_supports_feature("secrets")

async def monitor_supports_resource_references() -> bool:
    return await monitor_supports_feature("resourceReferences")
