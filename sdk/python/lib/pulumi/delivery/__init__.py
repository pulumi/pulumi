# Copyright 2026, Pulumi Corporation.
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

"""
Delivers Pulumi stacks from a Pulumi program: previewing the program previews the stacks, and
updating it updates them, all in one coherence window.
"""

from asyncio import ensure_future
from collections.abc import Mapping, Sequence
from typing import Any, Optional, TypedDict

from ..output import Input, Output
from ..resource import CustomResource, ResourceOptions


class StackSource(TypedDict):
    """
    Where a delivered stack's program is.
    """

    directory: Input[str]
    """
    The directory holding the program, relative to the delivery program's own.
    """


class StackArgs(TypedDict):
    stack: Input[str]
    """
    The fully qualified name of the stack to deliver, as ``<organization>/<project>/<stack>``.
    """

    source: Input[StackSource]


class Stack(CustomResource):
    """
    Delivers a Pulumi stack in the same coherence window as every other stack the program
    delivers. The stack's outputs are available through ``outputs`` or ``get_output``.
    """

    stack: Output[str]
    outputs: Output[dict[str, Any]]
    secret_output_names: Output[list[str]]

    def __init__(
        self,
        name: str,
        stack: Input[str],
        source: Input[StackSource],
        opts: Optional[ResourceOptions] = None,
    ) -> None:
        super().__init__(
            "pulumi:delivery:Stack",
            name,
            {
                "stack": stack,
                "source": source,
                "outputs": None,
                "secret_output_names": None,
            },
            opts,
        )

    def get_output(self, name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named stack output, or None if the stack output was not found.
        """
        return _output_named(
            self.outputs, self.secret_output_names, name, required=False
        )

    def require_output(self, name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named stack output, or raises a KeyError if the output was not
        found.
        """
        return _output_named(
            self.outputs, self.secret_output_names, name, required=True
        )

    def translate_output_property(self, prop: str) -> str:
        return "secret_output_names" if prop == "secretOutputNames" else prop


class StackGroup(CustomResource):
    """
    Delivers a set of Pulumi stacks together without knowing the order they depend on each other
    in. They start at once, and a stack that reads another's outputs waits for that stack's own
    preview or update rather than reading its saved state.
    """

    members: Output[list[dict[str, Any]]]

    def __init__(
        self,
        name: str,
        stacks: Input[Sequence[Input[StackArgs]]],
        opts: Optional[ResourceOptions] = None,
    ) -> None:
        super().__init__(
            "pulumi:delivery:StackGroup",
            name,
            {
                "stacks": stacks,
                "members": None,
            },
            opts,
        )

    def stack_outputs(self, stack: Input[str]) -> Output[dict[str, Any]]:
        """
        Fetches the outputs of one of the group's stacks.
        """
        return Output.all(Output.from_input(stack), self.members).apply(
            lambda l: _member(l[1], l[0])["outputs"]
        )

    def get_output(self, stack: Input[str], name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named output of one of the group's stacks, or None if the stack
        output was not found.
        """
        return _output_named(
            self.stack_outputs(stack),
            self._secret_output_names(stack),
            name,
            required=False,
        )

    def require_output(self, stack: Input[str], name: Input[str]) -> Output[Any]:
        """
        Fetches the value of the named output of one of the group's stacks, or raises a KeyError
        if the output was not found.
        """
        return _output_named(
            self.stack_outputs(stack),
            self._secret_output_names(stack),
            name,
            required=True,
        )

    def _secret_output_names(self, stack: Input[str]) -> Output[list[str]]:
        return Output.all(Output.from_input(stack), self.members).apply(
            lambda l: _member(l[1], l[0])["secretOutputNames"]
        )


def _member(members: Sequence[Mapping[str, Any]], stack: str) -> Mapping[str, Any]:
    for member in members:
        if member["stack"] == stack:
            return member
    raise KeyError(f"Stack '{stack}' is not a member of this stack group.")


def _output_named(
    outputs: Output[dict[str, Any]],
    secret_output_names: Output[list[str]],
    name: Input[str],
    required: bool,
) -> Output[Any]:
    # A plain `apply` marks its result secret whenever `outputs` holds any secret; deciding from
    # the names keeps outputs that are not secret from being tainted.
    def pick(l: Sequence[Any]) -> Any:
        return l[1][l[0]] if required else l[1].get(l[0])

    value: Output[Any] = Output.all(Output.from_input(name), outputs).apply(pick)  # type: ignore
    is_secret = ensure_future(_is_secret_name(outputs, secret_output_names, name))
    return Output(value.resources(), value.future(), value.is_known(), is_secret)


async def _is_secret_name(
    outputs: Output[dict[str, Any]],
    secret_output_names: Output[list[str]],
    name: Input[str],
) -> bool:
    if not (
        await Output.from_input(name).is_known()
        and await secret_output_names.is_known()
    ):
        return await outputs.is_secret()
    names = await secret_output_names.future()
    if names is None:
        return await outputs.is_secret()
    return await Output.from_input(name).future() in names
