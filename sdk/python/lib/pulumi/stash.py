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
    def __init__(
        __self__, *, value: Input[Any], passthrough: Optional[Input[bool]] = None
    ):
        """
        The set of arguments for constructing a State resource.
        """
        _types.set(__self__, "value", value)
        _types.set(__self__, "passthrough", passthrough)

    @property
    @_types.getter
    def value(self) -> Input[Any]:
        return _types.get(self, "value")

    @value.setter
    def value(self, value: Input[Any]):
        _types.set(self, "value", value)

    @property
    @_types.getter
    def passthrough(self) -> Optional[Input[bool]]:
        return _types.get(self, "passthrough")

    @passthrough.setter
    def passthrough(self, passthrough: Optional[Input[bool]]):
        _types.set(self, "passthrough", passthrough)


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
    Manages a reference to a Pulumi stash value.
    """

    value: Output[Any]
    """
    The value of the stash resource.
    """

    @overload
    def __init__(
        __self__,
        resource_name: str,
        opts: Optional[ResourceOptions] = None,
        value: Optional[Input[Any]] = None,
        passthrough: Optional[Input[bool]] = None,
        __props__=None,
    ):
        """
        Create a Resource resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param ResourceOptions opts: Options for the resource.
        """
        ...

    @overload
    def __init__(
        __self__,
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

    def __init__(__self__, resource_name: str, *args, **kwargs):
        resource_args, opts = _get_resource_args_opts(
            StashArgs, ResourceOptions, *args, **kwargs
        )
        if resource_args is not None:
            __self__._internal_init(resource_name, opts, **resource_args.__dict__)
        else:
            __self__._internal_init(resource_name, *args, **kwargs)

    def _internal_init(
        __self__,
        resource_name: str,
        opts: Optional[ResourceOptions] = None,
        value: Optional[Input[Any]] = None,
        passthrough: Optional[Input[bool]] = None,
    ):
        opts = opts or ResourceOptions()
        if not isinstance(opts, ResourceOptions):
            raise TypeError(
                "Expected resource options to be a ResourceOptions instance"
            )

        props = {}
        if value is not None:
            props["value"] = value
        if passthrough is not None:
            props["passthrough"] = passthrough

        super().__init__(
            "pulumi:pulumi:Stash",
            resource_name,
            props,
            opts,
        )
