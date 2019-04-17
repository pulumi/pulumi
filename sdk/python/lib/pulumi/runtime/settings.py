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
from typing import Optional, TYPE_CHECKING

import grpc
from ..runtime.proto import engine_pb2_grpc, resource_pb2_grpc
from ..errors import RunError

if TYPE_CHECKING:
    from ..resource import Resource


class Settings:
    monitor: Optional[resource_pb2_grpc.ResourceMonitorStub]
    engine: Optional[engine_pb2_grpc.EngineStub]
    project: Optional[str]
    stack: Optional[str]
    parallel: Optional[str]
    dry_run: Optional[bool]
    test_mode_enabled: Optional[bool]

    """
    A bag of properties for configuring the Pulumi Python language runtime.
    """
    def __init__(self,
                 monitor: Optional[str] = None,
                 engine: Optional[str] = None,
                 project: Optional[str] = None,
                 stack: Optional[str] = None,
                 parallel: Optional[str] = None,
                 dry_run: Optional[bool] = None,
                 test_mode_enabled: Optional[bool] = None):
        # Save the metadata information.
        self.project = project
        self.stack = stack
        self.parallel = parallel
        self.dry_run = dry_run
        self.test_mode_enabled = test_mode_enabled

        # Actually connect to the monitor/engine over gRPC.
        if monitor:
            self.monitor = resource_pb2_grpc.ResourceMonitorStub(grpc.insecure_channel(monitor))
        else:
            self.monitor = None
        if engine:
            self.engine = engine_pb2_grpc.EngineStub(grpc.insecure_channel(engine))
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
    return True if SETTINGS.dry_run else False


def is_test_mode_enabled() -> bool:
    """
    Returns true if test mode is enabled (PULUMI_TEST_MODE).
    """
    return True if SETTINGS.test_mode_enabled else False


def _set_test_mode_enabled(v: Optional[bool]):
    """
    Enable or disable testing mode programmatically -- meant for testing only.
    """
    SETTINGS.test_mode_enabled = v


def require_test_mode_enabled():
    if not is_test_mode_enabled():
        raise RunError('Program run without the `pulumi` CLI; this may not be what you want '+
                       '(enable PULUMI_TEST_MODE to disable this error)')


def get_project() -> Optional[str]:
    """
    Returns the current project name.
    """
    project = SETTINGS.project
    if not project:
        require_test_mode_enabled()
        raise RunError('Missing project name; for test mode, please set PULUMI_NODEJS_PROJECT')
    return project


def _set_project(v: Optional[str]):
    """
    Set the project name programmatically -- meant for testing only.
    """
    SETTINGS.project = v


def get_stack() -> Optional[str]:
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


def get_monitor() -> Optional[resource_pb2_grpc.ResourceMonitorStub]:
    """
    Returns the current resource monitoring service client for RPC communications.
    """
    monitor = SETTINGS.monitor
    if not monitor:
        require_test_mode_enabled()
    return monitor


def get_engine() -> Optional[engine_pb2_grpc.EngineStub]:
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
