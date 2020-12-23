# Copyright 2016-2020, Pulumi Corporation.
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

from __future__ import annotations
from typing import List
from .cmd import CommandResult, _run_pulumi_cmd
from .config import ConfigValue, ConfigMap
from .errors import StackAlreadyExistsError
from .workspace import Workspace


def fully_qualified_stack_name(org: str, project: str, stack: str) -> str:
    """
    Returns a stack name formatted with the greatest possible specificity:
    org/project/stack or user/project/stack
    Using this format avoids ambiguity in stack identity guards creating or selecting the wrong stack.
    Note that filestate backends (local file, S3, Azure Blob) do not support stack names in this
    format, and instead only use the stack name without an org/user or project to qualify it.
    See: https://github.com/pulumi/pulumi/issues/2522
    """
    return f"{org}/{project}/{stack}"


class Stack:
    name: str
    """The name identifying the Stack."""

    workspace: Workspace
    """The Workspace the Stack was created from."""

    def __init__(self, name: str, workspace: Workspace, select_if_exists: bool = False) -> None:
        self.name = name
        self.workspace = workspace

        try:
            workspace.create_stack(name)
        except StackAlreadyExistsError:
            if select_if_exists:
                workspace.select_stack(name)
            else:
                raise

    def get_config(self, key: str) -> ConfigValue:
        """Returns the config value associated with the specified key."""
        return self.workspace.get_config(self.name, key)

    def get_all_config(self) -> ConfigMap:
        """Returns the full config map associated with the stack in the Workspace."""
        return self.workspace.get_all_config(self.name)

    def set_config(self, key: str, value: ConfigValue) -> None:
        """Sets a config key-value pair on the Stack in the associated Workspace."""
        self.workspace.set_config(self.name, key, value)

    def set_all_config(self, config: ConfigMap) -> None:
        """Sets all specified config values on the stack in the associated Workspace."""
        self.workspace.set_all_config(self.name, config)

    def remove_config(self, key: str) -> None:
        """Removes the specified config key from the Stack in the associated Workspace."""
        self.workspace.remove_config(self.name, key)

    def remove_all_config(self, keys: List[str]) -> None:
        """Removes the specified config keys from the Stack in the associated Workspace."""
        self.workspace.remove_all_config(self.name, keys)

    def refresh_config(self) -> None:
        """Gets and sets the config map used with the last update."""
        self.workspace.refresh_config(self.name)

    def _run_pulumi_cmd_sync(self, args: List[str]) -> CommandResult:
        envs = {"PULUMI_HOME": self.workspace.pulumi_home} if self.workspace.pulumi_home else {}
        envs = {**envs, **self.workspace.env_vars}

        additional_args = self.workspace.serialize_args_for_op(self.name)
        args.extend(additional_args)

        result = _run_pulumi_cmd(args, self.workspace.work_dir, envs)
        self.workspace.post_command_callback(self.name)
        return result
