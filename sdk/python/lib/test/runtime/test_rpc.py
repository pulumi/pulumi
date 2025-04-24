# Copyright 2016-2021, Pulumi Corporation.
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

from collections import abc
from pulumi.runtime import rpc
from typing import Any
import typing


def test_get_list_element_type():
    assert rpc._get_list_element_type(None) is None

    assert rpc._get_list_element_type(list) == Any
    assert rpc._get_list_element_type(abc.Sequence) == Any
    assert rpc._get_list_element_type(typing.List) == Any
    assert rpc._get_list_element_type(typing.Sequence) == Any

    assert rpc._get_list_element_type(typing.List[Any]) == Any
    assert rpc._get_list_element_type(typing.Sequence[Any]) == Any

    assert rpc._get_list_element_type(typing.List[int]) == int
    assert rpc._get_list_element_type(typing.Sequence[int]) == int

    assert rpc._get_list_element_type(typing.List[typing.List[int]]) == typing.List[int]
    assert (
        rpc._get_list_element_type(typing.Sequence[typing.Sequence[int]])
        == typing.Sequence[int]
    )

    assert rpc._get_list_element_type(typing.List[typing.List]) == typing.List
