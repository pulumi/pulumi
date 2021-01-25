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

import os
import tempfile
import json
import yaml
from datetime import datetime
from typing import Optional, List, Mapping, Callable

from .config import ConfigMap, ConfigValue
from .project_settings import ProjectSettings
from .stack_settings import StackSettings
from .workspace import Workspace, PluginInfo, StackSummary, WhoAmIResult, PulumiFn, Deployment
from .stack import _DATETIME_FORMAT, Stack
from .cmd import _run_pulumi_cmd, CommandResult, OnOutput

_setting_extensions = [".yaml", ".yml", ".json"]


class LocalWorkspaceOptions:
    work_dir: Optional[str] = None
    pulumi_home: Optional[str] = None
    program: Optional[PulumiFn] = None
    env_vars: Optional[Mapping[str, str]] = None
    secrets_provider: Optional[str] = None
    project_settings: Optional[ProjectSettings] = None
    stack_settings: Optional[Mapping[str, StackSettings]] = None

    def __init__(self,
                 work_dir: Optional[str] = None,
                 pulumi_home: Optional[str] = None,
                 program: Optional[PulumiFn] = None,
                 env_vars: Optional[Mapping[str, str]] = None,
                 secrets_provider: Optional[str] = None,
                 project_settings: Optional[ProjectSettings] = None,
                 stack_settings: Optional[Mapping[str, StackSettings]] = None):
        self.work_dir = work_dir
        self.pulumi_home = pulumi_home
        self.program = program
        self.env_vars = env_vars
        self.secrets_provider = secrets_provider
        self.project_settings = project_settings
        self.stack_settings = stack_settings


class LocalWorkspace(Workspace):
    """
    LocalWorkspace is a default implementation of the Workspace interface.
    A Workspace is the execution context containing a single Pulumi project, a program,
    and multiple stacks. Workspaces are used to manage the execution environment,
    providing various utilities such as plugin installation, environment configuration
    ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    LocalWorkspace relies on Pulumi.yaml and Pulumi.[stack].yaml as the intermediate format
    for Project and Stack settings. Modifying ProjectSettings will
    alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.[stack].yaml file.
    This is identical to the behavior of Pulumi CLI driven workspaces.
    """
    def __init__(self,
                 work_dir: Optional[str] = None,
                 pulumi_home: Optional[str] = None,
                 program: Optional[PulumiFn] = None,
                 env_vars: Optional[Mapping[str, str]] = None,
                 secrets_provider: Optional[str] = None,
                 project_settings: Optional[ProjectSettings] = None,
                 stack_settings: Optional[Mapping[str, StackSettings]] = None):
        self.pulumi_home = pulumi_home
        self.program = program
        self.secrets_provider = secrets_provider
        self.env_vars = env_vars or {}
        self.work_dir = work_dir or tempfile.mkdtemp(dir=tempfile.gettempdir(), prefix="automation-")

        if project_settings:
            self.save_project_settings(project_settings)
        if stack_settings:
            for key in stack_settings:
                self.save_stack_settings(key, stack_settings[key])

    def __repr__(self):
        return f"{self.__class__.__name__}(work_dir={self.work_dir!r}, program={self.program.__name__}, " \
               f"pulumi_home={self.pulumi_home!r}, env_vars={self.env_vars!r}, " \
               f"secrets_provider={self.secrets_provider})"

    def project_settings(self) -> ProjectSettings:
        for ext in _setting_extensions:
            project_path = os.path.join(self.work_dir, f"Pulumi{ext}")
            if not os.path.exists(project_path):
                continue
            with open(project_path, "r") as file:
                settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
                return ProjectSettings(**settings)
        raise FileNotFoundError(f"failed to find project settings file in workdir: {self.work_dir}")

    def save_project_settings(self, settings: ProjectSettings) -> None:
        found_ext = ".yaml"
        for ext in _setting_extensions:
            test_path = os.path.join(self.work_dir, f"Pulumi{ext}")
            if os.path.exists(test_path):
                found_ext = ext
                break
        path = os.path.join(self.work_dir, f"Pulumi{found_ext}")
        with open(path, "w") as file:
            json.dump(settings, file, indent=4) if found_ext == ".json" else yaml.dump(settings, stream=file)

    def stack_settings(self, stack_name: str) -> StackSettings:
        stack_settings_name = get_stack_settings_name(stack_name)
        for ext in _setting_extensions:
            path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{ext}")
            if not os.path.exists(path):
                continue
            with open(path, "r") as file:
                settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
                return StackSettings(
                    secrets_provider=settings["secretsProvider"] if "secretsProvider" in settings else None,
                    encrypted_key=settings["encryptedKey"] if "encryptedKey" in settings else None,
                    encryption_salt=settings["encryptionSalt"] if "encryptionSalt" in settings else None,
                    config=settings["config"] if "config" in settings else None)
        raise FileNotFoundError(f"failed to find stack settings file in workdir: {self.work_dir}")

    def save_stack_settings(self, stack_name: str, settings: StackSettings) -> None:
        stack_settings_name = get_stack_settings_name(stack_name)
        found_ext = ".yaml"
        for ext in _setting_extensions:
            test_path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{ext}")
            if os.path.exists(test_path):
                found_ext = ext
                break
        path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{found_ext}")
        with open(path, "w") as file:
            json.dump(settings, file, indent=4) if found_ext == ".json" else yaml.dump(settings, stream=file)

    def serialize_args_for_op(self, stack_name: str) -> List[str]:
        # Not used by LocalWorkspace
        return []

    def post_command_callback(self, stack_name: str) -> None:
        # Not used by LocalWorkspace
        return

    def get_config(self, stack_name: str, key: str) -> ConfigValue:
        self.select_stack(stack_name)
        result = self._run_pulumi_cmd_sync(["config", "get", key, "--json"])
        val = json.loads(result.stdout)
        return ConfigValue(**val)

    def get_all_config(self, stack_name: str) -> ConfigMap:
        self.select_stack(stack_name)
        result = self._run_pulumi_cmd_sync(["config", "--show-secrets", "--json"])
        config_json = json.loads(result.stdout)
        config_map: ConfigMap = {}
        for key in config_json:
            config_map[key] = ConfigValue(**config_json[key])
        return config_map

    def set_config(self, stack_name: str, key: str, value: ConfigValue) -> None:
        self.select_stack(stack_name)
        secret_arg = "--secret" if value.secret else "--plaintext"
        self._run_pulumi_cmd_sync(["config", "set", key, value.value, secret_arg])

    def set_all_config(self, stack_name: str, config: ConfigMap) -> None:
        # TODO: Do this in parallel after https://github.com/pulumi/pulumi/issues/6050
        for key, value in config.items():
            self.set_config(stack_name, key, value)

    def remove_config(self, stack_name: str, key: str) -> None:
        self.select_stack(stack_name)
        self._run_pulumi_cmd_sync(["config", "rm", key])

    def remove_all_config(self, stack_name: str, keys: List[str]) -> None:
        # TODO: Do this in parallel after https://github.com/pulumi/pulumi/issues/6050
        for key in keys:
            self.remove_config(stack_name, key)

    def refresh_config(self, stack_name: str) -> None:
        self.select_stack(stack_name)
        self._run_pulumi_cmd_sync(["config", "refresh", "--force"])
        self.get_all_config(stack_name)

    def who_am_i(self) -> WhoAmIResult:
        result = self._run_pulumi_cmd_sync(["whoami"])
        return WhoAmIResult(user=result.stdout.strip())

    def stack(self) -> Optional[StackSummary]:
        stacks = self.list_stacks()
        for stack in stacks:
            if stack.current:
                return stack
        return None

    def create_stack(self, stack_name: str) -> None:
        args = ["stack", "init", stack_name]
        if self.secrets_provider:
            args.extend(["--secrets-provider", self.secrets_provider])
        self._run_pulumi_cmd_sync(args)

    def select_stack(self, stack_name: str) -> None:
        self._run_pulumi_cmd_sync(["stack", "select", stack_name])

    def remove_stack(self, stack_name: str) -> None:
        self._run_pulumi_cmd_sync(["stack", "rm", "--yes", stack_name])

    def list_stacks(self) -> List[StackSummary]:
        result = self._run_pulumi_cmd_sync(["stack", "ls", "--json"])
        json_list = json.loads(result.stdout)
        stack_list: List[StackSummary] = []
        for stack_json in json_list:
            stack = StackSummary(
                name=stack_json["name"],
                current=stack_json["current"],
                update_in_progress=stack_json["updateInProgress"],
                last_update=datetime.strptime(stack_json["lastUpdate"], _DATETIME_FORMAT) if "lastUpdate" in stack_json else None,
                resource_count=stack_json["resourceCount"] if "resourceCount" in stack_json else None,
                url=stack_json["url"] if "url" in stack_json else None)
            stack_list.append(stack)
        return stack_list

    def install_plugin(self, name: str, version: str, kind: str = "resource") -> None:
        self._run_pulumi_cmd_sync(["plugin", "install", kind, name, version])

    def remove_plugin(self,
                      name: Optional[str] = None,
                      version_range: Optional[str] = None,
                      kind: str = "resource") -> None:
        args = ["plugin", "rm", kind]
        if name:
            args.append(name)
        if version_range:
            args.append(version_range)
        args.append("--yes")
        self._run_pulumi_cmd_sync(args)

    def list_plugins(self) -> List[PluginInfo]:
        result = self._run_pulumi_cmd_sync(["plugin", "ls", "--json"])
        json_list = json.loads(result.stdout)
        plugin_list: List[PluginInfo] = []
        for plugin_json in json_list:
            plugin = PluginInfo(
                name=plugin_json["name"],
                kind=plugin_json["kind"],
                size=plugin_json["size"],
                last_used_time=datetime.strptime(plugin_json["lastUsedTime"], _DATETIME_FORMAT),
                install_time=datetime.strptime(plugin_json["installTime"], _DATETIME_FORMAT) if "installTime" in plugin_json else None,
                version=plugin_json["version"] if "version" in plugin_json else None
            )
            plugin_list.append(plugin)
        return plugin_list

    def export_stack(self, stack_name: str) -> Deployment:
        self.select_stack(stack_name)
        result = self._run_pulumi_cmd_sync(["stack", "export", "--show-secrets"])
        state_json = json.loads(result.stdout)
        return Deployment(**state_json)

    def import_stack(self, stack_name: str, state: Deployment) -> None:
        self.select_stack(stack_name)
        file = tempfile.NamedTemporaryFile(mode="w", delete=False)
        json.dump(state.__dict__, file, indent=4)
        file.close()
        self._run_pulumi_cmd_sync(["stack", "import", "--file", file.name])
        os.remove(file.name)

    def _run_pulumi_cmd_sync(self, args: List[str], on_output: Optional[OnOutput] = None) -> CommandResult:
        envs = {"PULUMI_HOME": self.pulumi_home} if self.pulumi_home else {}
        envs = {**envs, **self.env_vars}
        return _run_pulumi_cmd(args, self.work_dir, envs, on_output)


def _is_inline_program(**kwargs) -> bool:
    for key in ["program", "project_name"]:
        if key not in kwargs or kwargs[key] is None:
            return False
    return True


StackInitializer = Callable[[str, Workspace], Stack]


def create_stack(stack_name: str,
                 project_name: Optional[str] = None,
                 program: Optional[PulumiFn] = None,
                 work_dir: Optional[str] = None,
                 opts: Optional[LocalWorkspaceOptions] = None) -> Stack:
    """
    Creates a Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program or the local
    Pulumi CLI program from the specified workdir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. If no project_settings option is specified, default project settings will be
    created on behalf of the user. Similarly, unless a `work_dir` option is specified, the working directory will
    default to a new temporary directory provided by the OS.

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided.
    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    :param stack_name: The name of the stack.
    :param project_name: The name of the project - required for inline programs.
    :param program: The inline program - required for inline programs.
    :param work_dir: The directory for a CLI-driven stack - required for local programs.
    :param opts: Extensibility options to configure a LocalWorkspace; e.g: settings to seed and environment
           variables to pass through to every command.
    :return: Stack
    """
    args = locals()
    if _is_inline_program(**args):
        # Type checks are ignored because we have already asserted that the correct args are present.
        return _inline_source_stack_helper(stack_name, program, project_name, Stack.create, opts)  # type: ignore
    elif _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.create, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def select_stack(stack_name: str,
                 project_name: Optional[str] = None,
                 program: Optional[PulumiFn] = None,
                 work_dir: Optional[str] = None,
                 opts: Optional[LocalWorkspaceOptions] = None) -> Stack:
    """
    Selects a Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program or the local
    Pulumi CLI program from the specified workdir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. If no project_settings option is specified, default project settings will be
    created on behalf of the user. Similarly, unless a `work_dir` option is specified, the working directory will
    default to a new temporary directory provided by the OS.

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided.
    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    :param stack_name: The name of the stack.
    :param project_name: The name of the project - required for inline programs.
    :param program: The inline program - required for inline programs.
    :param work_dir: The directory for a CLI-driven stack - required for local programs.
    :param opts: Extensibility options to configure a LocalWorkspace; e.g: settings to seed and environment
           variables to pass through to every command.
    :return: Stack
    """
    args = locals()
    if _is_inline_program(**args):
        return _inline_source_stack_helper(stack_name, program, project_name, Stack.select, opts)  # type: ignore
    elif _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.select, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def create_or_select_stack(stack_name: str,
                           project_name: Optional[str] = None,
                           program: Optional[PulumiFn] = None,
                           work_dir: Optional[str] = None,
                           opts: Optional[LocalWorkspaceOptions] = None) -> Stack:
    """
    Creates or selects an existing Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi
    program or the local Pulumi CLI program from the specified workdir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. If no project_settings option is specified, default project settings will be
    created on behalf of the user. Similarly, unless a `work_dir` option is specified, the working directory will
    default to a new temporary directory provided by the OS.

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided.
    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    :param stack_name: The name of the stack.
    :param project_name: The name of the project - required for inline programs.
    :param program: The inline program - required for inline programs.
    :param work_dir: The directory for a CLI-driven stack - required for local programs.
    :param opts: Extensibility options to configure a LocalWorkspace; e.g: settings to seed and environment
           variables to pass through to every command.
    :return: Stack
    """
    args = locals()
    if _is_inline_program(**args):
        return _inline_source_stack_helper(stack_name, program, project_name, Stack.create_or_select, opts)  # type: ignore
    elif _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.create_or_select, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def _inline_source_stack_helper(stack_name: str,
                                program: PulumiFn,
                                project_name: str,
                                init_fn: StackInitializer,
                                opts: Optional[LocalWorkspaceOptions] = None):
    workspace_options = opts or LocalWorkspaceOptions()
    workspace_options.program = program

    if not workspace_options.project_settings:
        workspace_options.project_settings = default_project(project_name)

    ws = LocalWorkspace(**workspace_options.__dict__)
    return init_fn(stack_name, ws)


def _is_local_program(**kwargs) -> bool:
    return "work_dir" in kwargs and kwargs["work_dir"] is not None


def _local_source_stack_helper(stack_name: str,
                               work_dir: str,
                               init_fn: StackInitializer,
                               opts: Optional[LocalWorkspaceOptions] = None):
    workspace_options = opts or LocalWorkspaceOptions()
    workspace_options.work_dir = work_dir

    ws = LocalWorkspace(**workspace_options.__dict__)
    return init_fn(stack_name, ws)


def default_project(project_name: str) -> ProjectSettings:
    return ProjectSettings(name=project_name, runtime="python")


def get_stack_settings_name(name: str) -> str:
    parts = name.split("/")
    if len(parts) < 1:
        return name
    return parts[-1]
