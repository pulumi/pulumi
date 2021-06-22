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
The Pulumi Core SDK for Python. This package defines the core primitives that
providers and libraries in the Pulumi ecosystem use to create and manage
resources.
"""

# Make all module members inside of this package available as package members.
from .asset import (
    Asset,
    Archive,
    AssetArchive,
    FileArchive,
    FileAsset,
    RemoteArchive,
    RemoteAsset,
    StringAsset,
)

from .config import (
    Config,
    ConfigMissingError,
    ConfigTypeError,
)

from .errors import (
    RunError,
)

from .invoke import (
    InvokeOptions,
)

from .metadata import (
    get_project,
    get_stack,
)

from .resource import (
    Alias,
    Resource,
    CustomResource,
    CustomTimeouts,
    ComponentResource,
    ProviderResource,
    ResourceOptions,
    create_urn,
    export,
    ROOT_STACK_RESOURCE,
    ResourceTransformation,
    ResourceTransformationArgs,
    ResourceTransformationResult,
)

from .output import (
    Output,
    Input,
    Inputs,
    InputType,
    UNKNOWN,
    contains_unknowns,
)

from .log import (
    debug,
    info,
    warn,
    error,
)

from .stack_reference import (
    StackReference,
)

# pylint: disable=redefined-builtin
from ._types import (
    MISSING,
    input_type,
    output_type,
    property,
    getter,
    get,
    set,
)

from . import runtime, dynamic, policy, automation

__all__ = [
    # asset
    "Asset",
    "Archive",
    "AssetArchive",
    "FileArchive",
    "FileAsset",
    "RemoteArchive",
    "RemoteAsset",
    "StringAsset",

    # config
    "Config",
    "ConfigMissingError",
    "ConfigTypeError",

    # errors
    "RunError",

    # invoke
    "InvokeOptions",

    # metadata
    "get_project",
    "get_stack",

    # resource
    "Alias",
    "Resource",
    "CustomResource",
    "CustomTimeouts",
    "ComponentResource",
    "ProviderResource",
    "ResourceOptions",
    "create_urn",
    "export",
    "ROOT_STACK_RESOURCE",
    "ResourceTransformation",
    "ResourceTransformationArgs",
    "ResourceTransformationResult",

    # output
    "Output",
    "Input",
    "Inputs",
    "InputType",
    "UNKNOWN",
    "contains_unknowns",

    # log
    "debug",
    "info",
    "warn",
    "error",

    # stack_reference
    "StackReference",

    # _types
    "MISSING",
    "input_type",
    "output_type",
    "property",
    "getter",
    "get",
    "set",

    # sub-modules
    "runtime",
    "dynamic",
    "policy",
    "automation"
]
