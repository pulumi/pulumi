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
import asyncio
import dataclasses
import json
import os
from functools import reduce
from inspect import isawaitable
from typing import (
    TYPE_CHECKING,
    Any,
    Generic,
    Optional,
    TypeVar,
    Union,
    cast,
    overload,
)
from collections.abc import Callable
from collections.abc import Awaitable, Mapping

from . import log
from . import _types
from .runtime import rpc
from .runtime.sync_await import _sync_await
from .runtime.settings import SETTINGS
from .runtime._serialization import (
    _serialization_enabled,
    _secrets_allowed,
    _set_contained_secrets,
)

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


@dataclasses.dataclass
class _OutputData(Generic[T_co]):
    """
    Holds the fully-resolved state of an Output in one place.  All four fields are
    computed together and stored as a single awaitable inside Output._data.
    """

    resources: set["Resource"]
    value: T_co
    is_known: bool
    is_secret: bool

    def __post_init__(self) -> None:
        # An output whose value contains unknowns is never "known", regardless of what the
        # provider reported.  Normalising here means every code path that constructs an
        # _OutputData gets consistent semantics automatically.
        if self.is_known and contains_unknowns(self.value):
            self.is_known = False


class Output(Generic[T_co]):
    """
    Output helps encode the relationship between Resources in a Pulumi application. Specifically an
    Output holds onto a piece of Data and the Resource it was generated from. An Output value can
    then be provided when constructing new Resources, allowing that new Resource to know both the
    value as well as the Resource the value came from.  This allows for a precise 'Resource
    dependency graph' to be created, which properly tracks the relationship between resources.
    """

    _data: asyncio.Future[_OutputData[T_co]]
    """
    Single future that resolves to all Output state at once: the value, whether it is known,
    whether it is secret, and the set of resource dependencies.
    """

    # Despite not being part of the public API we make use of the following members in code generated SDKs. So they have
    # to keep working for backwards compatibility.

    @property
    async def _is_known(self) -> bool:
        """
        Whether or not this 'Output' should actually perform .apply calls.  During a preview,
        an Output value may not be known (because it would have to actually be computed by doing an
        'update').  In that case, we don't want to perform any .apply calls as the callbacks
        may not expect an undefined value.  So, instead, we just transition to another Output
        value that itself knows it should not perform .apply calls.
        """
        return (await self._data).is_known

    @property
    async def _is_secret(self) -> bool:
        """
        Whether or not this 'Output' should be treated as containing secret data. Secret outputs are tagged when
        flowing across the RPC interface to the resource monitor, such that when they are persisted to disk in
        our state file, they are encrypted instead of being in plaintext.
        """
        return (await self._data).is_secret

    @property
    async def _future(self) -> T_co:
        """
        Future that actually produces the concrete value of this output.
        """
        return (await self._data).value

    @property
    async def _resources(self) -> set["Resource"]:
        """
        The list of resources that this output value depends on.
        """
        return (await self._data).resources

    def __init__(
        self,
        resources: Union[Awaitable[set["Resource"]], set["Resource"]],
        future: Awaitable[T_co],
        is_known: Awaitable[bool],
        is_secret: Optional[Awaitable[bool]] = None,
    ) -> None:
        async def compute_data() -> "_OutputData[T_co]":
            res = resources if isinstance(resources, set) else await resources
            val = await future
            known = await is_known
            secret = (await is_secret) if is_secret is not None else False
            # _OutputData.__post_init__ will normalise is_known if the value contains unknowns.
            return _OutputData(
                resources=res, value=val, is_known=known, is_secret=secret
            )

        self._data = asyncio.ensure_future(compute_data())
        self._track()

    def _track(self) -> None:
        """Register _data with SETTINGS.outputs for lifecycle tracking."""
        with SETTINGS.lock:
            SETTINGS.outputs.append(self._data)

        def cleanup(fut: "asyncio.Future[_OutputData[T_co]]") -> None:
            if fut.cancelled() or (fut.exception() is not None):
                # if cancelled or error'd leave it in the deque to pick up at program exit
                return
            # else remove it from the deque
            with SETTINGS.lock:
                try:
                    SETTINGS.outputs.remove(fut)
                except ValueError:
                    # if it's not in the deque then it's already been removed in wait_for_rpcs
                    pass

        self._data.add_done_callback(cleanup)

    @classmethod
    def _from_data(cls, data: "Awaitable[_OutputData[T_co]]") -> "Output[T_co]":
        """Internal factory: create an Output directly from a single _OutputData awaitable."""
        out: Output[T_co] = object.__new__(cls)
        out._data = asyncio.ensure_future(data)
        out._track()
        return out

    # Private implementation details - do not document.
    async def resources(self) -> set["Resource"]:
        return (await self._data).resources

    async def future(self, with_unknowns: Optional[bool] = None) -> Optional[T_co]:
        # If the caller did not explicitly ask to see unknown values and the value of this output contains unknowns,
        # return None. This preserves compatibility with earlier versions of the Pulumi SDK.
        data = await self._data
        return (
            None if not with_unknowns and contains_unknowns(data.value) else data.value
        )

    async def is_known(self) -> bool:
        return (await self._data).is_known

    # End private implementation details.

    def __getstate__(self):
        """
        Serialize this Output into a dictionary for pickling, only when serialization is enabled.
        """

        if not _serialization_enabled():
            raise Exception("__getstate__ can only be called during serialization")

        value, is_secret = _sync_await(asyncio.gather(self.future(), self.is_secret()))

        if is_secret:
            if _secrets_allowed():
                _set_contained_secrets(True)
            else:
                raise Exception("Secret outputs cannot be captured")

        return {"value": value}

    def __setstate__(self, state):
        """
        Deserialize this Output from a dictionary, only when serialization is enabled.
        """

        if not _serialization_enabled():
            raise Exception("__setstate__ can only be called during deserialization")

        value = state["value"]

        # Replace '.get' with a function that returns the value without raising an error.
        self.get = lambda: value

        def error(name: str):
            def f(*args: Any, **kwargs: Any):
                raise Exception(
                    f"'{name}' is not allowed from inside a cloud-callback. "
                    + "Use 'get' to retrieve the value of this Output directly."
                )

            return f

        # Replace '.apply' and other methods on Output with implementations that raise an error.
        self.apply = error("apply")
        self.resources = error("resources")
        self.future = error("future")
        self.is_known = error("is_known")
        self.is_secret = error("is_secret")

    def get(self) -> T_co:
        """
        Retrieves the underlying value of this Output.

        This function is only callable in code that runs post-deployment. At this point all Output
        values will be known and can be safely retrieved. During pulumi deployment or preview
        execution this must not be called (and will raise an error). This is because doing so would
        allow Output values to flow into Resources while losing the data that would allow the
        dependency graph to be changed.
        """
        raise Exception(
            "Cannot call '.get' during update or preview. To manipulate the value of this Output, "
            + "use '.apply' instead."
        )

    async def is_secret(self) -> bool:
        return (await self._data).is_secret

    def apply(
        self, func: Callable[[T_co], Input[U]], run_with_unknowns: bool = False
    ) -> "Output[U]":
        """
        Transforms the data of the output with the provided func.  The result remains an
        Output so that dependent resources can be properly tracked.

        'func' should not be used to create resources unless necessary as 'func' may not be run during some program executions.

        'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
        and you want to get a transitive dependency of it.

        This function will be called during execution of a `pulumi up` or `pulumi preview` request.
        It may not run when the values of the resource is unknown.

        :param Callable[[T_co],Input[U]] func: A function that will, given this Output's value, transform the value to
               an Input of some kind, where an Input is either a prompt value, a Future, or another Output of the given
               type.
        :return: A transformed Output obtained from running the transformation function on this Output's value.
        :rtype: Output[U]
        """

        # The "run" coroutine actually runs the apply.
        async def run() -> "_OutputData[U]":
            # Await this output's details.
            data = await self._data
            resources = data.resources
            is_known = data.is_known
            is_secret = data.is_secret
            value = data.value

            # Only perform the apply if the engine was able to give us an actual value for this
            # Output or if the caller is able to tolerate unknown values.
            if not (is_known or run_with_unknowns):
                # We didn't actually run the function, our new Output is definitely **not** known.
                return _OutputData(
                    resources=resources,
                    value=cast(U, None),
                    is_known=False,
                    is_secret=is_secret,
                )

            # If we are running with unknown values and the value is explicitly unknown but does not
            # actually contain any unknown values, collapse its value to the unknown value. This
            # ensures that callbacks that expect to see unknowns during preview in outputs that are
            # not known will always do so.
            if not is_known and run_with_unknowns and not contains_unknowns(value):
                value = cast(T_co, UNKNOWN)

            transformed: Input[U] = func(value)
            # Transformed is an Input, meaning there are three cases:
            #  1. transformed is an Output[U]
            if isinstance(transformed, Output):
                transformed_as_output = cast(Output[U], transformed)
                # Forward along the inner output's resources, is_known and is_secret values.
                t_data = await transformed_as_output._data
                return _OutputData(
                    resources=resources | t_data.resources,
                    value=cast(U, t_data.value),
                    is_known=t_data.is_known,
                    is_secret=t_data.is_secret or is_secret,
                )

            #  2. transformed is an Awaitable[U]
            if isawaitable(transformed):
                # Since transformed is not an Output, it is known.
                result = await cast(Awaitable[U], transformed)
                return _OutputData(
                    resources=resources,
                    value=result,
                    is_known=True,
                    is_secret=is_secret,
                )

            #  3. transformed is U. It is trivially known.
            return _OutputData(
                resources=resources,
                value=cast(U, transformed),
                is_known=True,
                is_secret=is_secret,
            )

        return Output._from_data(run())

    def __getattr__(self, item: str) -> "Output[Any]":  # type: ignore
        """
        Syntactic sugar for retrieving attributes off of outputs.

        Note that strictly speaking, this implementation of __getattr__ violates
        the contract expected by Python. __getattr__ is expected to raise
        (synchronously) an AttributeError if the attribute is not found.
        However, we return an Output value, which is asynchronous and represents
        a future value. If we try to lift an attribute that does not exist
        therefore, we'll violate the contract by returning an Output that will
        later blow up with an AttributeError. This means that builtins such as
        hasattr generally won't work correctly on Outputs.

        This is generally fine for most Pulumi use cases, but it can cause
        problems when interacting with other libraries that expect attribute
        access to behave correctly. To try and strike a balance that works in a
        majority of cases, we raise an AttributeError immediately if the
        attribute is one of a set that we expect not to need to lift in order to
        make provider SDKs ergonomic (e.g., things that "look reserved" such as
        class-private identifiers and dunder methods).

        :param str item: An attribute name.
        :return: An Output of this Output's underlying value's property with the given name.
        :rtype: Output[Any]
        """
        if item.startswith("__"):
            raise AttributeError(f"'Output' object has no attribute '{item}'")

        def lift(v: Any) -> Any:
            return UNKNOWN if isinstance(v, Unknown) else getattr(v, item)

        return self.apply(lift, True)

    def __getitem__(self, key: Any) -> "Output[Any]":
        """
        Syntactic sugar for looking up attributes dynamically off of outputs.

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
        raise TypeError(
            "'Output' object is not iterable, consider iterating the underlying value inside an 'apply'"
        )

    @overload
    @staticmethod
    def from_input(val: "Output[U]") -> "Output[U]": ...

    @overload
    @staticmethod
    def from_input(val: Input[U]) -> "Output[U]": ...

    @staticmethod
    def from_input(val: Input[U]) -> "Output[U]":
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values through nested
        lists, dicts, and input classes.  Nested objects of other types (including Resources) are not deeply unwrapped.

        :param Input[U] val: An Input to be converted to an Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values.
        :rtype: Output[U]
        """

        # Is it an output already? Recurse into the value contained within it.
        if isinstance(val, Output):
            return val.apply(Output.from_input, True)

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
                lambda d: typ(**d) if d else typ()
            )
            return cast(Output[U], o_typ)

        # Is a (non-empty) dict, list, or tuple? Recurse into the values within them.
        if val and isinstance(val, dict):
            # The keys themselves might be outputs, so we can't just pass `**val` to all.

            # keys() and values() will be in the same order: https://docs.python.org/3/library/stdtypes.html#dictionary-view-objects
            keys = list(val.keys())
            values = list(val.values())

            def liftValues(keys: list[Any]):
                d = {keys[i]: values[i] for i in range(len(keys))}
                return Output.all(**d)

            o_dict: Output[dict] = Output.all(*keys).apply(liftValues)
            return cast(Output[U], o_dict)

        if val and isinstance(val, list):
            o_list: Output[list] = Output.all(*val)
            return cast(Output[U], o_list)

        if val and isinstance(val, tuple):
            # We can splat a tuple into all, but we'll always get back a list...
            o_list = Output.all(*val)
            # ...so we need to convert back to a tuple.
            return cast(Output[U], o_list.apply(tuple))

        # If it's not an output, tuple, list, or dict, it must be known and not secret

        # Is it awaitable? If so, schedule it for execution and use the resulting future
        # as the value future for a new output.
        if isawaitable(val):

            async def from_awaitable(v: Awaitable[Any]) -> _OutputData[Any]:
                resolved = await v
                return _OutputData(
                    resources=set(), value=resolved, is_known=True, is_secret=False
                )

            return Output._from_data(from_awaitable(cast(Awaitable[Any], val))).apply(
                Output.from_input, True
            )

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        data_fut: asyncio.Future[_OutputData[Any]] = asyncio.Future()
        data_fut.set_result(
            _OutputData(resources=set(), value=val, is_known=True, is_secret=False)
        )
        return Output._from_data(data_fut)

    @staticmethod
    def _from_input_shallow(val: Input[U]) -> "Output[U]":
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

            async def from_awaitable() -> _OutputData[Any]:
                resolved = await val
                return _OutputData(
                    resources=set(), value=resolved, is_known=True, is_secret=False
                )

            return Output._from_data(from_awaitable())

        # Is it a prompt value? Set up a new resolved future and use that as the value future.
        data_fut: asyncio.Future[_OutputData[Any]] = asyncio.Future()
        data_fut.set_result(
            _OutputData(resources=set(), value=val, is_known=True, is_secret=False)
        )
        return Output._from_data(data_fut)

    @staticmethod
    def unsecret(val: "Output[U]") -> "Output[U]":
        """
        Takes an existing Output, deeply unwraps the nested values and returns a new Output without any secrets included

        :param Output[T] val: An Output to be converted to a non-Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any secret values.
        :rtype: Output[T]
        """

        async def run() -> "_OutputData[U]":
            data = await val._data
            return _OutputData(
                resources=data.resources,
                value=data.value,
                is_known=data.is_known,
                is_secret=False,
            )

        return Output._from_data(run())

    @staticmethod
    def secret(val: Input[U]) -> "Output[U]":
        """
        Takes an Input value and produces an Output value from it, deeply unwrapping nested Input values as necessary
        given the type. It also marks the returned Output as a secret, so its contents will be persisted in an encrypted
        form in state files.

        :param Input[T] val: An Input to be converted to an Secret Output.
        :return: A deeply-unwrapped Output that is guaranteed to not contain any Input values and is marked as a Secret.
        :rtype: Output[T]
        """

        o = Output.from_input(val)

        async def run() -> "_OutputData[U]":
            data = await o._data
            return _OutputData(
                resources=data.resources,
                value=data.value,
                is_known=data.is_known,
                is_secret=True,
            )

        return Output._from_data(run())

    # According to mypy these overloads unsafely overlap, so we ignore the type check.
    # https://mypy.readthedocs.io/en/stable/more_types.html#type-checking-the-variants:~:text=considered%20unsafely%20overlapping
    @overload
    @staticmethod
    def all(*args: "Output[Any]") -> "Output[list[Any]]": ...  # type: ignore

    @overload
    @staticmethod
    def all(**kwargs: "Output[Any]") -> "Output[dict[str, Any]]": ...  # type: ignore

    @overload
    @staticmethod
    def all(*args: Input[Any]) -> "Output[list[Any]]": ...  # type: ignore

    @overload
    @staticmethod
    def all(**kwargs: Input[Any]) -> "Output[dict[str, Any]]": ...  # type: ignore

    @staticmethod
    def all(
        *args: Input[Any], **kwargs: Input[Any]
    ) -> "Output[list[Any] | dict[str, Any]]":
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

        if args and kwargs:
            raise ValueError(
                "Output.all() was supplied a mix of named and unnamed inputs"
            )
        # First, map all inputs to outputs using `from_input`.
        all_outputs: Union[list, dict] = (
            {k: Output.from_input(v) for k, v in kwargs.items()}
            if kwargs
            else [Output.from_input(x) for x in args]
        )

        async def gather_data() -> "_OutputData[list[Any] | dict[str, Any]]":
            output_list = (
                list(all_outputs.values())
                if isinstance(all_outputs, dict)
                else all_outputs
            )
            all_data = await asyncio.gather(*[o._data for o in output_list])
            resources: set[Resource] = reduce(
                lambda acc, d: acc | d.resources, all_data, set()
            )
            known = all(d.is_known for d in all_data)
            secret = any(d.is_secret for d in all_data)
            if isinstance(all_outputs, dict):
                values: list[Any] | dict[str, Any] = {
                    k: d.value for k, d in zip(all_outputs.keys(), all_data)
                }
            else:
                values = [d.value for d in all_data]
            return _OutputData(
                resources=resources, value=values, is_known=known, is_secret=secret
            )

        return Output._from_data(gather_data())

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

        transformed_items: list[Input[Any]] = [Output.from_input(v) for v in args]
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
        cls: Optional[type[json.JSONEncoder]] = None,
        indent: Optional[Union[int, str]] = None,
        separators: Optional[tuple[str, str]] = None,
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

        output = Output.from_input(obj)

        async def run() -> "_OutputData[str]":
            seen_unknown = False
            seen_secret = False
            seen_resources: set = set()

            class OutputEncoder(cls):  # type: ignore
                def default(self, o):
                    if isinstance(o, Output):
                        nonlocal seen_unknown
                        nonlocal seen_secret
                        nonlocal seen_resources

                        # We need to synchronously wait for o to complete
                        inner_data = _sync_await(o._data)
                        seen_secret = seen_secret or inner_data.is_secret
                        seen_resources.update(inner_data.resources)
                        if inner_data.is_known:
                            return inner_data.value
                        # The value wasn't known; set seenUnknown and return None so the
                        # serialization doesn't raise an exception at this point.
                        seen_unknown = True
                        return None

                    return super().default(o)

            # Await this output's details.
            data = await output._data

            if not data.is_known:
                return _OutputData(
                    resources=data.resources,
                    value=cast(str, None),
                    is_known=False,
                    is_secret=data.is_secret,
                )

            # Try and dump using our special OutputEncoder to handle nested outputs.
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

            # Update the final resources and secret flag based on what we saw while dumping.
            final_is_secret = data.is_secret or seen_secret
            final_resources = set(data.resources) | seen_resources

            # If we saw an unknown during dumping then throw away the result and return not known.
            if seen_unknown:
                return _OutputData(
                    resources=final_resources,
                    value=cast(str, None),
                    is_known=False,
                    is_secret=final_is_secret,
                )

            return _OutputData(
                resources=final_resources,
                value=result,
                is_known=True,
                is_secret=final_is_secret,
            )

        return Output._from_data(run())

    @staticmethod
    def json_loads(
        s: Input[Union[str, bytes, bytearray]],
        *,
        cls: Optional[type[json.JSONDecoder]] = None,
        object_hook: Optional[Callable[[dict[Any, Any]], Any]] = None,
        parse_float: Optional[Callable[[str], Any]] = None,
        parse_int: Optional[Callable[[str], Any]] = None,
        parse_constant: Optional[Callable[[str], Any]] = None,
        object_pairs_hook: Optional[Callable[[list[tuple[Any, Any]]], Any]] = None,
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
        s_output: Output[Union[str, bytes, bytearray]] = Output.from_input(s)
        return s_output.apply(loads)

    def __str__(self) -> str:
        err = _OutputToStringError()
        if os.getenv("PULUMI_ERROR_OUTPUT_STRING", "").lower() in ["1", "true"]:
            raise err
        msg = str(err)
        log.warn(msg)
        msg += "\nThis function may throw in a future version of Pulumi."
        return msg


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


def _map_output(o: Output[T], transform: Callable[[T], U]) -> Output[U]:
    """Transforms an output's result value with a pure function."""

    async def run() -> _OutputData[U]:
        data = await o._data
        value = None if contains_unknowns(data.value) else data.value
        result = transform(value) if value is not None else cast(U, UNKNOWN)
        return _OutputData(
            resources=data.resources,
            value=result,
            is_known=data.is_known,
            is_secret=data.is_secret,
        )

    return Output._from_data(run())


def _map2_output(
    o1: Output[T1], o2: Output[T2], transform: Callable[[T1, T2], U]
) -> Output[U]:
    """
    Joins two outputs and transforms their result with a pure function.
    Similar to `all` but does not deeply await.
    """

    async def run() -> _OutputData[U]:
        d1, d2 = await asyncio.gather(o1._data, o2._data)
        v1 = None if contains_unknowns(d1.value) else d1.value
        v2 = None if contains_unknowns(d2.value) else d2.value
        result = (
            transform(v1, v2)
            if (v1 is not None) and (v2 is not None)
            else cast(U, UNKNOWN)
        )
        return _OutputData(
            resources=d1.resources | d2.resources,
            value=result,
            is_known=d1.is_known and d2.is_known,
            is_secret=d1.is_secret or d2.is_secret,
        )

    return Output._from_data(run())


def _map3_output(
    o1: Output[T1], o2: Output[T2], o3: Output[T3], transform: Callable[[T1, T2, T3], U]
) -> Output[U]:
    """
    Joins three outputs and transforms their result with a pure function.
    Similar to `all` but does not deeply await.
    """

    async def run() -> _OutputData[U]:
        d1, d2, d3 = await asyncio.gather(o1._data, o2._data, o3._data)
        v1 = None if contains_unknowns(d1.value) else d1.value
        v2 = None if contains_unknowns(d2.value) else d2.value
        v3 = None if contains_unknowns(d3.value) else d3.value
        result = (
            transform(v1, v2, v3)
            if (v1 is not None) and (v2 is not None) and (v3 is not None)
            else cast(U, UNKNOWN)
        )
        return _OutputData(
            resources=d1.resources | d2.resources | d3.resources,
            value=result,
            is_known=d1.is_known and d2.is_known and d3.is_known,
            is_secret=d1.is_secret or d2.is_secret or d3.is_secret,
        )

    return Output._from_data(run())


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


def deferred_output() -> tuple[Output[T], Callable[[Output[T]], None]]:
    """
    Creates an Output[T] whose value can be later resolved from another Output[T] instance.
    """
    data_future: asyncio.Future[_OutputData[T]] = asyncio.Future()
    already_resolved = False

    def resolve(o: Output[T]) -> None:
        nonlocal already_resolved
        if already_resolved:
            raise Exception("Deferred Output has already been resolved")
        already_resolved = True

        def data_callback(fut: "asyncio.Future[_OutputData[T]]") -> None:
            if fut.cancelled():
                data_future.cancel()
            elif (exc := fut.exception()) is not None:
                data_future.set_exception(exc)
            else:
                data_future.set_result(fut.result())

        o._data.add_done_callback(data_callback)

    return Output._from_data(data_future), resolve


class _OutputToStringError(Exception):
    """_OutputToStringError is the class of errors raised when __str__ is called
    on a Pulumi Output."""

    def __init__(self) -> None:
        super().__init__(
            """Calling __str__ on an Output[T] is not supported.

To get the value of an Output[T] as an Output[str] consider:
1. o.apply(lambda v: f"prefix{v}suffix")

See https://www.pulumi.com/docs/concepts/inputs-outputs for more details."""
        )


def _safe_str(v: Any) -> str:
    """_safe_str returns the string representation of v if possible. If v is an
    Output, _safe_str returns a fallback string, whether it's able to detect an
    Output ahead of time or not by catching the _OutputToStringError. _safe_str
    is designed for use in e.g. logging and debugging contexts where it's useful
    to print all the information that can be reasonably obtained, without
    falling afoul of things like PULUMI_ERROR_OUTPUT_STRING."""

    # This is not a perfect implementation. If v's __str__ method tries to
    # stringify an Output, and PULUMI_ERROR_OUTPUT_STRING is not set, we'll
    # still produce an ugly message somwhere inside the resulting string. If
    # this becomes an issue, we could spot it using e.g. string comparison or
    # (far uglier but potentially more performant) monkey patching/subclassing
    # the strings involved. For now this feels like a sensible compromise.

    if isinstance(v, Output):
        return "Output[T]"

    try:
        return str(v)
    except _OutputToStringError:
        return "Output[T]"
