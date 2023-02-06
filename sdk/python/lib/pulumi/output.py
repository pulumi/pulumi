# Copyright 2016-2022, Pulumi Corporation.
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
import contextlib
import json
from functools import reduce
from inspect import isawaitable
from typing import (
    TYPE_CHECKING,
    Any,
    Awaitable,
    Callable,
    Dict,
    Generic,
    List,
    Mapping,
    Optional,
    Set,
    Tuple,
    Type,
    TypeVar,
    Union,
    cast,
    overload,
)

from . import _types, runtime
from .runtime import rpc
from .runtime.sync_await import _sync_await

if TYPE_CHECKING:
    from .resource import Resource

T = TypeVar("T")
T1 = TypeVar("T1")
T2 = TypeVar("T2")
T3 = TypeVar("T3")
T_co = TypeVar("T_co", covariant=True)
U = TypeVar("U")

Input = Union[T, Awaitable[T], "Output[T]"]
Inputs = Mapping[str, Input[Any]]
InputType = Union[T, Mapping[str, Any]]


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


class OutputData(Generic[T]):
    """
    This is an advanced type used to report back internal details of an Output.
    """

    resources: Set["Resource"]
    """
    The list of resources that the output value depends on.
    """
    value: Union[T, Unknown]
    """
    The concrete value of the output if known; else UNKNOWN.

    If Unknown the output should not actually perform .apply calls. During a preview, an output value may not
    be known (because it would have to actually be computed by doing an 'update'). In that case, we don't
    want to perform any .apply calls as the callbacks may not expect an undefined value. So, instead, we just
    transition to another output value that itself knows it should not perform .apply calls.
    """
    secret: bool
    """
    Whether or not the output should be treated as containing secret data. Secret outputs are tagged when
    flowing across the RPC interface to the resource monitor, such that when they are persisted to disk in our
    state file, they are encrypted instead of being in plaintext.
    """

    def __init__(
        self,
        resources: Set["Resource"],
        value: T | Unknown,
        secret: Optional[bool] = None,
    ) -> None:
        self.resources = resources
        self.value = value
        self.secret = False if secret is None else secret


class Output(Generic[T_co]):
    """
    Output helps encode the relationship between Resources in a Pulumi application. Specifically an
    Output holds onto a piece of Data and the Resource it was generated from. An Output value can
    then be provided when constructing new Resources, allowing that new Resource to know both the
    value as well as the Resource the value came from.  This allows for a precise 'Resource
    dependency graph' to be created, which properly tracks the relationship between resources.
    """

    _data: asyncio.Task[OutputData[T_co]]
    """
    The future internal data for this Output.
    """

    def __init__(
        self,
        resources_or_data: Union[
            Awaitable[OutputData], Awaitable[Set["Resource"]], Set["Resource"]
        ],
        future: Optional[Awaitable[T_co]] = None,
        is_known: Optional[Awaitable[bool]] = None,
        is_secret: Optional[Awaitable[bool]] = None,
    ) -> None:
        # Support old code that used to send 3/4 awaitables to set the output
        if future is not None:

            async def get_data() -> OutputData:
                if isinstance(resources_or_data, set):
                    resources = resources_or_data
                else:
                    r = await resources_or_data
                    assert isinstance(r, set)
                    resources = r

                assert future is not None
                value = await future

                assert is_known is not None
                known = await is_known

                secret = False
                if is_secret is not None:
                    secret = await is_secret

                return OutputData(resources, value if known else UNKNOWN, secret)

            self._data = asyncio.ensure_future(get_data())
        else:
            # New style code just sends the one future which should be of OutputData
            self._data = asyncio.ensure_future(
                cast(Awaitable[OutputData], resources_or_data)
            )

    # Private implementation details - do not document.
    async def resources(self) -> Set["Resource"]:
        data = await self._data
        return data.resources

    @overload
    async def future(self) -> Optional[T_co]:
        ...

    @overload
    async def future(self, with_unknowns: bool) -> Optional[T_co] | Unknown:
        ...

    async def future(
        self, with_unknowns: Optional[bool] = None
    ) -> Optional[T_co] | Unknown:
        data = await self._data
        if isinstance(data.value, Unknown):
            # If the caller did not explicitly ask to see unknown values and the value of this output contains
            # unknowns, return None. This preserves compatibility with earlier versions of the Pulumi SDK.
            return UNKNOWN if with_unknowns else None
        return data.value

    async def is_known(self) -> bool:
        data = await self._data
        return not isinstance(data.value, Unknown)

    # End private implementation details.

    async def is_secret(self) -> bool:
        data = await self._data
        return data.secret

    def apply(
        self, func: Callable[[T_co], Input[U]], run_with_unknowns: Optional[bool] = None
    ) -> "Output[U]":
        """
        Transforms the data of the output with the provided func.  The result remains an
        Output so that dependent resources can be properly tracked.

        'func' is not allowed to make resources.

        'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
        and you want to get a transitive dependency of it.

        This function will be called during execution of a `pulumi up` request.  It may not run
        during `pulumi preview` (as the values of resources are of course may not be known then).

        :param Callable[[T_co],Input[U]] func: A function that will, given this Output's value, transform the value to
               an Input of some kind, where an Input is either a prompt value, a Future, or another Output of the given
               type.
        :return: A transformed Output obtained from running the transformation function on this Output's value.
        :rtype: Output[U]
        """

        # The "run" coroutine actually runs the apply.
        async def run() -> OutputData[U]:
            # Await this output's details.
            data = await self._data

            if isinstance(data.value, Unknown) and not run_with_unknowns:
                # We can't run the apply because the value isn't known
                return OutputData(data.resources, UNKNOWN, data.secret)

            # Because of run_with_unknowns the value we pass to func might not actually be T_co, but this
            # supports old SDKs that used this feature.
            transformed = func(cast(T_co, data.value))
            # Transformed is an Input, meaning there are three cases:
            #  1. transformed is an Output[U]
            if isinstance(transformed, Output):
                # Forward along the inner output's resources, value, and secret values.
                transformed_data = await transformed._data

                result_resources = data.resources | transformed_data.resources
                result_secret = data.secret | transformed_data.secret
                return OutputData(
                    result_resources,
                    cast(U, transformed_data.value),
                    result_secret,
                )

            #  2. transformed is an Awaitable[U]
            if isawaitable(transformed):
                # Since transformed is not an Output, it is known.
                transformed_value = cast(U, await transformed)
                return OutputData(data.resources, transformed_value, data.secret)

            #  3. transformed is U. It is trivially known.
            return OutputData(data.resources, cast(U, transformed), data.secret)

        return Output(asyncio.ensure_future(run()))

    def __getattr__(self, item: str) -> "Output[Any]":  # type: ignore
        """
        Syntax sugar for retrieving attributes off of outputs.

        :param str item: An attribute name.
        :return: An Output of this Output's underlying value's property with the given name.
        :rtype: Output[Any]
        """
        # mypy warns about "Cannot use a covariant type variable as a parameter" for this use of apply, that's
        # because Output is covariant so given Derived a subtype of Base this may be called on an Output[Base]
        # but be an instance of Output[Derived]. In that case v is typed as Base, but is actually a Derived
        # and it would be an error if we wrote any other derived type of Base to the variable. But we're not
        # doing that here, we're only treating v purely as a readonly value.
        return self.apply(lambda v: getattr(v, item))  # type: ignore

    def __getitem__(self, key: Any) -> "Output[Any]":
        """
        Syntax sugar for looking up attributes dynamically off of outputs.

        :param Any key: Key for the attribute dictionary.
        :return: An Output of this Output's underlying value, keyed with the given key as if it were a dictionary.
        :rtype: Output[Any]
        """
        # See the comment in __getattr__ for why we ignore the type here.
        return self.apply(lambda v: v[key])  # type: ignore

    def __iter__(self) -> Any:
        """
        Output instances are not iterable, but since they implement __getitem__ we need to explicitly prevent
        iteration by implementing __iter__ to raise a TypeError.
        """
        raise TypeError(
            "'Output' object is not iterable, consider iterating the underlying value inside an 'apply'"
        )

    @staticmethod
    def from_input(val: Input[T_co]) -> "Output[T_co]":
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values through nested
        lists, dicts, and input classes.  Nested objects of other types (including Resources) are not deeply unwrapped.

        :param Input[T_co] val: An Input to be converted to an Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values.
        :rtype: Output[T_co]
        """

        # Is it an output already? Recurse into the value contained within it.
        if isinstance(val, Output):
            return val.apply(Output.from_input)

        # Is it an input type (i.e. args class)? Recurse into the values within.
        typ = type(val)
        if _types.is_input_type(typ):
            # We know that any input type can safely be decomposed into it's `__dict__`, and then reconstructed
            # via `type(**d)` from the (unwrapped) properties (bar empty input types, see next comment).
            o_typ = Output.all(**val.__dict__).apply(
                # if __dict__ was empty `all` will return an empty list object rather than a dict object,
                # there isn't really a good way to express this in mypy so the type checker doesn't pickup on
                # this. If we get an empty list we can't splat it as that results in a type error, so check
                # that we have some values before splatting. If it's empty just call the `typ` constructor
                # directly with no arguments.
                lambda d: typ(**d)
                if d
                else typ()
            )
            return cast(Output[T_co], o_typ)

        # Is a (non-empty) dict or list? Recurse into the values within them.
        if val and isinstance(val, dict):
            # The keys themselves might be outputs, so we can't just pass `**val` to all.

            # keys() and values() will be in the same order: https://docs.python.org/3/library/stdtypes.html#dictionary-view-objects
            keys = list(val.keys())
            values = list(val.values())

            def liftValues(keys: List[Any]):
                d = {keys[i]: values[i] for i in range(len(keys))}
                return Output.all(**d)

            o_dict: Output[dict] = Output.all(*keys).apply(liftValues)
            return cast(Output[T_co], o_dict)

        if val and isinstance(val, list):
            o_list: Output[list] = Output.all(*val)
            return cast(Output[T_co], o_list)

        # If it's not an output, list, or dict, it must be known and not secret

        # Is it awaitable? If so, schedule it for execution and use the resulting future
        # as the value future for a new output.
        if isawaitable(val):

            async def get_data(val: Awaitable[T]) -> OutputData[T]:
                o: Output[T] = Output.from_input(val)
                return await o._data

            return Output(get_data(val))

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        data_future: asyncio.Future[OutputData[T_co]] = asyncio.Future()
        data_future.set_result(OutputData(set(), cast(T_co, val), False))
        return Output(data_future)

    @staticmethod
    def _from_input_shallow(val: Input[T]) -> "Output[T]":
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
        if isawaitable(val):

            async def get_data(val: Awaitable[T]) -> OutputData[T]:
                o: Output[T] = Output.from_input(await val)
                return await o._data

            return Output(get_data(val))

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        data_future: asyncio.Future[OutputData[T_co]] = asyncio.Future()
        data_future.set_result(OutputData(set(), cast(T_co, val), False))
        return Output(data_future)

    @staticmethod
    def unsecret(val: "Output[T]") -> "Output[T]":
        """
        Takes an existing Output, deeply unwraps the nested values and returns a new Output without any secrets included

        :param Output[T] val: An Output to be converted to a non-Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any secret values.
        :rtype: Output[T]
        """

        async def get_data() -> OutputData[T]:
            data = await val._data
            return OutputData(data.resources, data.value, False)

        return Output(get_data())

    @staticmethod
    def secret(val: Input[T]) -> "Output[T]":
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values as necessary
        given the type. It also marks the returned Output as a secret, so its contents will be persisted in an encrypted
        form in state files.

        :param Input[T] val: An Input to be converted to an Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values and is marked as a Secret.
        :rtype: Output[T]
        """

        async def get_data() -> OutputData[T]:
            data = await Output.from_input(val)._data
            return OutputData(data.resources, data.value, True)

        return Output(get_data())

    # According to mypy these overloads unsafely overlap, so we ignore the type check.
    # https://mypy.readthedocs.io/en/stable/more_types.html#type-checking-the-variants:~:text=considered%20unsafely%20overlapping
    @overload
    @staticmethod
    def all(*args: Input[T]) -> "Output[List[T]]":  # type: ignore
        ...

    @overload
    @staticmethod
    def all(**kwargs: Input[T]) -> "Output[Dict[str, T]]":
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

        async def gather_dict(outputs: dict[str, Output[T]]) -> OutputData:
            data_list: list[OutputData[T]] = await asyncio.gather(
                *[o._data for o in outputs.values()]
            )

            resources: Set["Resource"] = reduce(
                lambda acc, d: acc.union(d.resources), data_list, set()
            )
            secret = any(data.secret for data in data_list)
            known = all(not isinstance(data.value, Unknown) for data in data_list)
            value = (
                {k: v.value for (k, v) in zip(outputs.keys(), data_list)}
                if known
                else UNKNOWN
            )

            return OutputData(resources, value, secret)

        async def gather_list(outputs: list[Output[T]]) -> OutputData:
            data_list: list[OutputData[T]] = await asyncio.gather(
                *[o._data for o in outputs]
            )

            resources: Set["Resource"] = reduce(
                lambda acc, d: acc.union(d.resources), data_list, set()
            )
            secret = any(data.secret for data in data_list)
            known = all(not isinstance(data.value, Unknown) for data in data_list)
            value = [data.value for data in data_list] if known else UNKNOWN

            return OutputData(resources, value, secret)

        if args and kwargs:
            raise ValueError(
                "Output.all() was supplied a mix of named and unnamed inputs"
            )

        # First, map all inputs to outputs using `from_input`, then aggregate the list or dict of futures into
        # a future of list or dict.
        if kwargs:
            return Output(
                gather_dict({k: Output.from_input(v) for k, v in kwargs.items()})
            )
        return Output(gather_list([Output.from_input(x) for x in args]))

    @staticmethod
    def concat(*args: Input[str]) -> "Output[str]":
        """
        Concatenates a collection of Input[str] into a single Output[str].

        This function takes a sequence of Input[str], stringifies each, and concatenates all values
        into one final string. This can be used like so:

            url = Output.concat("http://", server.hostname, ":", loadBalancer.port)

        :param Input[str] args: A list of string Inputs to concatenate.
        :return: A concatenated output string.
        :rtype: Output[str]
        """

        transformed_items: List[Output[str]] = [Output.from_input(v) for v in args]
        # invariant http://mypy.readthedocs.io/en/latest/common_issues.html#variance
        return Output.all(*transformed_items).apply("".join)  # type: ignore

    @staticmethod
    def format(
        format_string: Input[str], *args: Input[object], **kwargs: Input[object]
    ) -> "Output[str]":
        """
        Perform a string formatting operation.

        This has the same semantics as `str.format` except it handles Input types.

        :param Input[str] format_string: A formatting string
        :param Input[object] args: Positional arguments for the format string
        :param Input[object] kwargs: Keyword arguments for the format string
        :return: A formatted output string.
        :rtype: Output[str]
        """

        if args and kwargs:
            return _map3_output(
                Output.from_input(format_string),
                Output.all(*args),
                Output.all(**kwargs),
                lambda str, args, kwargs: str.format(*args, **kwargs),
            )
        if args:
            return _map2_output(
                Output.from_input(format_string),
                Output.all(*args),
                lambda str, args: str.format(*args),
            )
        if kwargs:
            return _map2_output(
                Output.from_input(format_string),
                Output.all(**kwargs),
                lambda str, kwargs: str.format(**kwargs),
            )
        return Output.from_input(format_string).apply(lambda str: str.format())

    @staticmethod
    def json_dumps(
        obj: Input[Any],
        *,
        skipkeys: bool = False,
        ensure_ascii: bool = True,
        check_circular: bool = True,
        allow_nan: bool = True,
        cls: Optional[Type[json.JSONEncoder]] = None,
        indent: Optional[Union[int, str]] = None,
        separators: Optional[Tuple[str, str]] = None,
        default: Optional[Callable[[Any], Any]] = None,
        sort_keys: bool = False,
        **kw: Any,
    ) -> "Output[str]":
        """
        Uses json.dumps to serialize the given Input[object] value into a JSON string.

        The arguments have the same meaning as in `json.dumps` except obj is an Input.
        """

        if cls is None:
            cls = json.JSONEncoder

        async def run(output: "Output[Any]") -> OutputData[str]:
            seen_unknown = False
            seen_secret = False
            seen_resources = set["Resource"]()

            class OutputEncoder(cls):  # type: ignore
                def default(self, o) -> Optional[Any]:
                    if isinstance(o, Output):
                        nonlocal seen_unknown
                        nonlocal seen_secret
                        nonlocal seen_resources
                        # We need to synchronously wait for o to complete
                        data = _sync_await(o._data)
                        # Update the secret flag and set of seen resources
                        seen_secret = seen_secret or data.secret
                        seen_resources.update(resources)
                        if isinstance(data.value, Unknown):
                            # The value wasn't known set the local seenUnknown variable and just return None
                            # so the serialization doesn't raise an exception at this point
                            seen_unknown = True
                            return None
                        return data.value

                    return super().default(o)

            # Await the output's details.
            data = await output._data

            if isinstance(data.value, Unknown):
                return OutputData(data.resources, UNKNOWN, data.secret)

            # Try and dump using our special OutputEncoder to handle nested outputs
            result = json.dumps(
                data.value,
                skipkeys=skipkeys,
                ensure_ascii=ensure_ascii,
                check_circular=check_circular,
                allow_nan=allow_nan,
                cls=OutputEncoder,
                indent=indent,
                separators=separators,
                default=default,
                sort_keys=sort_keys,
                **kw,
            )

            # Update the final resources and secret flag based on what we saw while dumping
            is_secret = data.secret or seen_secret
            resources = set(data.resources)
            resources.update(seen_resources)

            # If we saw an unknown during dumping then throw away the result and return not known
            if seen_unknown:
                return OutputData(resources, UNKNOWN, is_secret)

            return OutputData(resources, result, is_secret)

        return Output(run(Output.from_input(obj)))

    @staticmethod
    def json_loads(
        s: Input[Union[str, bytes, bytearray]],
        *,
        cls: Optional[Type[json.JSONDecoder]] = None,
        object_hook: Optional[Callable[[Dict[Any, Any]], Any]] = None,
        parse_float: Optional[Callable[[str], Any]] = None,
        parse_int: Optional[Callable[[str], Any]] = None,
        parse_constant: Optional[Callable[[str], Any]] = None,
        object_pairs_hook: Optional[Callable[[List[Tuple[Any, Any]]], Any]] = None,
        **kwds: Any,
    ) -> "Output[Any]":
        """
        Uses json.loads to deserialize the given JSON Input[str] value into a value.

        The arguments have the same meaning as in `json.loads` except s is an Input.
        """

        def loads(s: Union[str, bytes, bytearray]) -> Any:
            return json.loads(
                s,
                cls=cls,
                object_hook=object_hook,
                parse_float=parse_float,
                parse_int=parse_int,
                parse_constant=parse_constant,
                object_pairs_hook=object_pairs_hook,
                **kwds,
            )

        # You'd think this could all be on one line but mypy seems to think `s` is a `Sequence[object]` if you
        # do.
        os: Output[Union[str, bytes, bytearray]] = Output.from_input(s)
        return os.apply(loads)

    def __str__(self) -> str:
        return """Calling __str__ on an Output[T] is not supported.

To get the value of an Output[T] as an Output[str] consider:
1. o.apply(lambda v: f"prefix{v}suffix")

See https://pulumi.io/help/outputs for more details.
This function may throw in a future version of Pulumi."""


def contains_unknowns(val: Any) -> bool:
    return rpc.contains_unknowns(val)


def _is_prompt(value: Input[T]) -> bool:
    """Checks if the value is prompty available."""

    return not isawaitable(value) and not isinstance(value, Output)


def _map_output(o: Output[T], transform: Callable[[T], U]) -> Output[U]:
    """Transforms an output's result value with a pure function."""

    async def fut() -> OutputData[U]:
        data = await o._data
        if isinstance(data.value, Unknown):
            return OutputData(data.resources, UNKNOWN, data.secret)
        return OutputData(data.resources, transform(data.value), data.secret)

    return Output(fut())


def _map2_output(
    o1: Output[T1], o2: Output[T2], transform: Callable[[T1, T2], U]
) -> Output[U]:
    """
    Joins two outputs and transforms their result with a pure function.
    Similar to `all` but does not deeply await.
    """

    async def fut() -> OutputData[U]:
        data1 = await o1._data
        data2 = await o2._data

        resources = data1.resources | data2.resources
        secret = data1.secret or data2.secret

        if isinstance(data1.value, Unknown) or isinstance(data2.value, Unknown):
            return OutputData(resources, UNKNOWN, secret)
        result = transform(data1.value, data2.value)
        return OutputData(resources, result, secret)

    return Output(fut())


def _map3_output(
    o1: Output[T1], o2: Output[T2], o3: Output[T3], transform: Callable[[T1, T2, T3], U]
) -> Output[U]:
    """
    Joins three outputs and transforms their result with a pure function.
    Similar to `all` but does not deeply await.
    """

    async def fut() -> OutputData[U]:
        data1 = await o1._data
        data2 = await o2._data
        data3 = await o3._data

        resources = data1.resources | data2.resources | data3.resources
        secret = data1.secret or data2.secret or data3.secret

        if (
            isinstance(data1.value, Unknown)
            or isinstance(data2.value, Unknown)
            or isinstance(data3.value, Unknown)
        ):
            return OutputData(resources, UNKNOWN, secret)
        result = transform(data1.value, data2.value, data3.value)
        return OutputData(resources, result, secret)

    return Output(fut())


def _map_input(i: Input[T], transform: Callable[[T], U]) -> Input[U]:
    """Transforms an input's result value with a pure function."""

    if _is_prompt(i):
        return transform(cast(T, i))

    if isawaitable(i):
        inp = cast(Awaitable[T], i)

        async def fut() -> U:
            return transform(await inp)

        return asyncio.ensure_future(fut())

    return _map_output(cast(Output[T], i), transform)


def _map2_input(
    i1: Input[T1], i2: Input[T2], transform: Callable[[T1, T2], U]
) -> Input[U]:
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

    return _map2_output(
        Output._from_input_shallow(i1), Output._from_input_shallow(i2), transform
    )
