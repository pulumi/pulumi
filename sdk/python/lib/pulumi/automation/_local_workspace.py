# Copyright 2016-2023, Pulumi Corporation.
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

import json
import os
import tempfile
from datetime import datetime
from typing import TYPE_CHECKING, Callable, List, Mapping, Optional, Union

import yaml
from semver import VersionInfo

from ._cmd import CommandResult, OnOutput, _run_pulumi_cmd
from ._config import _SECRET_SENTINEL, ConfigMap, ConfigValue
from ._minimum_version import _MINIMUM_VERSION
from ._output import OutputMap, OutputValue
from ._project_settings import ProjectSettings
from ._stack import _DATETIME_FORMAT, Stack
from ._stack_settings import StackSettings
from ._tag import TagMap
from ._workspace import (
    Deployment,
    PluginInfo,
    PulumiFn,
    StackSummary,
    WhoAmIResult,
    Workspace,
)
from .errors import InvalidVersionError

if TYPE_CHECKING:
    from pulumi.automation._remote_workspace import RemoteGitAuth

_setting_extensions = [".yaml", ".yml", ".json"]

_SKIP_VERSION_CHECK_VAR = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK"


class Secret(str):
    """
    Represents a secret value.
    """


class LocalWorkspaceOptions:
    work_dir: Optional[str] = None
    pulumi_home: Optional[str] = None
    program: Optional[PulumiFn] = None
    env_vars: Optional[Mapping[str, str]] = None
    secrets_provider: Optional[str] = None
    project_settings: Optional[ProjectSettings] = None
    stack_settings: Optional[Mapping[str, StackSettings]] = None

    def __init__(
        self,
        work_dir: Optional[str] = None,
        pulumi_home: Optional[str] = None,
        program: Optional[PulumiFn] = None,
        env_vars: Optional[Mapping[str, str]] = None,
        secrets_provider: Optional[str] = None,
        project_settings: Optional[ProjectSettings] = None,
        stack_settings: Optional[Mapping[str, StackSettings]] = None,
    ):
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

    _remote: bool = False
    _remote_env_vars: Optional[Mapping[str, Union[str, Secret]]]
    _remote_pre_run_commands: Optional[List[str]]
    _remote_skip_install_dependencies: Optional[bool]
    _remote_git_url: str
    _remote_git_project_path: Optional[str]
    _remote_git_branch: Optional[str]
    _remote_git_commit_hash: Optional[str]
    _remote_git_auth: Optional[RemoteGitAuth]

    def __init__(
        self,
        work_dir: Optional[str] = None,
        pulumi_home: Optional[str] = None,
        program: Optional[PulumiFn] = None,
        env_vars: Optional[Mapping[str, str]] = None,
        secrets_provider: Optional[str] = None,
        project_settings: Optional[ProjectSettings] = None,
        stack_settings: Optional[Mapping[str, StackSettings]] = None,
    ):
        self.pulumi_home = pulumi_home
        self.program = program
        self.secrets_provider = secrets_provider
        self.env_vars = env_vars or {}
        self.work_dir = work_dir or tempfile.mkdtemp(
            dir=tempfile.gettempdir(), prefix="automation-"
        )

        pulumi_version = self._get_pulumi_version()
        opt_out = self._version_check_opt_out()
        version = _parse_and_validate_pulumi_version(
            _MINIMUM_VERSION, pulumi_version, opt_out
        )
        self.__pulumi_version = str(version) if version else None

        if project_settings:
            self.save_project_settings(project_settings)
        if stack_settings:
            for key in stack_settings:
                self.save_stack_settings(key, stack_settings[key])

    # mypy does not support properties: https://github.com/python/mypy/issues/1362
    @property  # type: ignore
    def pulumi_version(self) -> str:  # type: ignore
        if self.__pulumi_version:
            return self.__pulumi_version
        raise InvalidVersionError("Could not get Pulumi CLI version")

    @pulumi_version.setter  # type: ignore
    def pulumi_version(self, v: str):
        self.__pulumi_version = v

    def __repr__(self):
        return (
            f"{self.__class__.__name__}(work_dir={self.work_dir!r}, "
            f"program={self.program.__name__ if self.program else None}, "
            f"pulumi_home={self.pulumi_home!r}, env_vars={self.env_vars!r}, "
            f"secrets_provider={self.secrets_provider})"
        )

    def project_settings(self) -> ProjectSettings:
        return _load_project_settings(self.work_dir)

    def save_project_settings(self, settings: ProjectSettings) -> None:
        found_ext = ".yaml"
        for ext in _setting_extensions:
            test_path = os.path.join(self.work_dir, f"Pulumi{ext}")
            if os.path.exists(test_path):
                found_ext = ext
                break
        path = os.path.join(self.work_dir, f"Pulumi{found_ext}")
        writable_settings = {
            key: settings.__dict__[key]
            for key in settings.__dict__
            if settings.__dict__[key] is not None
        }
        with open(path, "w", encoding="utf-8") as file:
            if found_ext == ".json":
                json.dump(writable_settings, file, indent=4)
            else:
                yaml.dump(writable_settings, stream=file)

    def stack_settings(self, stack_name: str) -> StackSettings:
        stack_settings_name = get_stack_settings_name(stack_name)
        for ext in _setting_extensions:
            path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{ext}")
            if not os.path.exists(path):
                continue
            with open(path, "r", encoding="utf-8") as file:
                settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
                return StackSettings._deserialize(settings)
        raise FileNotFoundError(
            f"failed to find stack settings file in workdir: {self.work_dir}"
        )

    def save_stack_settings(self, stack_name: str, settings: StackSettings) -> None:
        stack_settings_name = get_stack_settings_name(stack_name)
        found_ext = ".yaml"
        for ext in _setting_extensions:
            test_path = os.path.join(
                self.work_dir, f"Pulumi.{stack_settings_name}{ext}"
            )
            if os.path.exists(test_path):
                found_ext = ext
                break
        path = os.path.join(self.work_dir, f"Pulumi.{stack_settings_name}{found_ext}")
        with open(path, "w", encoding="utf-8") as file:
            if found_ext == ".json":
                json.dump(settings._serialize(), file, indent=4)
            else:
                yaml.dump(settings._serialize(), stream=file)

    def serialize_args_for_op(self, stack_name: str) -> List[str]:
        # Not used by LocalWorkspace
        return []

    def post_command_callback(self, stack_name: str) -> None:
        # Not used by LocalWorkspace
        return

    def add_environments(self, stack_name: str, *environment_names: str) -> None:
        # Assume an old version. Doesn't really matter what this is as long as it's pre-3.95.
        ver = VersionInfo(3)
        if self.__pulumi_version is not None:
            ver = VersionInfo.parse(self.__pulumi_version)

        # 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
        if ver >= VersionInfo(3, 95):
            args = ["config", "env", "add"]
            args.extend(environment_names)
            args.extend(["--yes", "--stack", stack_name])
            self._run_pulumi_cmd_sync(args)
        else:
            raise InvalidVersionError(
                "The installed version of the CLI does not support this operation. Please "
                "upgrade to at least version 3.95.0."
            )

    def remove_environment(self, stack_name: str, environment_name: str) -> None:
        # Assume an old version. Doesn't really matter what this is as long as it's pre-3.95.
        ver = VersionInfo(3)
        if self.__pulumi_version is not None:
            ver = VersionInfo.parse(self.__pulumi_version)

        # 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
        if ver >= VersionInfo(3, 95):
            args = [
                "config",
                "env",
                "rm",
                environment_name,
                "--yes",
                "--stack",
                stack_name,
            ]
            self._run_pulumi_cmd_sync(args)
        else:
            raise InvalidVersionError(
                "The installed version of the CLI does not support this operation. Please "
                "upgrade to at least version 3.95.0."
            )

    def get_config(
        self, stack_name: str, key: str, *, path: bool = False
    ) -> ConfigValue:
        args = ["config", "get"]
        if path:
            args.append("--path")
        args.extend([key, "--json", "--stack", stack_name])
        result = self._run_pulumi_cmd_sync(args)
        val = json.loads(result.stdout)
        return ConfigValue(value=val["value"], secret=val["secret"])

    def get_all_config(self, stack_name: str) -> ConfigMap:
        result = self._run_pulumi_cmd_sync(
            ["config", "--show-secrets", "--json", "--stack", stack_name]
        )
        config_json = json.loads(result.stdout)
        config_map: ConfigMap = {}
        for key in config_json:
            config_val_json = config_json[key]
            config_map[key] = ConfigValue(
                value=config_val_json["value"], secret=config_val_json["secret"]
            )
        return config_map

    def set_config(
        self, stack_name: str, key: str, value: ConfigValue, *, path: bool = False
    ) -> None:
        args = ["config", "set"]
        if path:
            args.append("--path")
        secret_arg = "--secret" if value.secret else "--plaintext"
        args.extend(
            [
                key,
                secret_arg,
                "--stack",
                stack_name,
                "--non-interactive",
                "--",
                value.value,
            ]
        )
        self._run_pulumi_cmd_sync(args)

    def set_all_config(
        self, stack_name: str, config: ConfigMap, *, path: bool = False
    ) -> None:
        args = ["config", "set-all", "--stack", stack_name]
        if path:
            args.append("--path")

        for key, value in config.items():
            secret_arg = "--secret" if value.secret else "--plaintext"
            args.extend([secret_arg, f"{key}={value.value}"])

        self._run_pulumi_cmd_sync(args)

    def remove_config(self, stack_name: str, key: str, *, path: bool = False) -> None:
        args = ["config", "rm", key, "--stack", stack_name]
        if path:
            args.append("--path")
        self._run_pulumi_cmd_sync(args)

    def remove_all_config(
        self, stack_name: str, keys: List[str], *, path: bool = False
    ) -> None:
        args = ["config", "rm-all", "--stack", stack_name]
        if path:
            args.append("--path")
        args.extend(keys)
        self._run_pulumi_cmd_sync(args)

    def refresh_config(self, stack_name: str) -> None:
        self._run_pulumi_cmd_sync(
            ["config", "refresh", "--force", "--stack", stack_name]
        )
        self.get_all_config(stack_name)

    def get_tag(self, stack_name: str, key: str) -> str:
        result = self._run_pulumi_cmd_sync(
            ["stack", "tag", "get", key, "--stack", stack_name]
        )
        return result.stdout.strip()

    def set_tag(self, stack_name: str, key: str, value: str) -> None:
        self._run_pulumi_cmd_sync(
            ["stack", "tag", "set", key, value, "--stack", stack_name]
        )

    def remove_tag(self, stack_name: str, key: str) -> None:
        self._run_pulumi_cmd_sync(["stack", "tag", "rm", key, "--stack", stack_name])

    def list_tags(self, stack_name: str) -> TagMap:
        result = self._run_pulumi_cmd_sync(
            ["stack", "tag", "ls", "--json", "--stack", stack_name]
        )
        return json.loads(result.stdout)

    def who_am_i(self) -> WhoAmIResult:
        # Assume an old version. Doesn't really matter what this is as long as it's pre-3.58.
        ver = VersionInfo(3)
        if self.__pulumi_version is not None:
            ver = VersionInfo.parse(self.__pulumi_version)

        # 3.58 added the --json flag (https://github.com/pulumi/pulumi/releases/tag/v3.58.0)
        if ver >= VersionInfo(3, 58):
            result = self._run_pulumi_cmd_sync(["whoami", "--json"])
            who_am_i_json = json.loads(result.stdout)
            return WhoAmIResult(**who_am_i_json)

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
        if self._remote:
            args.append("--no-select")
        self._run_pulumi_cmd_sync(args)

    def select_stack(self, stack_name: str) -> None:
        # If this is a remote workspace, we don't want to actually select the stack (which would modify global state);
        # but we will ensure the stack exists by calling `pulumi stack`.
        args: List[str] = ["stack"]
        if not self._remote:
            args.append("select")
        args.append("--stack")
        args.append(stack_name)
        self._run_pulumi_cmd_sync(args)

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
                update_in_progress=stack_json["updateInProgress"]
                if "updateInProgress" in stack_json
                else None,
                last_update=datetime.strptime(
                    stack_json["lastUpdate"], _DATETIME_FORMAT
                )
                if "lastUpdate" in stack_json
                else None,
                resource_count=stack_json["resourceCount"]
                if "resourceCount" in stack_json
                else None,
                url=stack_json["url"] if "url" in stack_json else None,
            )
            stack_list.append(stack)
        return stack_list

    def install_plugin(self, name: str, version: str, kind: str = "resource") -> None:
        self._run_pulumi_cmd_sync(["plugin", "install", kind, name, version])

    def install_plugin_from_server(self, name: str, version: str, server: str) -> None:
        self._run_pulumi_cmd_sync(
            ["plugin", "install", "resource", name, version, "--server", server]
        )

    def remove_plugin(
        self,
        name: Optional[str] = None,
        version_range: Optional[str] = None,
        kind: str = "resource",
    ) -> None:
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
                last_used_time=datetime.strptime(
                    plugin_json["lastUsedTime"], _DATETIME_FORMAT
                ),
                install_time=datetime.strptime(
                    plugin_json["installTime"], _DATETIME_FORMAT
                )
                if "installTime" in plugin_json
                else None,
                version=plugin_json["version"] if "version" in plugin_json else None,
            )
            plugin_list.append(plugin)
        return plugin_list

    def export_stack(self, stack_name: str) -> Deployment:
        result = self._run_pulumi_cmd_sync(
            ["stack", "export", "--show-secrets", "--stack", stack_name]
        )
        state_json = json.loads(result.stdout)
        return Deployment(**state_json)

    def import_stack(self, stack_name: str, state: Deployment) -> None:
        with tempfile.NamedTemporaryFile(mode="w", delete=False) as file:
            json.dump(state.__dict__, file, indent=4)
        self._run_pulumi_cmd_sync(
            ["stack", "import", "--file", file.name, "--stack", stack_name]
        )
        os.remove(file.name)

    def stack_outputs(self, stack_name: str) -> OutputMap:
        masked_result = self._run_pulumi_cmd_sync(
            ["stack", "output", "--json", "--stack", stack_name]
        )
        plaintext_result = self._run_pulumi_cmd_sync(
            ["stack", "output", "--json", "--show-secrets", "--stack", stack_name]
        )
        masked_outputs = json.loads(masked_result.stdout)
        plaintext_outputs = json.loads(plaintext_result.stdout)
        outputs: OutputMap = {}
        for key in plaintext_outputs:
            secret = masked_outputs[key] == _SECRET_SENTINEL
            outputs[key] = OutputValue(value=plaintext_outputs[key], secret=secret)
        return outputs

    def _version_check_opt_out(self) -> bool:
        return (
            os.getenv(_SKIP_VERSION_CHECK_VAR) is not None
            or self.env_vars.get(_SKIP_VERSION_CHECK_VAR) is not None
        )

    def _get_pulumi_version(self) -> str:
        result = self._run_pulumi_cmd_sync(["version"])
        version_string = result.stdout.strip()
        if version_string[0] == "v":
            version_string = version_string[1:]
        return version_string

    def _remote_supported(self) -> bool:
        # See if `--remote` is present in `pulumi preview --help`'s output.
        result = self._run_pulumi_cmd_sync(["preview", "--help"])
        help_string = result.stdout.strip()
        return "--remote" in help_string

    def _run_pulumi_cmd_sync(
        self, args: List[str], on_output: Optional[OnOutput] = None
    ) -> CommandResult:
        envs = {"PULUMI_HOME": self.pulumi_home} if self.pulumi_home else {}
        if self._remote:
            envs["PULUMI_EXPERIMENTAL"] = "true"
        envs = {**envs, **self.env_vars}
        return _run_pulumi_cmd(args, self.work_dir, envs, on_output)

    def _remote_args(self) -> List[str]:
        args: List[str] = []
        if not self._remote:
            return args

        args.append("--remote")
        if self._remote_git_url:
            args.append(self._remote_git_url)
        if self._remote_git_project_path:
            args.append("--remote-git-repo-dir")
            args.append(self._remote_git_project_path)
        if self._remote_git_branch:
            args.append("--remote-git-branch")
            args.append(self._remote_git_branch)
        if self._remote_git_commit_hash:
            args.append("--remote-git-commit")
            args.append(self._remote_git_commit_hash)
        auth = self._remote_git_auth
        if auth is not None:
            if auth.personal_access_token:
                args.append("--remote-git-auth-access-token")
                args.append(auth.personal_access_token)
            if auth.ssh_private_key:
                args.append("--remote-git-auth-ssh-private-key")
                args.append(auth.ssh_private_key)
            if auth.ssh_private_key_path:
                args.append("--remote-git-auth-ssh-private-key-path")
                args.append(auth.ssh_private_key_path)
            if auth.password:
                args.append("--remote-git-auth-password")
                args.append(auth.password)
            if auth.username:
                args.append("--remote-git-auth-username")
                args.append(auth.username)

        if self._remote_env_vars is not None:
            for k in self._remote_env_vars:
                v = self._remote_env_vars[k]
                if isinstance(v, Secret):
                    args.append("--remote-env-secret")
                    args.append(f"{k}={v}")
                elif isinstance(v, str):
                    args.append("--remote-env")
                    args.append(f"{k}={v}")
                else:
                    raise AssertionError(f"unexpected env value {v} for key '{k}'")

        if self._remote_pre_run_commands is not None:
            for command in self._remote_pre_run_commands:
                args.append("--remote-pre-run-command")
                args.append(command)

        if self._remote_skip_install_dependencies:
            args.append("--remote-skip-install-dependencies")

        return args


def _is_inline_program(**kwargs) -> bool:
    for key in ["program", "project_name"]:
        if key not in kwargs or kwargs[key] is None:
            return False
    return True


StackInitializer = Callable[[str, Workspace], Stack]


def create_stack(
    stack_name: str,
    project_name: Optional[str] = None,
    program: Optional[PulumiFn] = None,
    work_dir: Optional[str] = None,
    opts: Optional[LocalWorkspaceOptions] = None,
) -> Stack:
    """
    Creates a Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program or the local
    Pulumi CLI program from the specified working dir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. The work_dir keyword argument is ignored (but see the note on the work_dir
    field of opts, below).

    If no project_settings option is specified, default project settings will be created on behalf of the user.
    Similarly, unless a `work_dir` option is specified, the working directory will default to a new temporary
    directory provided by the OS.

    Example of creating a stack with an inline program:

        create_stack('dev', project_name='my-app', program=myAppFn)

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided, and will override the work_dir field in
    opts. Keyword arguments other than work_dir and opts are ignored.

    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    Example of creating a stack with a local program:

        create_stack('dev', work_dir='myapp/')

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
    if _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.create, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def select_stack(
    stack_name: str,
    project_name: Optional[str] = None,
    program: Optional[PulumiFn] = None,
    work_dir: Optional[str] = None,
    opts: Optional[LocalWorkspaceOptions] = None,
) -> Stack:
    """
    Selects a Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program or the local
    Pulumi CLI program from the specified working dir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. The work_dir keyword argument is ignored (but see the note on the work_dir
    field of opts, below).

    If no project_settings option is specified, default project settings will be created on behalf of the user.
    Similarly, unless a `work_dir` option is specified, the working directory will default to a new temporary
    directory provided by the OS.

    Example of selecting a stack with an inline program:

        select_stack('dev', project_name='my-app', program=myAppFn)

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided, and will override the work_dir field in
    opts. Keyword arguments other than work_dir and opts are ignored.

    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    Example of selecting a stack with a local program:

        select_stack('dev', work_dir='myapp/')

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
    if _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.select, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def create_or_select_stack(
    stack_name: str,
    project_name: Optional[str] = None,
    program: Optional[PulumiFn] = None,
    work_dir: Optional[str] = None,
    opts: Optional[LocalWorkspaceOptions] = None,
) -> Stack:
    """
    Creates or selects an existing Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi
    program or the local Pulumi CLI program from the specified working dir.

    **Inline Programs**

    For inline programs, the program and project_name keyword arguments must be provided. This program is fully
    debuggable and runs in process. The work_dir keyword argument is ignored (but see the note on the work_dir
    field of opts, below).

    If no project_settings option is specified, default project settings will be created on behalf of the user.
    Similarly, unless a `work_dir` option is specified, the working directory will default to a new temporary
    directory provided by the OS.

    Example of selecting a stack with an inline program:

        create_or_select_stack('dev', project_name='my-app', program=myAppFn)

    **Local Programs**

    For local programs, the work_dir keyword argument must be provided, and will override the work_dir field in
    opts. Keyword arguments other than work_dir and opts are ignored.

    This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up any
    available Settings files (Pulumi.yaml, Pulumi.[stack].yaml).

    Example of creating or selecting a stack with a local program:

        create_or_select_stack('dev', work_dir='myapp/')

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
    if _is_local_program(**args):
        return _local_source_stack_helper(stack_name, work_dir, Stack.create_or_select, opts)  # type: ignore
    raise ValueError(f"unexpected args: {' '.join(args)}")


def _inline_source_stack_helper(
    stack_name: str,
    program: PulumiFn,
    project_name: str,
    init_fn: StackInitializer,
    opts: Optional[LocalWorkspaceOptions] = None,
):
    workspace_options = opts or LocalWorkspaceOptions()
    workspace_options.program = program

    if not workspace_options.project_settings:
        work_dir = workspace_options.work_dir
        if work_dir:
            try:
                # This attempts to load the project settings, and if it
                # succeeds, then discards them. This is ok because the
                # LocalWorkspace will load them when it needs to. This is simply
                # establishing whether there is an appropritate file in
                # `work_dir`
                _load_project_settings(work_dir)
            except FileNotFoundError:
                workspace_options.project_settings = default_project(project_name)
        else:
            workspace_options.project_settings = default_project(project_name)
    elif workspace_options.project_settings.main is None:
        workspace_options.project_settings.main = os.getcwd()

    ws = LocalWorkspace(**workspace_options.__dict__)
    return init_fn(stack_name, ws)


def _is_local_program(**kwargs) -> bool:
    return "work_dir" in kwargs and kwargs["work_dir"] is not None


def _local_source_stack_helper(
    stack_name: str,
    work_dir: str,
    init_fn: StackInitializer,
    opts: Optional[LocalWorkspaceOptions] = None,
):
    workspace_options = opts or LocalWorkspaceOptions()
    workspace_options.work_dir = work_dir

    ws = LocalWorkspace(**workspace_options.__dict__)
    return init_fn(stack_name, ws)


def default_project(project_name: str) -> ProjectSettings:
    return ProjectSettings(name=project_name, runtime="python", main=os.getcwd())


def get_stack_settings_name(name: str) -> str:
    parts = name.split("/")
    if len(parts) < 1:
        return name
    return parts[-1]


def _parse_and_validate_pulumi_version(
    min_version: VersionInfo, current_version: str, opt_out: bool
) -> Optional[VersionInfo]:
    """
    Parse and return a version. An error is raised if the version is not
    valid. If *current_version* is not a valid version but *opt_out* is true,
    *None* is returned.
    """
    try:
        version: Optional[VersionInfo] = VersionInfo.parse(current_version)
    except ValueError:
        version = None
    if opt_out:
        return version
    if version is None:
        raise InvalidVersionError(
            f"Could not parse the Pulumi CLI version. This is probably an internal error. "
            f"If you are sure you have the correct version, set {_SKIP_VERSION_CHECK_VAR}=true."
        )
    if min_version.major < version.major:
        raise InvalidVersionError(
            f"Major version mismatch. You are using Pulumi CLI version {version} with "
            f"Automation SDK v{min_version.major}. Please update the SDK."
        )
    if min_version.compare(version) == 1:
        raise InvalidVersionError(
            f"Minimum version requirement failed. The minimum CLI version requirement is "
            f"{min_version}, your current CLI version is {version}. "
            f"Please update the Pulumi CLI."
        )
    return version


def _load_project_settings(work_dir: str) -> ProjectSettings:
    for ext in _setting_extensions:
        project_path = os.path.join(work_dir, f"Pulumi{ext}")
        if not os.path.exists(project_path):
            continue
        with open(project_path, "r", encoding="utf-8") as file:
            settings = json.load(file) if ext == ".json" else yaml.safe_load(file)
            return ProjectSettings(**settings)
    raise FileNotFoundError(
        f"failed to find project settings file in workdir: {work_dir}"
    )
