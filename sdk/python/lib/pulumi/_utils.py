# Copyright 2016-2020, Pulumi Corporation.
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

import typing


# Empty function definitions.

def _empty():
    ...

def _empty_doc():
    """Empty function docstring."""
    ...

_empty_lambda = lambda: None

_empty_lambda_doc = lambda: None
_empty_lambda_doc.__doc__ = """Empty lambda docstring."""


def _consts(fn: typing.Callable) -> tuple:
    """
    Returns a tuple of the function's constants excluding the docstring.
    """
    return tuple(x for x in fn.__code__.co_consts if x != fn.__doc__)


# Precompute constants for each of the empty functions.
_consts_empty = _consts(_empty)
_consts_empty_doc = _consts(_empty_doc)
_consts_empty_lambda = _consts(_empty_lambda)
_consts_empty_lambda_doc = _consts(_empty_lambda_doc)


def is_empty_function(fn: typing.Callable) -> bool:
    """
    Returns true if the function is empty.
    """
    consts = _consts(fn)
    return (
        (fn.__code__.co_code == _empty.__code__.co_code and consts == _consts_empty) or
        (fn.__code__.co_code == _empty_doc.__code__.co_code and consts == _consts_empty_doc) or
        (fn.__code__.co_code == _empty_lambda.__code__.co_code and consts == _consts_empty_lambda) or
        (fn.__code__.co_code == _empty_lambda_doc.__code__.co_code and consts == _consts_empty_lambda_doc)
    )
