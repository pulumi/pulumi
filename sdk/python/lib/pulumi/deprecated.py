# Copyright 2024, Pulumi Corporation.
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

import functools
import warnings
from typing import (
    Callable,
    TypeVar,
    cast,
)
from .log import warn
from ._types import _PULUMI_DEPRECATED_CALLABLE

C = TypeVar("C", bound=Callable)


def deprecated(message: str) -> Callable[[C], C]:
    """
    Decorator to indicate a function is deprecated.

    As well as inserting appropriate statements to indicate that the function is
    deprecated, this decorator also tags the function with a special attribute
    so that Pulumi code can detect that it is deprecated and react appropriately
    in certain situations.

    message is the deprecation message that should be printed if the function is called.
    """

    def decorator(fn: C) -> C:
        if not callable(fn):
            raise TypeError("Expected fn to be callable")

        @functools.wraps(fn)
        def deprecated_fn(*args, **kwargs):
            warnings.warn(message)
            warn(f"{fn.__name__} is deprecated: {message}")

            return fn(*args, **kwargs)

        deprecated_fn.__dict__[_PULUMI_DEPRECATED_CALLABLE] = fn
        return cast(C, deprecated_fn)

    return decorator
