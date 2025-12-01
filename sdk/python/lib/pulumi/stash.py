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
from typing import Optional, Any, overload

from .output import Input, Output
from .resource import CustomResource, ResourceOptions
from . import _types


@_types.input_type
class StashArgs:
    def __init__(self, *, input: Input[Any]):
        """
        The set of arguments for constructing a State resource.
        """
        _types.set(self, "input", input)

    @property
    @_types.getter
    def input(self) -> Input[Any]:
        return _types.get(self, "input")

    @input.setter
    def input(self, input: Input[Any]):
        _types.set(self, "input", input)


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
        __props__=None,
    ):
        """
        Create a Resource resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param Input[Any] input: The value to store in the stash resource.
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
            self._internal_init(resource_name, opts, **resource_args.__dict__)
        else:
            self._internal_init(resource_name, *args, **kwargs)

    def _internal_init(
        self,
        resource_name: str,
        opts: Optional[ResourceOptions] = None,
        input: Optional[Input[Any]] = None,
    ):
        opts = opts or ResourceOptions()
        if not isinstance(opts, ResourceOptions):
            raise TypeError(
                "Expected resource options to be a ResourceOptions instance"
            )

        props = {}
        if input is not None:
            props["input"] = input

        props["output"] = None

        super().__init__(
            "pulumi:index:Stash",
            resource_name,
            props,
            opts,
        )
