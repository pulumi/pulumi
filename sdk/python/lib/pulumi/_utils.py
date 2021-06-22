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


import importlib
import sys
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


def lazy_import(fullname):
    """Defers module import until first attribute access. For example:

    import a.b.c as x

    Becomes:

    x = lazy_import('a.b.c')

    The code started from the official Python example:

    https://github.com/python/cpython/blob/master/Doc/library/importlib.rst#implementing-lazy-imports

    This example is extended by early returns to support import cycles
    and registration of sub-modules as attributes.
    """

    # Return early if already registered; this supports import cycles.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    spec = importlib.util.find_spec(fullname)

    # Return early if find_spec has recursively called lazy_import
    # again and pre-populated the sys.modules slot; an example of this
    # is covered by test_lazy_import.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    loader = importlib.util.LazyLoader(spec.loader)
    spec.loader = loader
    module = importlib.util.module_from_spec(spec)

    # Return early rather than overwriting the sys.modules slot.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    sys.modules[fullname] = module
    loader.exec_module(module)
    return module
