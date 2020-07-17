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
from asyncio import ensure_future
from typing import Optional, Any, List, Callable
from copy import deepcopy

from .output import Output, Input
from .resource import CustomResource, ResourceOptions


class StackReference(CustomResource):
    """
    Manages a reference to a Pulumi stack. The referenced stack's outputs are available via its "outputs" property or
    the "output" method.
    """

    name: Output[str]
    """
    The name of the referenced stack.
    """

    outputs: Output[dict]
    """
    The outputs of the referenced stack.
    """

    secret_output_names: Output[List[str]]
    """
    The names of any stack outputs which contain secrets.
    """

    def __init__(self,
                 name: str,
                 stack_name: Optional[str] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        """
        :param str name: The unique name of the stack reference.
        :param Optional[str] stack_name: The name of the stack to reference. If not provided, defaults to the name of
               this resource.
        :param Optional[ResourceOptions] opts: An optional set of resource options for this resource.
        """

        target_stack = stack_name if stack_name is not None else name
        opts = ResourceOptions.merge(opts, ResourceOptions(id=target_stack))

        super().__init__("pulumi:pulumi:StackReference", name, {
            "name": target_stack,
            "outputs": None,
            "secret_output_names": None,
        }, opts)

    def get_output(self, name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named stack output, or None if the stack output was not found.

        :param Input[str] name: The name of the stack output to fetch.
        """
        value: Output[Any] = Output.all(Output.from_input(name), self.outputs).apply(lambda l: l[1].get(l[0])) # type: ignore
        is_secret = ensure_future(self.__is_secret_name(name))

        return Output(value.resources(), value.future(), value.is_known(), is_secret)

    def require_output(self, name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named stack output, or raises a KeyError if the output was not
        found.

        :param Input[str] name: The name of the stack output to fetch.
        """

        value = Output.all(Output.from_input(name), self.outputs).apply(lambda l: l[1][l[0]]) # type: ignore
        is_secret = ensure_future(self.__is_secret_name(name))

        return Output(value.resources(), value.future(), value.is_known(), is_secret)

    def translate_output_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of output properties
        into a format of their choosing before writing those properties to the resource object.

        :param str prop: A property name.
        :return: A potentially transformed property name.
        :rtype: str
        """

        return "secret_output_names" if prop == "secretOutputNames" else prop

    async def __is_secret_name(self, name: Input[str]) -> bool:
        # If either the name or set of secret outputs is unknown, we can't do anything smart, so we
        # just copy the secretness from the entire outputs value.
        if not (await Output.from_input(name).is_known() and await self.secret_output_names.is_known()):
            return await self.outputs.is_secret()

        # Otherwise, if we have a list of outputs we know are secret, we can use that list to
        # determine if this output should be secret. Names could be None here in cases where we are
        # using an older CLI that did not return this information (in this case we again fallback to
        # the secretness of outputs value).
        names = await (self.secret_output_names.future())
        if names is None:
            return await self.outputs.is_secret()

        return await Output.from_input(name).future() in names
