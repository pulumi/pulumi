# Copyright 2016, Pulumi Corporation.
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

from typing import Any

# Cache class references to avoid repeated import machinery overhead. These functions are called *a lot* during
# serialization, so this optimization does add up.
_Asset: type | None = None
_Archive: type | None = None
_Resource: type | None = None
_CustomResource: type | None = None
_CustomTimeouts: type | None = None
_Stack: type | None = None
_Output: type | None = None
_Unknown: type | None = None


def is_asset(obj: Any) -> bool:
    """
    Returns true if the given type is an Asset, false otherwise.
    """
    global _Asset  # noqa: PLW0603
    if _Asset is None:
        from .. import Asset

        _Asset = Asset
    return isinstance(obj, _Asset)


def is_archive(obj: Any) -> bool:
    """
    Returns true if the given type is an Archive, false otherwise.
    """
    global _Archive  # noqa: PLW0603
    if _Archive is None:
        from .. import Archive

        _Archive = Archive
    return isinstance(obj, _Archive)


def is_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a Resource, false otherwise.
    """
    global _Resource  # noqa: PLW0603
    if _Resource is None:
        from .. import Resource

        _Resource = Resource
    return isinstance(obj, _Resource)


def is_custom_resource(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomResource, false otherwise.
    """
    global _CustomResource  # noqa: PLW0603
    if _CustomResource is None:
        from .. import CustomResource

        _CustomResource = CustomResource
    return isinstance(obj, _CustomResource)


def is_custom_timeouts(obj: Any) -> bool:
    """
    Returns true if the given type is a CustomTimeouts, false otherwise.
    """
    global _CustomTimeouts  # noqa: PLW0603
    if _CustomTimeouts is None:
        from .. import CustomTimeouts

        _CustomTimeouts = CustomTimeouts
    return isinstance(obj, _CustomTimeouts)


def is_stack(obj: Any) -> bool:
    """
    Returns true if the given type is a Stack, false otherwise.
    """
    global _Stack  # noqa: PLW0603
    if _Stack is None:
        from .stack import Stack

        _Stack = Stack
    return isinstance(obj, _Stack)


def is_output(obj: Any) -> bool:
    """
    Returns true if the given type is an Output, false otherwise.
    """
    global _Output  # noqa: PLW0603
    if _Output is None:
        from .. import Output

        _Output = Output
    return isinstance(obj, _Output)


def is_unknown(obj: Any) -> bool:
    """
    Returns true if the given object is an Unknown, false otherwise.
    """
    global _Unknown  # noqa: PLW0603
    if _Unknown is None:
        from ..output import Unknown

        _Unknown = Unknown
    return isinstance(obj, _Unknown)
