# Copyright 2016-2018, Pulumi Corporation.
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
from typing import TypeVar, Generic, Set, Callable, Awaitable, Union, cast, Mapping, Any
from inspect import isawaitable
from .resource import Resource

T = TypeVar('T')
U = TypeVar('U')


class Output(Generic[T]):
    _is_known: Awaitable[bool]
    _future: Awaitable[T]
    _resources: Set[Resource]

    def __init__(self, resources: Set[Resource], future: Awaitable[T], is_known: Awaitable[bool]) -> None:
        self._resources = resources
        self._future = future
        self._is_known = is_known

    def resources(self) -> Set[Resource]:
        return self._resources

    def future(self) -> Awaitable[T]:
        return self._future

    async def apply(self, func: Callable[[T], Union[U, Awaitable[U], 'Output'[U]]]) -> 'Output'[U]:
        async def result_is_known() -> bool:
            is_known = await self._is_known
            return is_known

        async def inner(value: T) -> U:
            transformed = func(value)
            if isinstance(transformed, Output):
                return await transformed._future

            if isawaitable(transformed):
                return await cast(Awaitable[U], transformed)

            return cast(U, transformed)

        inner_value = await self._future
        return Output(self._resources, inner(inner_value), result_is_known())


Input = Union[T, Awaitable[T], Output[T]]
Inputs = Mapping[str, Input[Any]]
