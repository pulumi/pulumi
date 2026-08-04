# Copyright 2026, Pulumi Corporation.
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

"""Callback-local restrictions on re-entering the Pulumi runtime."""

from collections.abc import Iterator
from contextlib import contextmanager
from contextvars import ContextVar
from dataclasses import dataclass


@dataclass(frozen=True)
class _CallbackRestrictions:
    """
    Describes the Pulumi runtime capabilities available to a callback.

    Provider functions include Invoke and Call.
    """

    description: str
    allow_provider_functions: bool
    guidance: str


_CALLBACK_RESTRICTIONS: ContextVar[_CallbackRestrictions | None] = ContextVar(
    "callback_restrictions", default=None
)


@contextmanager
def _callback_restrictions(
    description: str,
    *,
    allow_provider_functions: bool,
    guidance: str,
) -> Iterator[None]:
    """
    Applies restrictions for the current callback's asynchronous context.

    The restrictions remain active across normal asynchronous suspension and in
    tasks created by the callback. The description and guidance appear in
    user-visible errors.
    """

    token = _CALLBACK_RESTRICTIONS.set(
        _CallbackRestrictions(
            description=description,
            allow_provider_functions=allow_provider_functions,
            guidance=guidance,
        )
    )
    try:
        yield
    finally:
        _CALLBACK_RESTRICTIONS.reset(token)


def _ensure_runtime_operation_allowed(
    operation: str, *, provider_function: bool = False
) -> None:
    """
    Raises when the active callback restrictions do not permit an operation.

    ``provider_function`` means the operation is part of an Invoke or Call to a
    provider.
    """

    restrictions = _CALLBACK_RESTRICTIONS.get()
    if restrictions is None or (
        provider_function and restrictions.allow_provider_functions
    ):
        return

    raise RuntimeError(
        f"Pulumi runtime operation {operation!r} is not allowed inside "
        f"{restrictions.description}. {restrictions.guidance}"
    )
