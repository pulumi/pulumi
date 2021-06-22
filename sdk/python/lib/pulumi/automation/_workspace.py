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

from abc import ABC, abstractmethod
from datetime import datetime
from typing import (
    Callable,
    Mapping,
    Any,
    List,
    Optional
)

from ._stack_settings import StackSettings
from ._project_settings import ProjectSettings
from ._config import ConfigMap, ConfigValue
from ._output import OutputMap

PulumiFn = Callable[[], None]


class StackSummary:
    """A summary of the status of a given stack."""
    name: str
    current: bool
    update_in_progress: bool
    last_update: Optional[datetime]
    resource_count: Optional[int]
    url: Optional[str]

    def __init__(self,
                 name: str,
                 current: bool,
                 update_in_progress: bool = False,
                 last_update: Optional[datetime] = None,
                 resource_count: Optional[int] = None,
                 url: Optional[str] = None) -> None:
        self.name = name
        self.current = current
        self.update_in_progress = update_in_progress
        self.last_update = last_update
        self.resource_count = resource_count
        self.url = url


class WhoAmIResult:
    """The currently logged-in Pulumi identity."""
    user: str

    def __init__(self, user: str):
        self.user = user


class PluginInfo:
    name: str
    kind: str
    size: int
    last_used_time: datetime
    install_time: Optional[datetime]
    version: Optional[str]

    def __init__(self,
                 name: str,
                 kind: str,
                 size: int,
                 last_used_time: datetime,
                 install_time: Optional[datetime] = None,
                 version: Optional[str] = None) -> None:
        self.name = name
        self.kind = kind
        self.size = size
        self.install_time = install_time
        self.last_used = last_used_time
        self.version = version


class Deployment:
    version: Optional[int]
    deployment: Optional[Mapping[str, Any]]

    def __init__(self, version: Optional[int] = None, deployment: Optional[Mapping[str, Any]] = None) -> None:
        self.version = version
        self.deployment = deployment

    def __repr__(self):
        return f"Deployment(version={self.version!r}, deployment={self.deployment!r})"


class Workspace(ABC):
    """
    Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
    Workspaces are used to manage the execution environment, providing various utilities such as plugin
    installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    """

    work_dir: str
    """
    The working directory to run Pulumi CLI commands
    """

    pulumi_home: Optional[str]
    """
    The directory override for CLI metadata if set.
    This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
    """

    secrets_provider: Optional[str]
    """
    The secrets provider to use for encryption and decryption of stack secrets.
    See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
    """

    program: Optional[PulumiFn]
    """
    The inline program `PulumiFn` to be used for Preview/Update operations if any.
    If none is specified, the stack will refer to ProjectSettings for this information.
    """

    env_vars: Mapping[str, str] = {}
    """
    Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
    """

    pulumi_version: str
    """
    The version of the underlying Pulumi CLI/Engine.
    """

    @abstractmethod
    def project_settings(self) -> ProjectSettings:
        """
        Returns the settings object for the current project if any.

        :returns: ProjectSettings
        """

    @abstractmethod
    def save_project_settings(self, settings: ProjectSettings) -> None:
        """
        Overwrites the settings object in the current project.
        There can only be a single project per workspace. Fails is new project name does not match old.

        :param settings: The project settings to save.
        """

    @abstractmethod
    def stack_settings(self, stack_name: str) -> StackSettings:
        """
        Returns the settings object for the stack matching the specified stack name if any.

        :param stack_name: The name of the stack.
        :return: StackSettings
        """

    @abstractmethod
    def save_stack_settings(self, stack_name: str, settings: StackSettings) -> None:
        """
        Overwrites the settings object for the stack matching the specified stack name.

        :param stack_name: The name of the stack.
        :param settings: The stack settings to save.
        """

    @abstractmethod
    def serialize_args_for_op(self, stack_name: str) -> List[str]:
        """
        A hook to provide additional args to CLI commands before they are executed.
        Provided with stack name, returns a list of args to append to an invoked command ["--config=...", ]
        LocalWorkspace does not utilize this extensibility point.

        :param stack_name: The name of the stack.
        """

    @abstractmethod
    def post_command_callback(self, stack_name: str) -> None:
        """
        A hook executed after every command. Called with the stack name.
        An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
        LocalWorkspace does not utilize this extensibility point.

        :param stack_name: The name of the stack.
        """

    @abstractmethod
    def get_config(self, stack_name: str, key: str) -> ConfigValue:
        """
        Returns the value associated with the specified stack name and key,
        scoped to the Workspace.

        :param stack_name: The name of the stack.
        :param key: The key for the config item to get.
        :returns: ConfigValue
        """

    @abstractmethod
    def get_all_config(self, stack_name: str) -> ConfigMap:
        """
        Returns the config map for the specified stack name, scoped to the current Workspace.

        :param stack_name: The name of the stack.
        :returns: ConfigMap
        """

    @abstractmethod
    def set_config(self, stack_name: str, key: str, value: ConfigValue) -> None:
        """
        Sets the specified key-value pair on the provided stack name.

        :param stack_name: The name of the stack.
        :param key: The config key to add.
        :param value: The config value to add.
        """

    @abstractmethod
    def set_all_config(self, stack_name: str, config: ConfigMap) -> None:
        """
        Sets all values in the provided config map for the specified stack name.

        :param stack_name: The name of the stack.
        :param config: A mapping of key to ConfigValue to set to config.
        """

    @abstractmethod
    def remove_config(self, stack_name: str, key: str) -> None:
        """
        Removes the specified key-value pair on the provided stack name.

        :param stack_name: The name of the stack.
        :param key: The key to remove from config.
        """

    @abstractmethod
    def remove_all_config(self, stack_name: str, keys: List[str]) -> None:
        """
        Removes all values in the provided key list for the specified stack name.

        :param stack_name: The name of the stack.
        :param keys: The keys to remove from config.
        """

    @abstractmethod
    def refresh_config(self, stack_name: str) -> None:
        """
        Gets and sets the config map used with the last update for Stack matching stack name.

        :param stack_name: The name of the stack.
        """

    @abstractmethod
    def who_am_i(self) -> WhoAmIResult:
        """
        Returns the currently authenticated user.

        :returns: WhoAmIResult
        """

    @abstractmethod
    def stack(self) -> Optional[StackSummary]:
        """
        Returns a summary of the currently selected stack, if any.

        :returns: Optional[StackSummary]
        """

    @abstractmethod
    def create_stack(self, stack_name: str) -> None:
        """
        Creates and sets a new stack with the stack name, failing if one already exists.

        :param str stack_name: The name of the stack to create
        :returns: None
        :raises CommandError Raised if a stack with the same name exists.
        """

    @abstractmethod
    def select_stack(self, stack_name: str) -> None:
        """
        Selects and sets an existing stack matching the stack stack_name, failing if none exists.

        :param stack_name: The name of the stack to select
        :returns: None
        :raises CommandError Raised if no matching stack exists.
        """

    @abstractmethod
    def remove_stack(self, stack_name: str) -> None:
        """
        Deletes the stack and all associated configuration and history.

        :param stack_name: The name of the stack to remove
        """

    @abstractmethod
    def list_stacks(self) -> List[StackSummary]:
        """
        Returns all Stacks created under the current Project.
        This queries underlying backend and may return stacks not present in the Workspace
        (as Pulumi.<stack>.yaml files).

        :returns: List[StackSummary]
        """

    @abstractmethod
    def install_plugin(self, name: str, version: str, kind: str = "resource") -> None:
        """
        Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.

        :param name: The name of the plugin to install.
        :param version: The version to install.
        :param kind: The kind of plugin.
        """

    @abstractmethod
    def remove_plugin(self,
                      name: Optional[str] = None,
                      version_range: Optional[str] = None,
                      kind: str = "resource") -> None:
        """
        Removes a plugin from the Workspace matching the specified name and version.

        :param name: The name of the plugin to remove.
        :param version_range: The version range to remove.
        :param kind: The kind of plugin.
        """

    @abstractmethod
    def list_plugins(self) -> List[PluginInfo]:
        """
        Returns a list of all plugins installed in the Workspace.

        :returns: List[PluginInfo]
        """

    @abstractmethod
    def export_stack(self, stack_name: str) -> Deployment:
        """
        ExportStack exports the deployment state of the stack matching the given name.
        This can be combined with ImportStack to edit a stack's state (such as recovery from failed deployments).

        :param stack_name: The name of the stack to export.
        :returns: Deployment
        """

    @abstractmethod
    def import_stack(self, stack_name: str, state: Deployment) -> None:
        """
        ImportStack imports the specified deployment state into a pre-existing stack.
        This can be combined with ExportStack to edit a stack's state (such as recovery from failed deployments).

        :param stack_name: The name of the stack to import.
        :param state: The deployment state to import.
        """

    @abstractmethod
    def stack_outputs(self, stack_name: str) -> OutputMap:
        """
        Gets the current set of Stack outputs from the last Stack.up().

        :param stack_name: The name of the stack.
        :returns: OutputMap
        """
