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
from functools import reduce
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
    List,
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

    # Private implementation details - do not document.
    def resources(self) -> Set['Resource']:
        return self._resources

    def future(self) -> Awaitable[T]:
        return self._future
    # End private implementation details.

    def apply(self, func: Callable[[T], Input[U]]) -> 'Output[U]':
        """
        Transforms the data of the output with the provided func.  The result remains a
        Output so that dependent resources can be properly tracked.

        'func' is not allowed to make resources.

        'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
        and you want to get a transitive dependency of it.

        This function will be called during execution of a 'pulumi update' request.  It may not run
        during 'pulumi preview' (as the values of resources are of course may not be known then).

        :param Callable[[T],Input[U]] func: A function that will, given this Output's value, transform the value to
               an Input of some kind, where an Input is either a prompt value, a Future, or another Output of the given
               type.
        :return: A transformed Output obtained from running the transformation function on this Output's value.
        :rtype: Output[U]
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

        :param str item: An attribute name.
        :return: An Output of this Output's underlying value's property with the given name.
        :rtype: Output[Any]
        """
        return self.apply(lambda v: getattr(v, item))


    def __getitem__(self, key: Any) -> 'Output[Any]':
        """
        Syntax sugar for looking up attributes dynamically off of outputs.

        :param Any key: Key for the attribute dictionary.
        :return: An Output of this Output's underlying value, keyed with the given key as if it were a dictionary.
        :rtype: Output[Any]
        """
        return self.apply(lambda v: v[key])

    @staticmethod
    def from_input(val: Input[T]) -> 'Output[T]':
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values as necessary
        given the type.

        :param Input[T] val: An Input to be converted to an Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values.
        :rtype: Output[T]
        """
        # Is it an output already? Recurse into the value contained within it.
        if isinstance(val, Output):
            return val.apply(Output.from_input)

        # Is a dict or list? Recurse into the values within them.
        if isinstance(val, dict):
            # Since Output.all works on lists early, serialize this dictionary into a list of lists first.
            # Once we have a output of the list of properties, we can use an apply to re-hydrate it back into a dict.
            transformed_items = [[k, Output.from_input(v)] for k, v in val.items()]
            return Output.all(*transformed_items).apply(lambda props: {k: v for k, v in props})

        if isinstance(val, list):
            transformed_items = [Output.from_input(v) for v in val]
            return Output.all(*transformed_items)

        # If it's not an output, list, or dict, it must be known.
        is_known_fut = asyncio.Future()
        is_known_fut.set_result(True)

        # Is it awaitable? If so, schedule it for execution and use the resulting future
        # as the value future for a new output.
        if isawaitable(val):
            promise_output = Output(set(), asyncio.ensure_future(val), is_known_fut)
            return promise_output.apply(Output.from_input)

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        value_fut = asyncio.Future()
        value_fut.set_result(val)
        return Output(set(), value_fut, is_known_fut)

    @staticmethod
    def all(*args: List[Input[T]]) -> 'Output[List[T]]':
        """
        Produces an Output of Lists from a List of Inputs.

        This function can be used to combine multiple, separate Inputs into a single
        Output which can then be used as the target of `apply`. Resource dependencies
        are preserved in the returned Output.

        :param List[Input[T]] args: A list of Inputs to convert.
        :return: An output of lists, converted from an Input to prompt values.
        :rtype: Output[List[T]]
        """

        # Two asynchronous helper functions to assist in the implementation:
        # is_known, which returns True if all of the input's values are known,
        # and false if any of them are not known,
        async def is_known(outputs):
            is_known_futures = list(map(lambda o: o._is_known, outputs))
            each_is_known = await asyncio.gather(*is_known_futures)
            return all(each_is_known)

        # gather_futures, which aggregates the list of futures in each input to a future of a list.
        async def gather_futures(outputs):
            value_futures = list(map(lambda o: asyncio.ensure_future(o.future()), outputs))
            return await asyncio.gather(*value_futures)

        # First, map all inputs to outputs using `from_input`.
        all_outputs = list(map(Output.from_input, args))

        # Merge the list of resource dependencies across all inputs.
        resources = reduce(lambda acc, r: acc.union(r.resources()), all_outputs, set())

        # Aggregate the list of futures into a future of lists.
        value_futures = asyncio.ensure_future(gather_futures(all_outputs))

        # Aggregate whether or not this output is known.
        known_futures = asyncio.ensure_future(is_known(all_outputs))
        return Output(resources, value_futures, known_futures)
