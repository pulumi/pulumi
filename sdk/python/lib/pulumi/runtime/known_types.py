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
"""
from typing import Any, Optional


def is_asset(obj: Any) -> bool:
    """
    Returns true if the given type is an Asset, false otherwise.
    """
    from .. import Asset  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Asset)


def is_archive(obj: Any) -> bool:
    """
    Returns true if the given type is an Archive, false otherwise.
    """
    from .. import Archive  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Archive)


def is_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a Resource, false otherwise.
    """
    from .. import Resource  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Resource)


def is_custom_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomResource, false otherwise.
    """
    from .. import CustomResource  # pylint: disable=import-outside-toplevel
    return isinstance(obj, CustomResource)


def is_custom_timeouts(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomTimeouts, false otherwise.
    """
    from .. import CustomTimeouts  # pylint: disable=import-outside-toplevel
    return isinstance(obj, CustomTimeouts)


def is_stack(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .stack import Stack  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Stack)


def is_output(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    from .. import Output  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Output)


def is_unknown(obj: Any) -> bool:
    """
    Returns true if the given object is an Unknown, false otherwise.
    """
    from ..output import Unknown  # pylint: disable=import-outside-toplevel
    return isinstance(obj, Unknown)
