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

import inspect
from typing import Optional, Union
import typing

import pytest

from pulumi import Output, Input
from pulumi import _types
import pulumi


def test_is_optional_type():
    assert _types._is_optional_type(Optional[str]) == True
    assert _types._is_optional_type(str | None) == True
    assert _types._is_optional_type(None | str) == True


def test_is_union_type():
    assert _types._is_union_type(Union[str, int]) == True
    assert _types._is_union_type(Optional[str]) == True
    assert _types._is_union_type(str | int) == True


def test_unwrap_optional_type():
    class A:
        a: Optional[str]
        b: int | None

    anno = inspect.get_annotations(A)
    assert _types.unwrap_optional_type(anno["a"]) == str
    assert _types.unwrap_optional_type(anno["b"]) == int


@pytest.mark.asyncio
async def test_unwrap_type():
    class A:
        a: pulumi.Output[str]
        b: pulumi.Input[str]
        c: Optional[pulumi.Input[str]]
        d: pulumi.InputType[str]
        e: Optional[pulumi.InputType[str]]
        f: Optional[pulumi.Input[pulumi.InputType[str]]]

    # We always call `unwrap_type` with the forward references for `Output[T]` resolved.
    localns = {"Output": Output, "T": typing.TypeVar("T")}
    anno = typing.get_type_hints(A, localns=localns)

    assert _types.unwrap_type(anno["a"]) == str
    assert _types.unwrap_type(anno["b"]) == str
    assert _types.unwrap_type(anno["c"]) == str
    assert _types.unwrap_type(anno["d"]) == str
    assert _types.unwrap_type(anno["e"]) == str
    assert _types.unwrap_type(anno["f"]) == str
