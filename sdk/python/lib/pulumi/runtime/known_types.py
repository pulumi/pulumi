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
The known_types module lazily loads classes defined in the parent module to
allow for type checking.

Python strictly disallows circular references between imported packages.
Because the Pulumi top-level module depends on the `pulumi.runtime` submodule,
it is not allowed for `pulumi.runtime` to reach back to the `pulumi` top-level
to reference types that are defined there.

In order to break this circular reference, and to be clear about what types
the runtime knows about and treats specially, we defer loading of the types from
within the functions themselves.

This implementation allows for overriding types used as stubs for testing
(see test/test_next_serialize.py)
"""
from typing import Any, Optional

_custom_resource_type: Optional[type] = None
"""The type of CustomResource."""

_custom_timeouts_type: Optional[type] = None
"""The type of CustomTimeouts."""

_asset_resource_type: Optional[type] = None
"""The type of Asset."""

_file_asset_resource_type: Optional[type] = None
"""The type of FileAsset."""

_string_asset_resource_type: Optional[type] = None
"""The type of StringAsset."""

_remote_asset_resource_type: Optional[type] = None
"""The type of RemoteAsset."""

_archive_resource_type: Optional[type] = None
"""The type of Archive."""

_asset_archive_resource_type: Optional[type] = None
"""The type of AssetArchive."""

_file_archive_resource_type: Optional[type] = None
"""The type of FileArchive."""

_remote_archive_resource_type: Optional[type] = None
"""The type of RemoteArchive."""

_stack_resource_type: Optional[type] = None
"""The type of Stack."""

_output_type: Optional[type] = None
"""The type of Output."""

_unknown_type: Optional[type] = None
"""The type of unknown."""


def is_asset(obj: Any) -> bool:
    """
    Returns true if the given type is an Asset, false otherwise.
    """
    from .. import Asset  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _asset_resource_type or Asset)


def is_archive(obj: Any) -> bool:
    """
    Returns true if the given type is an Archive, false otherwise.
    """
    from .. import Archive  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _archive_resource_type or Archive)


def is_custom_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomResource, false otherwise.
    """
    from .. import CustomResource  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _custom_resource_type or CustomResource)


def is_custom_timeouts(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomTimeouts, false otherwise.
    """
    from .. import CustomTimeouts  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _custom_timeouts_type or CustomTimeouts)


def is_stack(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .stack import Stack  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _stack_resource_type or Stack)


def is_output(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .. import Output  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _output_type or Output)


def is_unknown(obj: Any) -> bool:
    """
    Returns true if the given object is an Unknown, false otherwise.
    """
    from ..output import Unknown  # pylint: disable=import-outside-toplevel
    return isinstance(obj, _unknown_type or Unknown)
