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

import json
import grpc
from concurrent import futures
from enum import Enum
from datetime import datetime
from typing import List, Any, Mapping, MutableMapping, Optional

from .cmd import CommandResult, _run_pulumi_cmd, OnOutput
from .config import ConfigValue, ConfigMap, _SECRET_SENTINEL
from .errors import StackAlreadyExistsError
from .server import LanguageServer
from .workspace import Workspace, PulumiFn, Deployment
from ...runtime.settings import _GRPC_CHANNEL_OPTIONS
from ...runtime.proto import language_pb2_grpc

_DATETIME_FORMAT = '%Y-%m-%dT%H:%M:%S.%fZ'


class ExecKind(str, Enum):
    LOCAL = "auto.local"
    INLINE = "auto.inline"


class StackInitMode(Enum):
    CREATE = "create"
    SELECT = "select"
    CREATE_OR_SELECT = "create_or_select"


class OutputValue:
    value: Any
    secret: bool

    def __init__(self, value: Any, secret: bool):
        self.value = value
        self.secret = secret

    def __repr__(self):
        return _SECRET_SENTINEL if self.secret else repr(self.value)


OutputMap = MutableMapping[str, OutputValue]

OpMap = MutableMapping[str, int]


class UpdateSummary:
    # pre-update info
    kind: str
    start_time: datetime
    message: str
    environment: Mapping[str, str]
    config: ConfigMap

    # post-update info
    result: str
    end_time: datetime
    version: Optional[int]
    deployment: Optional[str]
    resource_changes: Optional[OpMap]

    def __init__(self,
                 kind: str,
                 start_time: datetime,
                 message: str,
                 environment: Mapping[str, str],
                 config: Mapping[str, dict],
                 result: str,
                 end_time: datetime,
                 version: Optional[int] = None,
                 deployment: Optional[str] = None,
                 resource_changes: Optional[OpMap] = None):
        self.kind = kind
        self.start_time = start_time
        self.end_time = end_time
        self.message = message
        self.environment = environment
        self.result = result
        self.Deployment = deployment
        self.resource_changes = resource_changes
        self.version = version
        self.config: ConfigMap = {}
        for key in config:
            self.config[key] = ConfigValue(**config[key])

    def __repr__(self):
        return f"UpdateSummary(result={self.result!r}, version={self.version!r}, " \
               f"start_time={self.start_time!r}, end_time={self.end_time!r}, kind={self.kind!r}, " \
               f"message={self.message!r}, environment={self.environment!r}, " \
               f"resource_changes={self.resource_changes!r}, config={self.config!r}, Deployment={self.Deployment!r})"


class BaseResult:
    stdout: str
    stderr: str
    summary: UpdateSummary

    def __init__(self, stdout: str, stderr: str, summary: UpdateSummary):
        self.stdout = stdout
        self.stderr = stderr
        self.summary = summary

    def __repr__(self):
        return f"{self.__class__.__name__}(summary={self.summary!r}, stdout={self.stdout!r}, stderr={self.stderr!r})"


class UpResult(BaseResult):
    outputs: OutputMap

    def __init__(self, stdout: str, stderr: str, summary: UpdateSummary, outputs: OutputMap):
        super().__init__(stdout, stderr, summary)
        self.outputs = outputs

    def __repr__(self):
        return f"{self.__class__.__name__}(outputs={self.outputs!r}, summary={self.summary!r}, " \
               f"stdout={self.stdout!r}, stderr={self.stderr!r})"


class PreviewResult(BaseResult):
    pass


class RefreshResult(BaseResult):
    pass


class DestroyResult(BaseResult):
    pass


class Stack:
    name: str
    """The name identifying the Stack."""

    workspace: Workspace
    """The Workspace the Stack was created from."""

    @classmethod
    def create(cls, stack_name: str, workspace: Workspace) -> 'Stack':
        """
        Creates a new stack using the given workspace, and stack name.
        It fails if a stack with that name already exists.

        :param stack_name: The name identifying the Stack
        :param workspace: The Workspace the Stack was created from.
        :return: Stack
        """
        return Stack(stack_name, workspace, StackInitMode.CREATE)

    @classmethod
    def select(cls, stack_name: str, workspace: Workspace) -> 'Stack':
        """
        Selects stack using the given workspace, and stack name.
        It returns an error if the given Stack does not exist. All LocalWorkspace operations will call `select` before
        running.

        :param stack_name: The name identifying the Stack
        :param workspace: The Workspace the Stack was created from.
        :return: Stack
        """
        return Stack(stack_name, workspace, StackInitMode.SELECT)

    @classmethod
    def create_or_select(cls, stack_name: str, workspace: Workspace) -> 'Stack':
        """
        Tries to create a new stack using the given workspace and stack name if the stack does not already exist,
        or falls back to selecting the existing stack. If the stack does not exist,
        it will be created and selected.

        :param stack_name: The name identifying the Stack
        :param workspace: The Workspace the Stack was created from.
        :return: Stack
        """
        return Stack(stack_name, workspace, StackInitMode.CREATE_OR_SELECT)

    def __init__(self, name: str, workspace: Workspace, mode: StackInitMode) -> None:
        self.name = name
        self.workspace = workspace
        self._mode = mode

        if not isinstance(name, str):
            raise TypeError("name must be of type 'str'")
        if not isinstance(workspace, Workspace):
            raise TypeError("workspace must be of type 'Workspace'")
        if not isinstance(mode, StackInitMode):
            raise TypeError("mode must be of type 'StackInitMode'")

        if mode is StackInitMode.CREATE:
            workspace.create_stack(name)
        elif mode is StackInitMode.SELECT:
            workspace.select_stack(name)
        elif mode is StackInitMode.CREATE_OR_SELECT:
            try:
                workspace.create_stack(name)
            except StackAlreadyExistsError:
                workspace.select_stack(name)

    def __repr__(self):
        return f"Stack(stack_name={self.name!r}, workspace={self.workspace!r}, mode={self._mode!r})"

    def __str__(self):
        return f"Stack(stack_name={self.name!r}, workspace={self.workspace!r})"

    def up(self,
           parallel: Optional[int] = None,
           message: Optional[str] = None,
           target: Optional[List[str]] = None,
           expect_no_changes: Optional[bool] = None,
           target_dependents: Optional[bool] = None,
           replace: Optional[List[str]] = None,
           on_output: Optional[OnOutput] = None,
           program: Optional[PulumiFn] = None) -> UpResult:
        """
        Creates or updates the resources in a stack by executing the program in the Workspace.
        https://www.pulumi.com/docs/reference/cli/pulumi_up/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the update operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param on_output: A function to process the stdout stream.
        :param program: The inline program.
        :returns: UpResult
        """
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["up", "--yes", "--skip-preview"]
        args.extend(extra_args)

        self.workspace.select_stack(self.name)
        kind = ExecKind.LOCAL.value
        on_exit = None

        if program:
            kind = ExecKind.INLINE.value
            server = grpc.server(futures.ThreadPoolExecutor(max_workers=4),
                                 options=_GRPC_CHANNEL_OPTIONS)
            language_server = LanguageServer(program)
            language_pb2_grpc.add_LanguageRuntimeServicer_to_server(language_server, server)

            port = server.add_insecure_port(address="0.0.0.0:0")
            server.start()

            def on_exit():
                language_server.on_pulumi_exit()
                server.stop(0)
            args.append(f"--client=127.0.0.1:{port}")

        args.extend(["--exec-kind", kind])

        try:
            up_result = self._run_pulumi_cmd_sync(args, on_output)
            outputs = self.outputs()
            summary = self.info()
            assert (summary is not None)
            return UpResult(stdout=up_result.stdout, stderr=up_result.stderr, summary=summary, outputs=outputs)
        finally:
            if on_exit is not None:
                on_exit()

    def preview(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                expect_no_changes: Optional[bool] = None,
                target_dependents: Optional[bool] = None,
                replace: Optional[List[str]] = None,
                program: Optional[PulumiFn] = None) -> PreviewResult:
        """
        Performs a dry-run update to a stack, returning pending changes.
        https://www.pulumi.com/docs/reference/cli/pulumi_preview/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message to associate with the preview operation.
        :param target: Specify an exclusive list of resource URNs to update.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param program: The inline program.
        :returns: PreviewResult
        """
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["preview"]
        args.extend(extra_args)

        self.workspace.select_stack(self.name)
        kind = ExecKind.LOCAL.value
        on_exit = None

        if program:
            kind = ExecKind.INLINE.value
            server = grpc.server(futures.ThreadPoolExecutor(max_workers=4),
                                 options=_GRPC_CHANNEL_OPTIONS)
            language_server = LanguageServer(program)
            language_pb2_grpc.add_LanguageRuntimeServicer_to_server(language_server, server)

            port = server.add_insecure_port(address="0.0.0.0:0")
            server.start()

            def on_exit():
                language_server.on_pulumi_exit()
                server.stop(0)
            args.append(f"--client=127.0.0.1:{port}")
        args.extend(["--exec-kind", kind])

        try:
            preview_result = self._run_pulumi_cmd_sync(args)
            summary = self.info()
            assert (summary is not None)
            return PreviewResult(stdout=preview_result.stdout, stderr=preview_result.stderr, summary=summary)
        finally:
            if on_exit is not None:
                on_exit()

    def refresh(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                expect_no_changes: Optional[bool] = None,
                on_output: Optional[OnOutput] = None) -> RefreshResult:
        """
        Compares the current stack’s resource state with the state known to exist in the actual
        cloud provider. Any such changes are adopted into the current stack.

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the refresh operation.
        :param target: Specify an exclusive list of resource URNs to refresh.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param on_output: A function to process the stdout stream.
        :returns: RefreshResult
        """
        extra_args = _parse_extra_args(**locals())
        args = ["refresh", "--yes", "--skip-preview"]
        args.extend(extra_args)

        self.workspace.select_stack(self.name)
        refresh_result = self._run_pulumi_cmd_sync(args, on_output)
        summary = self.info()
        assert(summary is not None)
        return RefreshResult(stdout=refresh_result.stdout, stderr=refresh_result.stderr, summary=summary)

    def destroy(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                target_dependents: Optional[bool] = None,
                on_output: Optional[OnOutput] = None) -> DestroyResult:
        """
        Destroy deletes all resources in a stack, leaving all history and configuration intact.

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the destroy operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param on_output: A function to process the stdout stream.
        :returns: DestroyResult
        """
        extra_args = _parse_extra_args(**locals())
        args = ["destroy", "--yes", "--skip-preview"]
        args.extend(extra_args)

        self.workspace.select_stack(self.name)
        destroy_result = self._run_pulumi_cmd_sync(args, on_output)
        summary = self.info()
        assert(summary is not None)
        return DestroyResult(stdout=destroy_result.stdout, stderr=destroy_result.stderr, summary=summary)

    def get_config(self, key: str) -> ConfigValue:
        """
        Returns the config value associated with the specified key.

        :param key: The key for the config item to get.
        :returns: ConfigValue
        """
        return self.workspace.get_config(self.name, key)

    def get_all_config(self) -> ConfigMap:
        """
        Returns the full config map associated with the stack in the Workspace.

        :returns: ConfigMap
        """
        return self.workspace.get_all_config(self.name)

    def set_config(self, key: str, value: ConfigValue) -> None:
        """
        Sets a config key-value pair on the Stack in the associated Workspace.

        :param key: The config key to add.
        :param value: The config value to add.
        """
        self.workspace.set_config(self.name, key, value)

    def set_all_config(self, config: ConfigMap) -> None:
        """
        Sets all specified config values on the stack in the associated Workspace.

        :param config: A mapping of key to ConfigValue to set to config.
        """
        self.workspace.set_all_config(self.name, config)

    def remove_config(self, key: str) -> None:
        """
        Removes the specified config key from the Stack in the associated Workspace.

        :param key: The key to remove from config.
        """
        self.workspace.remove_config(self.name, key)

    def remove_all_config(self, keys: List[str]) -> None:
        """
        Removes the specified config keys from the Stack in the associated Workspace.

        :param keys: The keys to remove from config.
        """
        self.workspace.remove_all_config(self.name, keys)

    def refresh_config(self) -> None:
        """Gets and sets the config map used with the last update."""
        self.workspace.refresh_config(self.name)

    def outputs(self) -> OutputMap:
        """
        Gets the current set of Stack outputs from the last Stack.up().

        :returns: OutputMap
        """
        self.workspace.select_stack(self.name)

        masked_result = self._run_pulumi_cmd_sync(["stack", "output", "--json"])
        plaintext_result = self._run_pulumi_cmd_sync(["stack", "output", "--json", "--show-secrets"])
        masked_outputs = json.loads(masked_result.stdout)
        plaintext_outputs = json.loads(plaintext_result.stdout)
        outputs: OutputMap = {}
        for key in plaintext_outputs:
            secret = masked_outputs[key] == _SECRET_SENTINEL
            outputs[key] = OutputValue(value=plaintext_outputs[key], secret=secret)
        return outputs

    def history(self) -> List[UpdateSummary]:
        """
        Returns a list summarizing all previous and current results from Stack lifecycle operations
        (up/preview/refresh/destroy).

        :returns: List[UpdateSummary]
        """
        result = self._run_pulumi_cmd_sync(["history", "--json", "--show-secrets"])
        summary_list = json.loads(result.stdout)

        summaries: List[UpdateSummary] = []
        for summary_json in summary_list:
            summary = UpdateSummary(kind=summary_json["kind"],
                                    start_time=datetime.strptime(summary_json["startTime"], _DATETIME_FORMAT),
                                    message=summary_json["message"],
                                    environment=summary_json["environment"],
                                    config=summary_json["config"],
                                    result=summary_json["result"],
                                    end_time=datetime.strptime(summary_json["endTime"], _DATETIME_FORMAT),
                                    version=summary_json["version"] if "version" in summary_json else None,
                                    deployment=summary_json["Deployment"] if "Deployment" in summary_json else None,
                                    resource_changes=summary_json["resourceChanges"] if "resourceChanges" in summary_json else None)
            summaries.append(summary)
        return summaries

    def info(self) -> Optional[UpdateSummary]:
        """
        Returns the current results from Stack lifecycle operations.

        :returns: Optional[UpdateSummary]
        """
        history = self.history()
        if not len(history):
            return None
        return history[0]

    def cancel(self) -> None:
        """
        Cancel stops a stack's currently running update. It returns an error if no update is currently running.
        Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
        if a resource operation was pending when the update was canceled.
        This command is not supported for local backends.
        """
        self.workspace.select_stack(self.name)
        self._run_pulumi_cmd_sync(["cancel", "--yes"])

    def export_stack(self) -> Deployment:
        """
        export_stack exports the deployment state of the stack.
        This can be combined with Stack.import_state to edit a stack's state (such as recovery from failed deployments).

        :returns: Deployment
        """
        return self.workspace.export_stack(self.name)

    def import_stack(self, state: Deployment) -> None:
        """
        import_stack imports the specified deployment state into a pre-existing stack.
        This can be combined with Stack.export_state to edit a stack's state (such as recovery from failed deployments).

        :param state: The deployment state to import.
        """
        return self.workspace.import_stack(self.name, state)

    def _run_pulumi_cmd_sync(self,
                             args: List[str],
                             on_output: Optional[OnOutput] = None) -> CommandResult:
        envs = {"PULUMI_HOME": self.workspace.pulumi_home} if self.workspace.pulumi_home else {}
        envs = {**envs, **self.workspace.env_vars}

        additional_args = self.workspace.serialize_args_for_op(self.name)
        args.extend(additional_args)

        result = _run_pulumi_cmd(args, self.workspace.work_dir, envs, on_output)
        self.workspace.post_command_callback(self.name)
        return result


def _parse_extra_args(**kwargs) -> List[str]:
    extra_args: List[str] = []

    if "message" in kwargs and kwargs["message"] is not None:
        extra_args.extend(["--message", kwargs["message"]])
    if "expect_no_changes" in kwargs and kwargs["expect_no_changes"] is not None:
        extra_args.append("--expect-no-changes")
    if "replace" in kwargs and kwargs["replace"] is not None:
        for r in kwargs["replace"]:
            extra_args.extend(["--replace", r])
    if "target" in kwargs and kwargs["target"] is not None:
        for t in kwargs["target"]:
            extra_args.extend(["--target", t])
    if "target_dependents" in kwargs and kwargs["target_dependents"] is not None:
        extra_args.append("--target-dependents")
    if "parallel" in kwargs and kwargs["parallel"] is not None:
        extra_args.extend(["--parallel", str(kwargs["parallel"])])
    return extra_args


def fully_qualified_stack_name(org: str, project: str, stack: str) -> str:
    """
    Returns a stack name formatted with the greatest possible specificity:
    org/project/stack or user/project/stack

    Using this format avoids ambiguity in stack identity guards creating or selecting the wrong stack.

    Note that filestate backends (local file, S3, Azure Blob) do not support stack names in this
    format, and instead only use the stack name without an org/user or project to qualify it.
    See: https://github.com/pulumi/pulumi/issues/2522

    :param org: The name of the org or user.
    :param project: The name of the project.
    :param stack: The name of the stack.
    :returns: The fully qualified stack name.
    """
    return f"{org}/{project}/{stack}"
