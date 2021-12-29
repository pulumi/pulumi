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

from typing import TYPE_CHECKING, Set, Iterable

if TYPE_CHECKING:
    from .. import Resource


_DEPENDENCIES_PROPERTY = '_direct_computed_dependencies'


def declare_dependency(from_resource: 'Resource', to_resource: 'Resource') -> bool:
    """Remembers that `from_resource` depends on `to_resource`, unless
       adding this dependency would form a cycle to the known
       dependency graph. Returns `True` if successful, `False` if
       skipped due to cycles.

    """

    if _reachable(from_resource=to_resource,
                  to_resource=from_resource):
        return False

    _add_dep(from_resource, to_resource)
    return True


def _deps(res: 'Resource') -> Set['Resource']:
    return getattr(res, _DEPENDENCIES_PROPERTY, set())


def _add_dep(from_resource: 'Resource', to_resource: 'Resource') -> None:
    return setattr(from_resource,
                   _DEPENDENCIES_PROPERTY,
                   _deps(from_resource) | set([to_resource]))


def _reachable(from_resource: 'Resource', to_resource: 'Resource') -> bool:
    visited: Set['Resource'] = set()

    for x in _with_transitive_deps(from_resource, visited):
        if x == to_resource:
            return True

    return False


def _with_transitive_deps(r: 'Resource', visited: Set['Resource']) -> Iterable['Resource']:
    if r in visited:
        return

    visited.add(r)
    yield r

    for x in _deps(r):
        for y in _with_transitive_deps(x, visited):
            yield y
