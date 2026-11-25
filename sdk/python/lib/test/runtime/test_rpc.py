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

from collections import abc
from unittest.mock import Mock
import grpc
from pulumi.errors import RunError
from pulumi.runtime import rpc
from pulumi.runtime.settings import _ErrorAlreadyReported, grpc_error_to_exception
from typing import Any
import typing


def test_error_already_reported_trailer():
    error = Mock()
    error.code.return_value = grpc.StatusCode.UNAVAILABLE
    error.details.return_value = "resource monitor shut down"
    error.trailing_metadata.return_value = ()
    unmarked = grpc_error_to_exception(error)
    assert isinstance(unmarked, RunError)
    assert not isinstance(unmarked, _ErrorAlreadyReported)

    error.trailing_metadata.return_value = (("pulumi-error-already-reported", "false"),)
    assert not isinstance(grpc_error_to_exception(error), _ErrorAlreadyReported)

    error.trailing_metadata.return_value = (("pulumi-error-already-reported", "true"),)
    assert isinstance(grpc_error_to_exception(error), _ErrorAlreadyReported)
    error.code.return_value = grpc.StatusCode.UNKNOWN
    assert isinstance(grpc_error_to_exception(error), _ErrorAlreadyReported)


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
