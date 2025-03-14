# Copyright 2025, Pulumi Corporation.
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

from typing import Any


class _OutputToStringError(Exception):
    """_OutputToStringError is the class of errors raised when __str__ is called
    on a Pulumi Output."""

    def __init__(self) -> None:
        super().__init__(
            """Calling __str__ on an Output[T] is not supported.

To get the value of an Output[T] as an Output[str] consider:
1. o.apply(lambda v: f"prefix{v}suffix")

See https://www.pulumi.com/docs/concepts/inputs-outputs for more details."""
        )


def _safe_str(v: Any) -> str:
    """_safe_str returns the string representation of v if possible. If v is an
    Output, _safe_str catches the error and returns a fallback string. _safe_str
    is designed for use in e.g. logging and debugging contexts where it's useful
    to print all the information that can be reasonably obtained, without
    falling afoul of things like PULUMI_ERROR_OUTPUT_STRING."""

    try:
        return str(v)
    except _OutputToStringError:
        return "Output[T]"
