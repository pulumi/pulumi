# Copyright 2025, Pulumi Corporation.
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
from typing import Any, Optional, Union, overload
from collections.abc import Awaitable, Callable

from . import _types, log
from .output import Input, Output
from .resource import CustomResource, ResourceOptions
from .runtime.settings import _get_callbacks
from .runtime.sync_await import _sync_await

# A reducer combines the previously stashed input and output with the current program input to
# produce a new output value. It is invoked by the engine on update; on create the initial output
# is just the current input. The callback may return an awaitable.
StashReducer = Callable[[Any, Any, Any], Union[Any, Awaitable[Any]]]


@_types.input_type
class StashArgs:
    def __init__(
        self,
        *,
        input: Input[Any],
        reduce: Optional[StashReducer] = None,
    ):
        """
        The set of arguments for constructing a State resource.
        """
        _types.set(self, "input", input)
        if reduce is not None:
            _types.set(self, "reduce", reduce)

    @property
    @_types.getter
    def input(self) -> Input[Any]:
        return _types.get(self, "input")

    @input.setter
    def input(self, input: Input[Any]):
        _types.set(self, "input", input)

    @property
    @_types.getter
    def reduce(self) -> Optional[StashReducer]:
        return _types.get(self, "reduce")

    @reduce.setter
    def reduce(self, reduce: Optional[StashReducer]):
        _types.set(self, "reduce", reduce)


def _get_resource_args_opts(resource_args_type, resource_options_type, *args, **kwargs):
    """
    Return the resource args and options given the *args and **kwargs of a resource's
    __init__ method.
    """

    resource_args, opts = None, None

    # If the first item is the resource args type, save it and remove it from the args list.
    if args and isinstance(args[0], resource_args_type):
        resource_args, args = args[0], args[1:]

    # Now look at the first item in the args list again.
    # If the first item is the resource options class, save it.
    if args and isinstance(args[0], resource_options_type):
        opts = args[0]

    # If resource_args is None, see if "args" is in kwargs, and, if so, if it's typed as the
    # the resource args type.
    if resource_args is None:
        a = kwargs.get("args")
        if isinstance(a, resource_args_type):
            resource_args = a

    # If opts is None, look it up in kwargs.
    if opts is None:
        opts = kwargs.get("opts")

    return resource_args, opts


class Stash(CustomResource):
    """
    Stash stores an arbitrary value in the state.
    """

    output: Output[Any]
    """
    The value saved in the state for the stash.
    """

    input: Output[Any]
    """
    The most recent value passed to the stash resource.
    """

    @overload
    def __init__(
        self,
        resource_name: str,
        opts: Optional[ResourceOptions] = None,
        input: Optional[Input[Any]] = None,
        reduce: Optional[StashReducer] = None,
        __props__=None,
    ):
        """
        Create a Resource resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param Input[Any] input: The value to store in the stash resource.
        :param StashReducer reduce: An optional reducer combining the previously stashed
               input and output with the current program input to produce a new output. On
               create the reducer is skipped and the initial output is just the input.
        :param ResourceOptions opts: Options for the resource.
        """
        ...

    @overload
    def __init__(
        self,
        resource_name: str,
        args: StashArgs,
        opts: Optional[ResourceOptions] = None,
    ):
        """
        Create a Resource resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param StashArgs args: The arguments to use to populate this resource's properties.
        :param ResourceOptions opts: Options for the resource.
        """
        ...

    def __init__(self, resource_name: str, *args, **kwargs):
        resource_args, opts = _get_resource_args_opts(
            StashArgs, ResourceOptions, *args, **kwargs
        )
        if resource_args is not None:
            self._internal_init(
                resource_name,
                opts,
                input=resource_args.input,
                reduce=resource_args.reduce,
            )
        else:
            self._internal_init(resource_name, *args, **kwargs)

    def _internal_init(
        self,
        resource_name: str,
        opts: Optional[ResourceOptions] = None,
        input: Optional[Input[Any]] = None,
        reduce: Optional[StashReducer] = None,
    ):
        opts = opts or ResourceOptions()
        if not isinstance(opts, ResourceOptions):
            raise TypeError(
                "Expected resource options to be a ResourceOptions instance"
            )

        props: dict = {}
        if input is not None:
            props["input"] = input

        props["output"] = None

        # If a reducer callback was supplied, register it with the callback server and pass a
        # `{ target, token }` `reducer` input through to the engine. The builtin provider will
        # invoke this callback during Check on update; the reducer object itself is stripped
        # from state.
        if reduce is not None:
            callbacks = _sync_await(_get_callbacks())
            if callbacks is None:
                log.warn(
                    "Stash reducer requires an active Pulumi resource monitor; ignoring reducer",
                )
            else:
                cb = callbacks.register_stash_reducer(reduce)
                props["reducer"] = {"target": cb.target, "token": cb.token}

        super().__init__(
            "pulumi:index:Stash",
            resource_name,
            props,
            opts,
        )
