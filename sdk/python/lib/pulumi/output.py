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
import asyncio
from inspect import isawaitable
from typing import (
    TypeVar,
    Generic,
    Set,
    Callable,
    Awaitable,
    Union,
    cast,
    Mapping,
    Any,
    TYPE_CHECKING
)

from . import runtime
from .runtime import known_types

if TYPE_CHECKING:
    from .resource import Resource

T = TypeVar('T')
U = TypeVar('U')

Input = Union[T, Awaitable[T], 'Output[T]']
Inputs = Mapping[str, Input[Any]]


@known_types.output
class Output(Generic[T]):
    """
    Output helps encode the relationship between Resources in a Pulumi application. Specifically an
    Output holds onto a piece of Data and the Resource it was generated from. An Output value can
    then be provided when constructing new Resources, allowing that new Resource to know both the
    value as well as the Resource the value came from.  This allows for a precise 'Resource
    dependency graph' to be created, which properly tracks the relationship between resources.
    """

    _is_known: Awaitable[bool]
    """
    Whether or not this 'Output' should actually perform .apply calls.  During a preview,
    an Output value may not be known (because it would have to actually be computed by doing an
    'update').  In that case, we don't want to perform any .apply calls as the callbacks
    may not expect an undefined value.  So, instead, we just transition to another Output
    value that itself knows it should not perform .apply calls.
    """

    _future: Awaitable[T]
    """
    Future that actually produces the concrete value of this output.
    """

    _resources: Set['Resource']
    """
    The list of resources that this output value depends on.
    """

    def __init__(self, resources: Set['Resource'], future: Awaitable[T], is_known: Awaitable[bool]) -> None:
        self._resources = resources
        self._future = future
        self._is_known = is_known

    def resources(self) -> Set['Resource']:
        return self._resources

    def future(self) -> Awaitable[T]:
        return self._future

    def apply(self, func: Callable[[T], Input[U]]) -> 'Output[U]':
        """
        Transforms the data of the output with the provided func.  The result remains a
        Output so that dependent resources can be properly tracked.

        'func' is not allowed to make resources.

        'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
        and you want to get a transitive dependency of it.  i.e.
        ```python
        d1: Output[SomeVal];
        d2 = d1.apply(lambda x: v.x.y.OtherOutput); # getting an output off of 'v'.
        (or, equivalently:)
        d2 = d1.x.y.OtherOutput
        ```

        In this example, taking a dependency on d2 means a resource will depend on all the resources
        of d1.  It will *not* depend on the resources of v.x.y.OtherDep.

        Importantly, the Resources that d2 feels like it will depend on are the same resources as d1.
        If you need have multiple Outputs and a single Output is needed that combines both
        set of resources, then 'pulumi.all' should be used instead.

        This function will only be called execution of a 'pulumi update' request.  It will not run
        during 'pulumi preview' (as the values of resources are of course not known then).
        """
        inner_is_known: asyncio.Future = asyncio.Future()

        # The "is_known" coroutine that we pass to the output we're about to create is derived from
        # the conjunction of the two is_knowns that we know about: our own (self._is_known) and a future
        # that we will resolve when running the apply.
        async def is_known() -> bool:
            inner = await inner_is_known
            known = await self._is_known
            return inner and known

        # The "run" coroutine actually runs the apply.
        async def run() -> U:
            try:
                value = await self._future
                if runtime.is_dry_run():
                    # During previews only perform the apply if the engine was able to
                    # give us an actual value for this Output.
                    apply_during_preview = await self._is_known
                    if not apply_during_preview:
                        # We didn't actually run the function, our new Output is definitely
                        # **not** known.
                        inner_is_known.set_result(False)
                        return cast(U, None)

                transformed: Input[U] = func(value)
                # Transformed is an Input, meaning there are three cases:
                #  1. transformed is an Output[U]
                if isinstance(transformed, Output):
                    # The inner Output is known if this returned output is known.
                    inner_is_known.set_result(await self._is_known)
                    return await transformed.future()

                #  2. transformed is an Awaitable[U]
                if isawaitable(transformed):
                    # The inner Output is known.
                    inner_is_known.set_result(True)
                    return await cast(Awaitable[U], transformed)

                #  3. transformed is U. It is trivially known.
                inner_is_known.set_result(True)
                return cast(U, transformed)
            finally:
                # Always resolve the future if it hasn't been done already.
                if not inner_is_known.done():
                    # Try and set the result. This might fail if we're shutting down,
                    # so swallow that error if that occurs.
                    try:
                        inner_is_known.set_result(False)
                    except RuntimeError:
                        pass

        run_fut = asyncio.ensure_future(run())
        is_known_fut = asyncio.ensure_future(is_known())
        return Output(self._resources, run_fut, is_known_fut)

    def __getattr__(self, item: str) -> 'Output[Any]':
        """
        Syntax sugar for retrieving attributes off of outputs.
        """
        return self.apply(lambda v: getattr(v, item))


    def __getitem__(self, key: Any) -> 'Output[Any]':
        """
        Syntax sugar for looking up attributes dynamically off of outputs.
        """
        return self.apply(lambda v: v[key])
