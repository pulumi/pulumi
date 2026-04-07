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

import os
from collections.abc import Callable, Mapping
from typing import Any, Optional

from pulumi.automation._cmd import CommandResult, PulumiCommand


class BaseOptions:
    cwd: Optional[str]
    additional_env: Optional[Mapping[str, str]]
    on_output: Optional[Callable[[str], Any]]
    on_error: Optional[Callable[[str], Any]]


class API:
    _command: PulumiCommand

    def __init__(self, command: PulumiCommand) -> None:
        self._command = command

    def _run(self, options: BaseOptions, args: list[str]) -> CommandResult:
        return self._command.run(
            args,
            options.cwd if getattr(options, "cwd", None) is not None else os.getcwd(),
            getattr(options, "additional_env", None) or {},
            getattr(options, "on_output", None),
            getattr(options, "on_error", None),
        )
