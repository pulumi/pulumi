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
from typing import Any, TypedDict

from pulumi.automation._cmd import CommandResult, PulumiCommand


class BaseOptions(TypedDict, total=False):
    cwd: str
    additional_env: Mapping[str, str]
    on_output: Callable[[str], Any]
    on_error: Callable[[str], Any]


class API:
    _command: PulumiCommand

    def __init__(self, command: PulumiCommand) -> None:
        self._command = command

    def _run(self, options: BaseOptions, args: list[str]) -> CommandResult:
        return self._command.run(
            args,
            options.get("cwd") or os.getcwd(),
            options.get("additional_env") or {},
            options.get("on_output"),
            options.get("on_error"),
        )
