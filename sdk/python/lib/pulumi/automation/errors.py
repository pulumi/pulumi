# Copyright 2016-2021, Pulumi Corporation.
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

import re
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from ._cmd import CommandResult


class CommandError(Exception):
    def __init__(self, command_result: "CommandResult"):
        self.name = "CommandError"
        super().__init__(str(command_result))


class StackNotFoundError(CommandError):
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "StackNotFoundError"


class ConcurrentUpdateError(CommandError):
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "ConcurrentUpdateError"


class StackAlreadyExistsError(CommandError):
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "StackAlreadyExistsError"


class InlineSourceRuntimeError(CommandError):
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "InlineSourceRuntimeError"


class RuntimeError(CommandError):  # pylint: disable=redefined-builtin
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "RuntimeError"


class CompilationError(CommandError):
    def __init__(self, command_result: "CommandResult"):
        super().__init__(command_result)
        self.name = "CompilationError"


class InvalidVersionError(Exception):
    pass


not_found_regex = re.compile("no stack named.*found")
already_exists_regex = re.compile("stack.*already exists")
conflict_text = "[409] Conflict: Another update is currently in progress."
diy_backend_conflict_text = "the stack is currently locked by"
inline_source_error_text = "python inline source runtime error"
runtime_error_regex = re.compile(
    "failed with an unhandled exception|panic: runtime error|an unhandled error occurred:"
)
compilation_error_regex = re.compile(
    "Build FAILED.|Unable to compile TypeScript|: syntax error:|: undefined:"
)


def create_command_error(command_result: "CommandResult") -> CommandError:
    stderr = command_result.stderr
    stdout = command_result.stdout
    if not_found_regex.search(stderr):
        return StackNotFoundError(command_result)
    if already_exists_regex.search(stderr):
        return StackAlreadyExistsError(command_result)
    if conflict_text in stderr:
        return ConcurrentUpdateError(command_result)
    if diy_backend_conflict_text in stderr:
        return ConcurrentUpdateError(command_result)
    if compilation_error_regex.search(stdout):
        return CompilationError(command_result)
    if inline_source_error_text in stdout:
        return InlineSourceRuntimeError(command_result)
    if runtime_error_regex.search(stdout):
        return RuntimeError(command_result)
    return CommandError(command_result)
