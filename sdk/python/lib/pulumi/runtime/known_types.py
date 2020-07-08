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
The known_types module contains state for keeping track of types that
are known to be special in the Pulumi type system.

Python strictly disallows circular references between imported packages.
Because the Pulumi top-level module depends on the `pulumi.runtime` submodule,
it is not allowed for `pulumi.runtime` to reach back to the `pulumi` top-level
to reference types that are defined there.

In order to break this circular reference, and to be clear about what types
the runtime knows about and treats specially, this module exports a number of
"known type" decorators that can be applied to types in `pulumi` to indicate
that they are specially treated.

The implementation of this mechanism is that, for every known type, that type
is stashed away in a global variable. Whenever the runtime wants to do a type
test using that type (or instantiate an instance of this type), it uses the
functions defined in this module to do so.
"""
from typing import Any, Optional

_custom_resource_type: Optional[type] = None
"""The type of CustomResource. Filled-in as the Pulumi package is initializing."""

_custom_timeouts_type: Optional[type] = None
"""The type of CustomTimeouts. Filled-in as the Pulumi package is initializing."""

_asset_resource_type: Optional[type] = None
"""The type of Asset. Filled-in as the Pulumi package is initializing."""

_file_asset_resource_type: Optional[type] = None
"""The type of FileAsset. Filled-in as the Pulumi package is initializing."""

_string_asset_resource_type: Optional[type] = None
"""The type of StringAsset. Filled-in as the Pulumi package is initializing."""

_remote_asset_resource_type: Optional[type] = None
"""The type of RemoteAsset. Filled-in as the Pulumi package is initializing."""

_archive_resource_type: Optional[type] = None
"""The type of Archive. Filled-in as the Pulumi package is initializing."""

_asset_archive_resource_type: Optional[type] = None
"""The type of AssetArchive. Filled-in as the Pulumi package is initializing."""

_file_archive_resource_type: Optional[type] = None
"""The type of FileArchive. Filled-in as the Pulumi package is initializing."""

_remote_archive_resource_type: Optional[type] = None
"""The type of RemoteArchive. Filled-in as the Pulumi package is initializing."""

_stack_resource_type: Optional[type] = None
"""The type of Stack. Filled-in as the Pulumi package is initializing."""

_output_type: Optional[type] = None
"""The type of Output. Filled-in as the Pulumi package is initializing."""

_unknown_type: Optional[type] = None
"""The type of unknown. Filled-in as the Pulumi package is initializing."""


def is_asset(obj: Any) -> bool:
    """
    Returns true if the given type is an Asset, false otherwise.
    """
    from .. import Asset
    return isinstance(obj, _asset_resource_type or Asset)


def is_archive(obj: Any) -> bool:
    """
    Returns true if the given type is an Archive, false otherwise.
    """
    from .. import Archive
    return isinstance(obj, _archive_resource_type or Archive)


def is_custom_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomResource, false otherwise.
    """
    from .. import CustomResource
    return isinstance(obj, _custom_resource_type or CustomResource)


def is_custom_timeouts(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomTimeouts, false otherwise.
    """
    from .. import CustomTimeouts
    return isinstance(obj, _custom_timeouts_type or CustomTimeouts)


def is_stack(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .stack import Stack
    return isinstance(obj, _stack_resource_type or Stack)


def is_output(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .. import Output
    return isinstance(obj, _output_type or Output)


def is_unknown(obj: Any) -> bool:
    """
    Returns true if the given object is an Unknown, false otherwise.
    """
    from ..output import Unknown
    return isinstance(obj, _unknown_type or Unknown)
