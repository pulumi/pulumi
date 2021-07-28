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
    Dict,
    Optional,
    TYPE_CHECKING,
    overload
)

from . import _types
from . import runtime
from .runtime import rpc

if TYPE_CHECKING:
    from .resource import Resource

T = TypeVar('T')
T1 = TypeVar('T1')
T2 = TypeVar('T2')
T_co = TypeVar('T_co', covariant=True)
U = TypeVar('U')

Input = Union[T, Awaitable[T], 'Output[T]']
Inputs = Mapping[str, Input[Any]]
InputType = Union[T, Mapping[str, Any]]


class Output(Generic[T_co]):
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

    _is_secret: Awaitable[bool]
    """
    Whether or not this 'Output' should be treated as containing secret data. Secret outputs are tagged when
    flowing across the RPC interface to the resource monitor, such that when they are persisted to disk in
    our state file, they are encrypted instead of being in plaintext.
    """

    _future: Awaitable[T_co]
    """
    Future that actually produces the concrete value of this output.
    """

    _resources: Awaitable[Set['Resource']]
    """
    The list of resources that this output value depends on.
    """

    def __init__(self, resources: Union[Awaitable[Set['Resource']], Set['Resource']],
                 future: Awaitable[T_co], is_known: Awaitable[bool],
                 is_secret: Optional[Awaitable[bool]] = None) -> None:
        is_known = asyncio.ensure_future(is_known)
        future = asyncio.ensure_future(future)

        async def is_value_known() -> bool:
            return await is_known and not contains_unknowns(await future)

        if isinstance(resources, set):
            self._resources = asyncio.Future()
            self._resources.set_result(resources)
        else:
            self._resources = asyncio.ensure_future(resources)

        self._future = future
        self._is_known = asyncio.ensure_future(is_value_known())

        if is_secret is not None:
            self._is_secret = asyncio.ensure_future(is_secret)
        else:
            self._is_secret = asyncio.Future()
            self._is_secret.set_result(False)

    # Private implementation details - do not document.
    def resources(self) -> Awaitable[Set['Resource']]:
        return self._resources

    def future(self, with_unknowns: Optional[bool] = None) -> Awaitable[Optional[T_co]]:
        # If the caller did not explicitly ask to see unknown values and the value of this output contains unnkowns,
        # return None. This preserves compatibility with earlier versios of the Pulumi SDK.
        async def get_value() -> Optional[T_co]:
            val = await self._future
            return None if not with_unknowns and contains_unknowns(val) else val
        return asyncio.ensure_future(get_value())

    def is_known(self) -> Awaitable[bool]:
        return self._is_known
    # End private implementation details.

    def is_secret(self) -> Awaitable[bool]:
        return self._is_secret

    def apply(self, func: Callable[[T_co], Input[U]], run_with_unknowns: Optional[bool] = None) -> 'Output[U]':
        """
        Transforms the data of the output with the provided func.  The result remains a
        Output so that dependent resources can be properly tracked.

        'func' is not allowed to make resources.

        'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
        and you want to get a transitive dependency of it.

        This function will be called during execution of a 'pulumi up' request.  It may not run
        during 'pulumi preview' (as the values of resources are of course may not be known then).

        :param Callable[[T_co],Input[U]] func: A function that will, given this Output's value, transform the value to
               an Input of some kind, where an Input is either a prompt value, a Future, or another Output of the given
               type.
        :return: A transformed Output obtained from running the transformation function on this Output's value.
        :rtype: Output[U]
        """
        result_resources: asyncio.Future[Set['Resource']] = asyncio.Future()
        result_is_known: asyncio.Future[bool] = asyncio.Future()
        result_is_secret: asyncio.Future[bool] = asyncio.Future()

        # The "run" coroutine actually runs the apply.
        async def run() -> U:
            try:
                # Await this output's details.
                resources = await self._resources
                is_known = await self._is_known
                is_secret = await self._is_secret
                value = await self._future

                if runtime.is_dry_run():
                    # During previews only perform the apply if the engine was able to give us an actual value for this
                    # Output or if the caller is able to tolerate unknown values.
                    apply_during_preview = is_known or run_with_unknowns

                    if not apply_during_preview:
                        # We didn't actually run the function, our new Output is definitely
                        # **not** known.
                        result_resources.set_result(resources)
                        result_is_known.set_result(False)
                        result_is_secret.set_result(is_secret)
                        return cast(U, None)

                    # If we are running with unknown values and the value is explicitly unknown but does not actually
                    # contain any unknown values, collapse its value to the unknown value. This ensures that callbacks
                    # that expect to see unknowns during preview in outputs that are not known will always do so.
                    if not is_known and run_with_unknowns and not contains_unknowns(value):
                        value = cast(T_co, UNKNOWN)

                transformed: Input[U] = func(value)
                # Transformed is an Input, meaning there are three cases:
                #  1. transformed is an Output[U]
                if isinstance(transformed, Output):
                    transformed_as_output = cast(Output[U], transformed)
                    # Forward along the inner output's _resources, _is_known and _is_secret values.
                    transformed_resources = await transformed_as_output._resources
                    result_resources.set_result(resources | transformed_resources)
                    result_is_known.set_result(await transformed_as_output._is_known)
                    result_is_secret.set_result(await transformed_as_output._is_secret or is_secret)
                    return await transformed.future(with_unknowns=True)

                #  2. transformed is an Awaitable[U]
                if isawaitable(transformed):
                    # Since transformed is not an Output, it is known.
                    result_resources.set_result(resources)
                    result_is_known.set_result(True)
                    result_is_secret.set_result(is_secret)
                    return await cast(Awaitable[U], transformed)

                #  3. transformed is U. It is trivially known.
                result_resources.set_result(resources)
                result_is_known.set_result(True)
                result_is_secret.set_result(is_secret)
                return cast(U, transformed)
            finally:
                # Always resolve the future if it hasn't been done already.
                if not result_is_known.done():
                    # Try and set the result. This might fail if we're shutting down,
                    # so swallow that error if that occurs.
                    try:
                        result_resources.set_result(resources)
                        result_is_known.set_result(False)
                        result_is_secret.set_result(False)
                    except RuntimeError:
                        pass

        run_fut = asyncio.ensure_future(run())
        return Output(result_resources, run_fut, result_is_known, result_is_secret)

    def __getattr__(self, item: str) -> 'Output[Any]': # type: ignore
        """
        Syntax sugar for retrieving attributes off of outputs.

        :param str item: An attribute name.
        :return: An Output of this Output's underlying value's property with the given name.
        :rtype: Output[Any]
        """
        def lift(v: Any) -> Any:
            return UNKNOWN if isinstance(v, Unknown) else getattr(v, item)
        return self.apply(lift, True)

    def __getitem__(self, key: Any) -> 'Output[Any]':
        """
        Syntax sugar for looking up attributes dynamically off of outputs.

        :param Any key: Key for the attribute dictionary.
        :return: An Output of this Output's underlying value, keyed with the given key as if it were a dictionary.
        :rtype: Output[Any]
        """
        def lift(v: Any) -> Any:
            return UNKNOWN if isinstance(v, Unknown) else cast(Any, v)[key]
        return self.apply(lift, True)

    def __iter__(self) -> Any:
        """
        Output instances are not iterable, but since they implement __getitem__ we need to explicitly prevent
        iteration by implementing __iter__ to raise a TypeError.
        """
        raise TypeError("'Output' object is not iterable, consider iterating the underlying value inside an 'apply'")

    @staticmethod
    def from_input(val: Input[T_co]) -> 'Output[T_co]':
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values through nested
        lists, dicts, and input classes.  Nested objects of other types (including Resources) are not deeply unwrapped.

        :param Input[T_co] val: An Input to be converted to an Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values.
        :rtype: Output[T_co]
        """

        # Is it an output already? Recurse into the value contained within it.
        if isinstance(val, Output):
            return val.apply(Output.from_input, True)

        # Is it an input type (i.e. args class)? Recurse into the values within.
        typ = type(val)
        if _types.is_input_type(typ):
            # We know that any input type can safely be decomposed into it's `__dict__`, and then reconstructed
            # via `type(**d)` from the (unwrapped) properties.
            o_typ: Output[typ] = Output.all(**val.__dict__).apply(lambda d: typ(**d)) # type:ignore
            return cast(Output[T_co], o_typ)

        # Is a (non-empty) dict or list? Recurse into the values within them.
        if val and isinstance(val, dict):
            o_dict: Output[dict] = Output.all(**val)
            return cast(Output[T_co], o_dict)

        if val and isinstance(val, list):
            o_list: Output[list] = Output.all(*val)
            return cast(Output[T_co], o_list)

        # If it's not an output, list, or dict, it must be known and not secret
        is_known_fut: asyncio.Future[bool] = asyncio.Future()
        is_secret_fut: asyncio.Future[bool] = asyncio.Future()
        is_known_fut.set_result(True)
        is_secret_fut.set_result(False)

        # Is it awaitable? If so, schedule it for execution and use the resulting future
        # as the value future for a new output.
        if isawaitable(val):
            val_fut = cast(asyncio.Future, val)
            promise_output = Output(set(), asyncio.ensure_future(val_fut), is_known_fut, is_secret_fut)
            return promise_output.apply(Output.from_input, True)

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        value_fut: asyncio.Future[Any] = asyncio.Future()
        value_fut.set_result(val)
        return Output(set(), value_fut, is_known_fut, is_secret_fut)

    @staticmethod
    def _from_input_shallow(val: Input[T]) -> 'Output[T]':
        """
        Like `from_input`, but does not recur deeply. Instead, checks if `val` is an `Output` value
        and returns it as is. Otherwise, promotes a known value or future to `Output`.

        :param Input[T] val: An Input to be converted to an Output.
        :return: An Output corresponding to `val`.
        :rtype: Output[T]
        """

        if isinstance(val, Output):
            return val

        # If it's not an output, it must be known and not secret
        is_known_fut: asyncio.Future[bool] = asyncio.Future()
        is_secret_fut: asyncio.Future[bool] = asyncio.Future()
        is_known_fut.set_result(True)
        is_secret_fut.set_result(False)

        if isawaitable(val):
            val_fut = cast(asyncio.Future, val)
            return Output(set(), asyncio.ensure_future(val_fut), is_known_fut, is_secret_fut)

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        value_fut: asyncio.Future[Any] = asyncio.Future()
        value_fut.set_result(val)
        return Output(set(), value_fut, is_known_fut, is_secret_fut)

    @staticmethod
    def unsecret(val: 'Output[T]') -> 'Output[T]':
        """
        Takes an existing Output, deeply unwraps the nested values and returns a new Output without any secrets included

        :param Output[T] val: An Output to be converted to a non-Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any secret values.
        :rtype: Output[T]
        """
        is_secret: asyncio.Future[bool] = asyncio.Future()
        is_secret.set_result(False)
        return Output(val._resources, val._future, val._is_known, is_secret)

    @staticmethod
    def secret(val: Input[T]) -> 'Output[T]':
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values as necessary
        given the type. It also marks the returned Output as a secret, so its contents will be persisted in an encrypted
        form in state files.

        :param Input[T] val: An Input to be converted to an Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values and is marked as a Secret.
        :rtype: Output[T]
        """

        o = Output.from_input(val)
        is_secret: asyncio.Future[bool] = asyncio.Future()
        is_secret.set_result(True)
        return Output(o._resources, o._future, o._is_known, is_secret)

    # According to mypy these overloads unsafely overlap, so we ignore the type check.
    # https://mypy.readthedocs.io/en/stable/more_types.html#type-checking-the-variants:~:text=considered%20unsafely%20overlapping
    @overload
    @staticmethod
    def all(*args: Input[T]) -> 'Output[List[T]]':  # type: ignore
        ...

    @overload
    @staticmethod
    def all(**kwargs: Input[T]) -> 'Output[Dict[str, T]]':
        ...

    @staticmethod
    def all(*args: Input[T], **kwargs: Input[T]):
        """
        Produces an Output of a list (if args i.e a list of inputs are supplied)
        or dict (if kwargs i.e. keyworded arguments are supplied).

        This function can be used to combine multiple, separate Inputs into a single
        Output which can then be used as the target of `apply`. Resource dependencies
        are preserved in the returned Output.

        Examples::

            Output.all(foo, bar) -> Output[[foo, bar]]
            Output.all(foo=foo, bar=bar) -> Output[{"foo": foo, "bar": bar}]

        :param Input[T] args: A list of Inputs to convert.
        :param Input[T] kwargs: A list of named Inputs to convert.
        :return: An output of list or dict, converted from unnamed or named Inputs respectively.
        """

        # Three asynchronous helper functions to assist in the implementation:
        # is_known, which returns True if all of the input's values are known,
        # and false if any of them are not known,
        async def is_known(outputs: list):
            is_known_futures = [o._is_known for o in outputs]
            each_is_known = await asyncio.gather(*is_known_futures)
            return all(each_is_known)

        # is_secret, which returns True if any of the input values are secret, and
        # false if none of them are secret.
        async def is_secret(outputs: list):
            is_secret_futures = [o._is_secret for o in outputs]
            each_is_secret = await asyncio.gather(*is_secret_futures)
            return any(each_is_secret)

        async def get_resources(outputs: list):
            resources_futures = [o._resources for o in outputs]
            resources_agg = await asyncio.gather(*resources_futures)
            # Merge the list of resource dependencies across all inputs.
            return reduce(lambda acc, r: acc.union(r), resources_agg, set())

        # gather_futures, which aggregates the list or dict of futures in each input to a future of a list or dict.
        async def gather_futures(outputs: Union[dict, list]):
            if isinstance(outputs, list):
                value_futures_list = [asyncio.ensure_future(o.future(with_unknowns=True)) for o in outputs]
                return await asyncio.gather(*value_futures_list)
            value_futures_dict = {k: asyncio.ensure_future(v.future(with_unknowns=True)) for k, v in outputs.items()}
            return await _gather_from_dict(value_futures_dict)
        from_input = cast(Callable[[Union[T, Awaitable[T], Output[T]]], Output[T]], Output.from_input)

        if args and kwargs:
            raise ValueError("Output.all() was supplied a mix of named and unnamed inputs")
        # First, map all inputs to outputs using `from_input`.
        all_outputs: Union[list, dict] = (
            {k: from_input(v) for k, v in kwargs.items()} if kwargs
            else [from_input(x) for x in args])

        # Aggregate the list or dict of futures into a future of list or dict.
        value_futures = asyncio.ensure_future(gather_futures(all_outputs))

        # Aggregate whether or not this output is known.
        output_values = [all_outputs[k] for k in all_outputs] if isinstance(all_outputs, dict) else all_outputs
        resources_futures = asyncio.ensure_future(get_resources(output_values))
        known_futures = asyncio.ensure_future(is_known(output_values))
        secret_futures = asyncio.ensure_future(is_secret(output_values))
        return Output(resources_futures, value_futures, known_futures, secret_futures)

    @staticmethod
    def concat(*args: Input[str]) -> 'Output[str]':
        """
        Concatenates a collection of Input[str] into a single Output[str].

        This function takes a sequence of Input[str], stringifies each, and concatenates all values
        into one final string. This can be used like so:

            url = Output.concat("http://", server.hostname, ":", loadBalancer.port)

        :param Input[str] args: A list of string Inputs to concatenate.
        :return: A concatenated output string.
        :rtype: Output[str]
        """

        transformed_items: List[Input[Any]] = [Output.from_input(v) for v in args]
        # invariant http://mypy.readthedocs.io/en/latest/common_issues.html#variance
        return Output.all(*transformed_items).apply("".join)  # type: ignore


class Unknown:
    """
    Unknown represents a value that is unknown.
    """

    def __init__(self):
        pass


UNKNOWN = Unknown()
"""
UNKNOWN is the singleton unknown value.
"""


def contains_unknowns(val: Any) -> bool:
    return rpc.contains_unknowns(val)


def _is_prompt(value: Input[T]) -> bool:
    """Checks if the value is prompty available."""

    return not isawaitable(value) and not isinstance(value, Output)


def _map_output(o: Output[T], transform: Callable[[T],U]) -> Output[U]:
    """Transforms an output's result value with a pure function."""

    async def fut() -> U:
        value = await o.future()
        return transform(value) if value is not None else cast(U, UNKNOWN)

    return Output(resources=o.resources(),
                  future=asyncio.ensure_future(fut()),
                  is_known=o.is_known(),
                  is_secret=o.is_secret())


def _map2_output(o1: Output[T1], o2: Output[T2], transform: Callable[[T1,T2],U]) -> Output[U]:
    """
    Joins two outputs and transforms their result with a pure function.
    Similar to `all` but does not deeply await.
    """

    async def fut() -> U:
        v1 = await o1.future()
        v2 = await o2.future()
        return transform(v1, v2) if (v1 is not None) and (v2 is not None) else cast(U, UNKNOWN)

    async def res() -> Set['Resource']:
        r1 = await o1.resources()
        r2 = await o2.resources()
        return r1 | r2

    return Output(resources=asyncio.ensure_future(res()),
                  future=asyncio.ensure_future(fut()),
                  is_known=o1.is_known() and o2.is_known(),
                  is_secret=o2.is_secret() or o2.is_secret())


def _map_input(i: Input[T], transform: Callable[[T],U]) -> Input[U]:
    """Transforms an input's result value with a pure function."""

    if _is_prompt(i):
        return transform(cast(T, i))

    if isawaitable(i):
        inp = cast(Awaitable[T], i)

        async def fut() -> U:
            return transform(await inp)

        return asyncio.ensure_future(fut())

    return _map_output(cast(Output[T], i), transform)


def _map2_input(i1: Input[T1], i2: Input[T2], transform: Callable[[T1,T2],U]) -> Input[U]:
    """
    Joins two inputs and transforms their result with a pure function.
    """

    if _is_prompt(i1):
        v1 = cast(T1, i1)
        return _map_input(i2, lambda v2: transform(v1, v2))

    if _is_prompt(i2):
        v2 = cast(T2, i2)
        return _map_input(i1, lambda v1: transform(v1, v2))

    if isawaitable(i1) and isawaitable(i2):
        a1 = cast(Awaitable[T1], i1)
        a2 = cast(Awaitable[T2], i2)

        async def join() -> U:
            v1 = await a1
            v2 = await a2
            return transform(v1, v2)

        return asyncio.ensure_future(join())

    return _map2_output(Output._from_input_shallow(i1),
                        Output._from_input_shallow(i2),
                        transform)


async def _gather_from_dict(tasks: dict) -> dict:
    results = await asyncio.gather(*tasks.values())
    return dict(zip(tasks.keys(), results))
