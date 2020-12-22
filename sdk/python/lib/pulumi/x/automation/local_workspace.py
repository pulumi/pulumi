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

import os
import tempfile
import json
import yaml
from typing import Optional, List, Awaitable, Mapping, Callable, Any

from .config import ConfigMap, ConfigValue
from .project_settings import ProjectSettings
from .stack_settings import StackSettings
from .workspace import Workspace
from .cmd import _run_pulumi_cmd, CommandResult

setting_extensions = [".yaml", ".yml", ".json"]


class LocalWorkspace(Workspace):
    """
    LocalWorkspace is a default implementation of the Workspace interface.
    A Workspace is the execution context containing a single Pulumi project, a program,
    and multiple stacks. Workspaces are used to manage the execution environment,
    providing various utilities such as plugin installation, environment configuration
    ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    LocalWorkspace relies on Pulumi.yaml and Pulumi.<stack>.yaml as the intermediate format
    for Project and Stack settings. Modifying ProjectSettings will
    alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.<stack>.yaml file.
    This is identical to the behavior of Pulumi CLI driven workspaces.
    """

    def __init__(self,
                 work_dir: Optional[str] = None,
                 pulumi_home: Optional[str] = None,
                 program: Optional[Callable[[], Any]] = None,
                 env_vars: Mapping[str, str] = None,
                 secrets_provider: Optional[str] = None,
                 project_settings: Optional[ProjectSettings] = None,
                 stack_settings: Optional[Mapping[str, StackSettings]] = None):
        self.pulumi_home = pulumi_home
        self.program = program
        self.secrets_provider = secrets_provider
        self.envs = env_vars or {}
        self.work_dir = work_dir or tempfile.mkdtemp(dir=tempfile.gettempdir(), prefix="automation-")

        if project_settings:
            self.save_project_settings(project_settings)
        if stack_settings:
            for key in stack_settings:
                self.save_stack_settings(key, stack_settings[key])

    def project_settings(self) -> ProjectSettings:
        for ext in setting_extensions:
            project_path = os.path.join(self.work_dir, f"Pulumi{ext}")
            if not os.path.exists(project_path):
                continue
            with open(project_path, "r") as file:
                settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
                return ProjectSettings(**settings)
        raise FileNotFoundError(f"failed to find project settings file in workdir: {self.work_dir}")

    async def save_project_settings(self, settings: ProjectSettings) -> None:
        pass

    def stack_settings(self, stack_name: str) -> StackSettings:
        stack_settings_name = get_stack_settings_name(stack_name)
        for ext in setting_extensions:
            path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{ext}")
            if not os.path.exists(path):
                continue
            with open(path, "r") as file:
                settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
                return StackSettings(**settings)
        raise FileNotFoundError(f"failed to find stack settings file in workdir: {self.work_dir}")

    async def save_stack_settings(self, stack_name: str, settings: StackSettings) -> None:
        pass

    async def serialize_args_for_op(self, stack_name: str) -> None:
        pass

    async def post_command_callback(self, stack_name: str) -> None:
        pass

    async def get_config(self, stack_name: str, key: str) -> Awaitable[ConfigValue]:
        pass

    async def get_all_config(self, stack_name: str) -> Awaitable[ConfigMap]:
        pass

    async def set_config(self, stack_name: str, key: str, value: ConfigValue) -> None:
        pass

    async def set_all_config(self, stack_name: str, config: ConfigMap) -> None:
        pass

    async def remove_config(self, stack_name: str, key: str) -> None:
        pass

    async def remove_all_config(self, stack_name: str, keys: List[str]) -> None:
        pass

    async def refresh_config(self, stack_name: str) -> None:
        pass

    async def who_am_i(self) -> None:
        pass

    async def stack(self) -> None:
        pass

    def create_stack(self, stack_name: str) -> None:
        args = ["stack", "init", stack_name]
        if self.secrets_provider:
            args.extend(["--secrets-provider", self.secrets_provider])
        self._run_pulumi_cmd_sync(args)

    async def select_stack(self, stack_name: str) -> None:
        pass

    async def remove_stack(self, stack_name: str) -> None:
        pass

    async def list_stacks(self) -> List[dict]:
        pass

    async def install_plugin(self, plugin_name: str, version: str, kind: Optional[str]) -> None:
        pass

    async def remove_plugin(self, plugin_name: Optional[str], version_range: Optional[str],
                            kind: Optional[str]) -> None:
        pass

    async def list_plugins(self) -> None:
        pass

    def _run_pulumi_cmd_sync(self, args: List[str]) -> CommandResult:
        envs = {"PULUMI_HOME": self.pulumi_home} if self.pulumi_home else {}
        envs = {**envs, **self.env_vars}
        return _run_pulumi_cmd(args, self.work_dir, envs)


def get_stack_settings_name(name: str) -> str:
    parts = name.split("/")
    if len(parts) < 1:
        return name
    return parts[-1]
