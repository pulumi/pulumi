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
import os
import tempfile
import time
import threading
from concurrent import futures
from enum import Enum
from datetime import datetime
from typing import List, Any, Mapping, MutableMapping, Optional, Callable, Tuple
import grpc

from ._cmd import CommandResult, _run_pulumi_cmd, OnOutput
from ._config import ConfigValue, ConfigMap, _SECRET_SENTINEL
from .errors import StackAlreadyExistsError
from .events import OpMap, EngineEvent, SummaryEvent
from ._server import LanguageServer
from ._workspace import Workspace, PulumiFn, Deployment
from ..runtime.settings import _GRPC_CHANNEL_OPTIONS
from ..runtime.proto import language_pb2_grpc

_DATETIME_FORMAT = '%Y-%m-%dT%H:%M:%S.%fZ'

OnEvent = Callable[[EngineEvent], Any]


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


class UpdateSummary:
    def __init__(self,
                 # pre-update info
                 kind: str,
                 start_time: datetime,
                 message: str,
                 environment: Mapping[str, str],
                 config: Mapping[str, dict],
                 # post-update info
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
            config_value = config[key]
            self.config[key] = ConfigValue(value=config_value["value"], secret=config_value["secret"])

    def __repr__(self):
        return f"UpdateSummary(result={self.result!r}, version={self.version!r}, " \
               f"start_time={self.start_time!r}, end_time={self.end_time!r}, kind={self.kind!r}, " \
               f"message={self.message!r}, environment={self.environment!r}, " \
               f"resource_changes={self.resource_changes!r}, config={self.config!r}, Deployment={self.Deployment!r})"


class BaseResult:
    def __init__(self, stdout: str, stderr: str):
        self.stdout = stdout
        self.stderr = stderr

    def __repr__(self):
        inputs = self.__dict__
        fields = [f"{key}={inputs[key]!r}" for key in inputs]
        fields = ", ".join(fields)
        return f"{self.__class__.__name__}({fields})"


class PreviewResult(BaseResult):
    def __init__(self, stdout: str, stderr: str, change_summary: OpMap):
        super().__init__(stdout, stderr)
        self.change_summary = change_summary


class UpResult(BaseResult):
    def __init__(self, stdout: str, stderr: str, summary: UpdateSummary, outputs: OutputMap):
        super().__init__(stdout, stderr)
        self.outputs = outputs
        self.summary = summary


class RefreshResult(BaseResult):
    def __init__(self, stdout: str, stderr: str, summary: UpdateSummary):
        super().__init__(stdout, stderr)
        self.summary = summary


class DestroyResult(BaseResult):
    def __init__(self, stdout: str, stderr: str, summary: UpdateSummary):
        super().__init__(stdout, stderr)
        self.summary = summary


class Stack:
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
        It returns an error if the given Stack does not exist.

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
        """
        Stack is an isolated, independently configurable instance of a Pulumi program.
        Stack exposes methods for the full pulumi lifecycle (up/preview/refresh/destroy), as well as managing configuration.
        Multiple Stacks are commonly used to denote different phases of development
        (such as development, staging and production) or feature branches (such as feature-x-dev, jane-feature-x-dev).
        """
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
           diff: Optional[bool] = None,
           target_dependents: Optional[bool] = None,
           replace: Optional[List[str]] = None,
           on_output: Optional[OnOutput] = None,
           on_event: Optional[OnEvent] = None,
           program: Optional[PulumiFn] = None) -> UpResult:
        """
        Creates or updates the resources in a stack by executing the program in the Workspace.
        https://www.pulumi.com/docs/reference/cli/pulumi_up/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the update operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param diff: Display operation as a rich diff showing the overall change.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :param program: The inline program.
        :returns: UpResult
        """
        # Disable unused-argument because pylint doesn't understand we process them in _parse_extra_args
        # pylint: disable=unused-argument
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["up", "--yes", "--skip-preview"]
        args.extend(extra_args)

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

            def on_exit_fn():
                language_server.on_pulumi_exit()
                server.stop(0)
            on_exit = on_exit_fn

            args.append(f"--client=127.0.0.1:{port}")

        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("up")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(target=_watch_logs, args=(log_file, on_event))
            log_watcher_thread.start()

        try:
            up_result = self._run_pulumi_cmd_sync(args, on_output)
            outputs = self.outputs()
            summary = self.info()
            assert summary is not None
        finally:
            _cleanup(temp_dir, log_watcher_thread, on_exit)

        return UpResult(stdout=up_result.stdout, stderr=up_result.stderr, summary=summary, outputs=outputs)

    def preview(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                expect_no_changes: Optional[bool] = None,
                diff: Optional[bool] = None,
                target_dependents: Optional[bool] = None,
                replace: Optional[List[str]] = None,
                on_output: Optional[OnOutput] = None,
                on_event: Optional[OnEvent] = None,
                program: Optional[PulumiFn] = None) -> PreviewResult:
        """
        Performs a dry-run update to a stack, returning pending changes.
        https://www.pulumi.com/docs/reference/cli/pulumi_preview/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message to associate with the preview operation.
        :param target: Specify an exclusive list of resource URNs to update.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param diff: Display operation as a rich diff showing the overall change.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :param program: The inline program.
        :returns: PreviewResult
        """
        # Disable unused-argument because pylint doesn't understand we process them in _parse_extra_args
        # pylint: disable=unused-argument
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["preview"]
        args.extend(extra_args)

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

            def on_exit_fn():
                language_server.on_pulumi_exit()
                server.stop(0)
            on_exit = on_exit_fn

            args.append(f"--client=127.0.0.1:{port}")
        args.extend(["--exec-kind", kind])

        log_file, temp_dir = _create_log_file("preview")
        args.extend(["--event-log", log_file])
        summary_events: List[SummaryEvent] = []

        def on_event_callback(event: EngineEvent) -> None:
            if event.summary_event:
                summary_events.append(event.summary_event)
            if on_event:
                on_event(event)

        # Start watching logs in a thread
        log_watcher_thread = threading.Thread(target=_watch_logs, args=(log_file, on_event_callback))
        log_watcher_thread.start()

        try:
            preview_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread, on_exit)

        if not summary_events:
            raise RuntimeError("summary event never found")

        return PreviewResult(stdout=preview_result.stdout,
                             stderr=preview_result.stderr,
                             change_summary=summary_events[0].resource_changes)

    def refresh(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                expect_no_changes: Optional[bool] = None,
                on_output: Optional[OnOutput] = None,
                on_event: Optional[OnEvent] = None) -> RefreshResult:
        """
        Compares the current stack’s resource state with the state known to exist in the actual
        cloud provider. Any such changes are adopted into the current stack.

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the refresh operation.
        :param target: Specify an exclusive list of resource URNs to refresh.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: RefreshResult
        """
        # Disable unused-argument because pylint doesn't understand we process them in _parse_extra_args
        # pylint: disable=unused-argument
        extra_args = _parse_extra_args(**locals())
        args = ["refresh", "--yes", "--skip-preview"]
        args.extend(extra_args)

        kind = ExecKind.INLINE.value if self.workspace.program else ExecKind.LOCAL.value
        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("refresh")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(target=_watch_logs, args=(log_file, on_event))
            log_watcher_thread.start()

        try:
            refresh_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread)

        summary = self.info()
        assert summary is not None
        return RefreshResult(stdout=refresh_result.stdout, stderr=refresh_result.stderr, summary=summary)

    def destroy(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                target_dependents: Optional[bool] = None,
                on_output: Optional[OnOutput] = None,
                on_event: Optional[OnEvent] = None) -> DestroyResult:
        """
        Destroy deletes all resources in a stack, leaving all history and configuration intact.

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the destroy operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: DestroyResult
        """
        # Disable unused-argument because pylint doesn't understand we process them in _parse_extra_args
        # pylint: disable=unused-argument
        extra_args = _parse_extra_args(**locals())
        args = ["destroy", "--yes", "--skip-preview"]
        args.extend(extra_args)

        kind = ExecKind.INLINE.value if self.workspace.program else ExecKind.LOCAL.value
        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("destroy")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(target=_watch_logs, args=(log_file, on_event))
            log_watcher_thread.start()

        try:
            destroy_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread)

        summary = self.info()
        assert summary is not None
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
        masked_result = self._run_pulumi_cmd_sync(["stack", "output", "--json"])
        plaintext_result = self._run_pulumi_cmd_sync(["stack", "output", "--json", "--show-secrets"])
        masked_outputs = json.loads(masked_result.stdout)
        plaintext_outputs = json.loads(plaintext_result.stdout)
        outputs: OutputMap = {}
        for key in plaintext_outputs:
            secret = masked_outputs[key] == _SECRET_SENTINEL
            outputs[key] = OutputValue(value=plaintext_outputs[key], secret=secret)
        return outputs

    def history(self,
                page_size: Optional[int] = None,
                page: Optional[int] = None) -> List[UpdateSummary]:
        """
        Returns a list summarizing all previous and current results from Stack lifecycle operations
        (up/preview/refresh/destroy).

        :param page_size: Paginate history entries (used in combination with page), defaults to all.
        :param page: Paginate history entries (used in combination with page_size), defaults to all.

        :returns: List[UpdateSummary]
        """
        args = ["stack", "history", "--json", "--show-secrets"]
        if page_size is not None:
            # default page=1 when page_size is set
            if page is None:
                page = 1
            args.extend(["--page-size", str(page_size), "--page", str(page)])
        result = self._run_pulumi_cmd_sync(args)
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
        history = self.history(page_size=1)
        if not history:
            return None
        return history[0]

    def cancel(self) -> None:
        """
        Cancel stops a stack's currently running update. It returns an error if no update is currently running.
        Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
        if a resource operation was pending when the update was canceled.
        This command is not supported for local backends.
        """
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
        envs = {"PULUMI_DEBUG_COMMANDS": "true"}
        if self.workspace.pulumi_home is not None:
            envs = {**envs, "PULUMI_HOME": self.workspace.pulumi_home}
        envs = {**envs, **self.workspace.env_vars}

        additional_args = self.workspace.serialize_args_for_op(self.name)
        args.extend(additional_args)
        args.extend(["--stack", self.name])
        result = _run_pulumi_cmd(args, self.workspace.work_dir, envs, on_output)
        self.workspace.post_command_callback(self.name)
        return result


def _parse_extra_args(**kwargs) -> List[str]:
    extra_args: List[str] = []

    message = kwargs.get("message")
    expect_no_changes = kwargs.get("expect_no_changes")
    diff = kwargs.get("diff")
    replace = kwargs.get("replace")
    target = kwargs.get("target")
    target_dependents = kwargs.get("target_dependents")
    parallel = kwargs.get("parallel")

    if message:
        extra_args.extend(["--message", message])
    if expect_no_changes:
        extra_args.append("--expect-no-changes")
    if diff:
        extra_args.append("--diff")
    if replace:
        for r in replace:
            extra_args.extend(["--replace", r])
    if target:
        for t in target:
            extra_args.extend(["--target", t])
    if target_dependents:
        extra_args.append("--target-dependents")
    if parallel:
        extra_args.extend(["--parallel", str(parallel)])
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


def _create_log_file(command: str) -> Tuple[str, tempfile.TemporaryDirectory]:
    log_dir = tempfile.TemporaryDirectory(prefix=f"automation-logs-{command}-")
    filepath = os.path.join(log_dir.name, "eventlog.txt")

    # Open and close the file to ensure it exists before we start polling for logs
    f = open(filepath, "w+")
    f.close()
    return filepath, log_dir


def _watch_logs(filename: str, callback: OnEvent):
    with open(filename) as f:
        while True:
            line = f.readline()

            # sleep if file hasn't been updated
            if not line:
                time.sleep(0.1)
                continue

            event = EngineEvent.from_json(json.loads(line))
            callback(event)

            # if this is the cancel event, stop watching logs.
            if event.cancel_event:
                break


def _cleanup(temp_dir: Optional[tempfile.TemporaryDirectory],
             thread: Optional[threading.Thread],
             on_exit_fn: Optional[Callable[[], None]] = None) -> None:
    # If there's an on_exit function, execute it (used in preview/up to shut down server)
    if on_exit_fn:
        on_exit_fn()
    # If we started a thread to watch logs, wait for it to terminate, timing out after 5 seconds.
    if thread:
        thread.join(5)
    # If we created a temp_dir for the logs, clean up.
    if temp_dir:
        temp_dir.cleanup()
