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
from typing import TYPE_CHECKING, List, Optional, Set

from ..output import Output
from .rpc import _expand_dependencies

if TYPE_CHECKING:
    from ..output import Input
    from ..resource import Resource


async def _resolve_depends_on(
    depends_on: "Input[List[Input[Resource]]]",
) -> Set["Resource"]:
    """
    Resolves the set of all dependent resources implied by `depends_on`.
    """

    if not depends_on:
        return set()

    outer = Output._from_input_shallow(depends_on)
    all_deps = await outer.resources()
    inner_list = await outer.future() or []

    for i in inner_list:
        inner = Output.from_input(i)
        more_deps = await inner.resources()
        all_deps = all_deps | more_deps
        direct_dep = await inner.future()
        if direct_dep is not None:
            all_deps.add(direct_dep)

    return all_deps


async def _resolve_depends_on_urns(
    depends_on: "Input[List[Input[Resource]]]",
    from_resource: Optional["Resource"] = None,
) -> Set[str]:
    """
    Resolves the set of all dependent resources implied by
    `depends_on`, either directly listed or implied in the Input
    layer. Returns a deduplicated URN list.
    """

    if not depends_on:
        return set()

    all_deps = await _resolve_depends_on(depends_on)

    return await _expand_dependencies(all_deps, from_resource)
