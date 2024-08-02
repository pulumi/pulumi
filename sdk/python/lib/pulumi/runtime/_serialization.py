# Copyright 2016-2024, Pulumi Corporation.
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

from contextvars import ContextVar, copy_context
from typing import Callable, TypeVar, Tuple


__all__ = [
    "_serialize",
    "_deserialize",
    "_serialization_enabled",
    "_secrets_allowed",
    "_set_contained_secrets",
]


_T = TypeVar("_T")

# ContextVars that control whether serialization is enabled, whether secrets are allowed to be serialized,
# and whether the serialized data contains secrets.
_var_serialization_enabled = ContextVar("serialization_enabled", default=False)
_var_serialization_allow_secrets = ContextVar(
    "serialization_allow_secrets", default=False
)
_var_serialization_contained_secrets = ContextVar(
    "serialization_contained_secrets", default=False
)


def _serialize(
    allow_secrets: bool, f: Callable[..., _T], *args, **kwargs
) -> Tuple[_T, bool]:
    """
    Run the given function with serialization enabled.
    """

    def serialize():
        _var_serialization_enabled.set(True)
        _var_serialization_allow_secrets.set(allow_secrets)
        result = f(*args, **kwargs)
        return result, _var_serialization_contained_secrets.get()

    ctx = copy_context()
    return ctx.run(serialize)


def _deserialize(f: Callable[..., _T], *args, **kwargs) -> _T:
    """
    Run the given function with serialization enabled.
    """

    def deserialize():
        _var_serialization_enabled.set(True)
        return f(*args, **kwargs)

    ctx = copy_context()
    return ctx.run(deserialize)


def _serialization_enabled() -> bool:
    """
    Returns whether serialization is enabled.
    """
    return _var_serialization_enabled.get()


def _secrets_allowed() -> bool:
    """
    Returns whether secrets are allowed to be serialized.
    """
    return _var_serialization_allow_secrets.get()


def _set_contained_secrets(value: bool) -> None:
    """
    Set that the serialized data contains secrets.
    """
    _var_serialization_contained_secrets.set(value)
