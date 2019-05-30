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
from typing import Optional, Any
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
        opts = deepcopy(opts) if opts is not None else ResourceOptions()
        opts.id = target_stack

        super().__init__("pulumi:pulumi:StackReference", name, {
            "name": target_stack,
            "outputs": None,
        }, opts)

    def get_output(self, name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named stack output.

        :param Input[str] name: The name of the stack output to fetch.
        """
        return Output.all(Output.from_input(name), self.outputs).apply(lambda l: l[1][l[0]])
