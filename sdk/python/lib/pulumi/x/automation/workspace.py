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

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime
from typing import (
    Callable,
    Awaitable,
    Mapping,
    Any,
    List,
    Optional,
    Literal
)
from .stack_settings import StackSettings
from .project_settings import ProjectSettings
from .config import ConfigMap, ConfigValue


PluginKind = Literal["analyzer", "language", "resource"]


@dataclass
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
                 updateInProgress: bool = False,
                 lastUpdate: Optional[str] = None,
                 resourceCount: Optional[int] = None,
                 url: Optional[str] = None) -> None:
        self.name = name
        self.current = current
        self.update_in_progress = updateInProgress
        self.last_update = datetime.strptime(lastUpdate[:-5], "%Y-%m-%dT%H:%M:%S") if lastUpdate else None
        self.resource_count = resourceCount
        self.url = url


@dataclass
class WhoAmIResult:
    """The currently logged-in Pulumi identity."""
    user: str


@dataclass
class PluginInfo:
    name: str
    kind: PluginKind
    size: int
    last_used: datetime
    install_time: Optional[datetime]
    version: Optional[str]

    def __init__(self,
                 name: str,
                 kind: PluginKind,
                 size: int,
                 lastUsedTime: str,
                 installTime: Optional[str] = None,
                 version: Optional[str] = None) -> None:
        self.name = name
        self.kind = kind
        self.size = size
        self.install_time = datetime.strptime(installTime[:-5], "%Y-%m-%dT%H:%M:%S") if installTime else None
        self.last_used = datetime.strptime(lastUsedTime[:-5], "%Y-%m-%dT%H:%M:%S")
        self.version = version


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

    # TODO improve typing to encapsulate stack exports
    program: Optional[Callable[[], Any]]
    """
    The inline program `PulumiFn` to be used for Preview/Update operations if any.
    If none is specified, the stack will refer to ProjectSettings for this information.
    """

    env_vars: Mapping[str, str] = {}
    """
    Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
    """

    @abstractmethod
    def project_settings(self) -> ProjectSettings:
        """
        Returns the settings object for the current project if any.

        :return: ProjectSettings
        """
        pass

    @abstractmethod
    def save_project_settings(self, settings: ProjectSettings) -> None:
        """
        Overwrites the settings object in the current project.
        There can only be a single project per workspace. Fails is new project name does not match old.

        :param settings:
        :return:
        """
        pass

    @abstractmethod
    def stack_settings(self, stack_name: str) -> StackSettings:
        """
        Returns the settings object for the stack matching the specified stack name if any.

        :param stack_name:
        :return:
        """
        pass

    @abstractmethod
    def save_stack_settings(self, stack_name: str, settings: StackSettings) -> None:
        """
        Overwrites the settings object for the stack matching the specified stack name.

        :param stack_name:
        :param settings:
        :return:
        """
        pass

    @abstractmethod
    async def serialize_args_for_op(self, stack_name: str) -> None:
        """
        A hook to provide additional args to CLI commands before they are executed.
        Provided with stack name, returns a list of args to append to an invoked command ["--config=...", ]
        LocalWorkspace does not utilize this extensibility point.

        :param stack_name:
        :return:
        """
        pass

    @abstractmethod
    async def post_command_callback(self, stack_name: str) -> None:
        """
        A hook executed after every command. Called with the stack name.
        An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
        LocalWorkspace does not utilize this extensibility point.

        :param stack_name:
        :return:
        """
        pass

    @abstractmethod
    def get_config(self, stack_name: str, key: str) -> ConfigValue:
        """
        Returns the value associated with the specified stack name and key,
        scoped to the Workspace.

        :param stack_name:
        :param key:
        :return:
        """
        pass

    @abstractmethod
    def get_all_config(self, stack_name: str) -> ConfigMap:
        """
        Returns the config map for the specified stack name, scoped to the current Workspace.

        :param stack_name:
        :return:
        """
        pass

    @abstractmethod
    def set_config(self, stack_name: str, key: str, value: ConfigValue) -> None:
        """
        Sets the specified key-value pair on the provided stack name.

        :param stack_name:
        :param key:
        :param value:
        :return:
        """
        pass

    @abstractmethod
    def set_all_config(self, stack_name: str, config: ConfigMap) -> None:
        """
        Sets all values in the provided config map for the specified stack name.

        :param stack_name:
        :param config:
        :return:
        """
        pass

    @abstractmethod
    def remove_config(self, stack_name: str, key: str) -> None:
        """
        Removes the specified key-value pair on the provided stack name.

        :param stack_name:
        :param key:
        :return:
        """
        pass

    @abstractmethod
    def remove_all_config(self, stack_name: str, keys: List[str]) -> None:
        """
        Removes all values in the provided key list for the specified stack name.

        :param stack_name:
        :param keys:
        :return:
        """
        pass

    @abstractmethod
    def refresh_config(self, stack_name: str) -> None:
        """
        Gets and sets the config map used with the last update for Stack matching stack name.

        :param stack_name:
        :return:
        """
        pass

    @abstractmethod
    def who_am_i(self) -> WhoAmIResult:
        """
        Returns the currently authenticated user.

        :return:
        """
        pass

    @abstractmethod
    def stack(self) -> Optional[StackSummary]:
        """
        Returns a summary of the currently selected stack, if any.

        :return:
        """
        pass

    @abstractmethod
    def create_stack(self, stack_name: str) -> None:
        """
        Creates and sets a new stack with the stack name, failing if one already exists.

        :param str stack_name: The name of the stack to create
        :return: None
        :raises CommandError Raised if a stack with the same name exists.
        """
        pass

    @abstractmethod
    def select_stack(self, stack_name: str) -> None:
        """
        Selects and sets an existing stack matching the stack stack_name, failing if none exists.

        :param str stack_name: The name of the stack to select
        :return: None
        :raises CommandError Raised if no matching stack exists.
        """
        pass

    @abstractmethod
    def remove_stack(self, stack_name: str) -> None:
        """
        Deletes the stack and all associated configuration and history.

        :param str stack_name: The name of the stack to remove
        :return: None
        """
        pass

    @abstractmethod
    def list_stacks(self) -> List[StackSummary]:
        """
        Returns all Stacks created under the current Project.
        This queries underlying backend and may return stacks not present in the Workspace
        (as Pulumi.<stack>.yaml files).

        """
        pass

    @abstractmethod
    def install_plugin(self, plugin_name: str, version: str, kind: str) -> None:
        """
        Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.

        """
        pass

    @abstractmethod
    def remove_plugin(self, plugin_name: Optional[str], version_range: Optional[str], kind: str) -> None:
        """
        Removes a plugin from the Workspace matching the specified name and version.

        """
        pass

    @abstractmethod
    def list_plugins(self) -> List[PluginInfo]:
        """
        Returns a list of all plugins installed in the Workspace.

        """
        pass
