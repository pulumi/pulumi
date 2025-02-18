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
The runtime implementation of the Pulumi Python SDK.
"""

from .config import (
    set_config,
    set_all_config,
    get_config,
    get_config_env,
    get_config_env_key,
    get_config_secret_keys_env,
    is_config_secret,
)

from .mocks import (
    Mocks,
    set_mocks,
    test,
    MockResourceArgs,
    MockCallArgs,
)

from .settings import (
    Settings,
    configure,
    is_dry_run,
    reset_options,
    get_root_resource,
    get_root_directory,
)

from .stack import (
    run_in_stack,
    register_stack_transformation,
    register_stack_transform,
    register_resource_transform,
    register_invoke_transform,
)

from .invoke import (
    invoke,
    invoke_async,
    invoke_output,
    call,
)

from ._json import (
    to_json,
)

from .rpc import (
    ResourceModule,
    ResourcePackage,
    register_resource_module,
    register_resource_package,
)

__all__ = [
    # config
    "set_config",
    "set_all_config",
    "get_config",
    "get_config_env",
    "get_config_env_key",
    "get_config_secret_keys_env",
    "is_config_secret",
    # mocks
    "Mocks",
    "set_mocks",
    "test",
    "MockCallArgs",
    "MockResourceArgs",
    # settings
    "Settings",
    "configure",
    "is_dry_run",
    "reset_options",
    "get_root_resource",
    "get_root_directory",
    # stack
    "run_in_stack",
    "register_stack_transformation",
    "register_stack_transform",
    "register_resource_transform",
    "register_invoke_transform",
    # invoke
    "invoke",
    "invoke_async",
    "invoke_output",
    "call",
    # _json
    "to_json",
    # rpc
    "ResourceModule",
    "ResourcePackage",
    "register_resource_module",
    "register_resource_package",
    # submodules
    "rpc",
]
