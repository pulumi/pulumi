# Copyright 2016-2022, Pulumi Corporation.
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
from typing import (
    Dict,
    List,
    Any,
    Mapping,
    Optional,
    Callable,
    Tuple,
    TypedDict,
)
import grpc

from ._cmd import CommandResult, OnOutput
from ._config import ConfigValue, ConfigMap
from .errors import StackNotFoundError
from .events import OpMap, EngineEvent, SummaryEvent
from ._output import OutputMap
from ._server import LanguageServer
from ._workspace import Workspace, PulumiFn, Deployment
from ..runtime.settings import _GRPC_CHANNEL_OPTIONS
from ..runtime.proto import language_pb2_grpc
from ._representable import _Representable
from ._tag import TagMap

_DATETIME_FORMAT = "%Y-%m-%dT%H:%M:%S.%fZ"

OnEvent = Callable[[EngineEvent], Any]


class ExecKind(str, Enum):
    LOCAL = "auto.local"
    INLINE = "auto.inline"


class StackInitMode(Enum):
    CREATE = "create"
    SELECT = "select"
    CREATE_OR_SELECT = "create_or_select"


class UpdateSummary:
    def __init__(
        self,
        # pre-update info
        kind: str,
        start_time: datetime,
        message: str,
        environment: Mapping[str, str],
        config: Mapping[str, dict],
        # post-update info
        result: str,
        end_time: Optional[datetime] = None,
        version: Optional[int] = None,
        deployment: Optional[str] = None,
        resource_changes: Optional[OpMap] = None,
    ):
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
            secret = config_value["secret"]
            # If it is a secret, and we're not showing secrets, the value is excluded from the JSON results.
            # In that case, we'll just use the sentinal `[secret]` value. Otherwise, we expect to get a value.
            value = (
                config_value.get("value", "[secret]")
                if secret
                else config_value["value"]
            )
            self.config[key] = ConfigValue(value=value, secret=secret)

    def __repr__(self):
        return (
            f"UpdateSummary(result={self.result!r}, version={self.version!r}, "
            f"start_time={self.start_time!r}, end_time={self.end_time!r}, kind={self.kind!r}, "
            f"message={self.message!r}, environment={self.environment!r}, "
            f"resource_changes={self.resource_changes!r}, config={self.config!r}, Deployment={self.Deployment!r})"
        )


class ImportResource(TypedDict, total=False):
    """
    ImportResource represents a resource to import into a stack.

      - id: The import ID of the resource. The format is specific to resource type.
      - type: The type token of the Pulumi resource
      - name: The name of the resource
      - logicalName: The logical name of the resource in the generated program
      - parent: The name of an optional parent resource
      - provider: The name of the provider resource
      - version: The version of the provider plugin, if any is specified
      - pluginDownloadUrl: The URL to download the provider plugin from
      - properties: Specified which input properties to import with
      - component: Whether the resource is a component resource
      - remote: Whether the resource is a remote resource

    If a resource does not specify any properties the default behaviour is to
    import using all required properties.

    If the resource is declared as a "component" (and optionally as "remote"). These resources
    don't have an id set and instead just create an empty placeholder component resource in the Pulumi state.
    """

    id: str
    type: str
    name: str
    logicalName: str
    parent: str
    provider: str
    version: str
    pluginDownloadUrl: str
    properties: str
    component: bool
    remote: bool


class BaseResult(_Representable):
    def __init__(self, stdout: str, stderr: str):
        self.stdout = stdout
        self.stderr = stderr


class PreviewResult(BaseResult):
    def __init__(self, stdout: str, stderr: str, change_summary: OpMap):
        super().__init__(stdout, stderr)
        self.change_summary = change_summary


class UpResult(BaseResult):
    def __init__(
        self, stdout: str, stderr: str, summary: UpdateSummary, outputs: OutputMap
    ):
        super().__init__(stdout, stderr)
        self.outputs = outputs
        self.summary = summary


class ImportResult(BaseResult):
    def __init__(
        self, stdout: str, stderr: str, summary: UpdateSummary, generated_code: str
    ):
        super().__init__(stdout, stderr)
        self.summary = summary
        self.generated_code = generated_code


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
    def create(cls, stack_name: str, workspace: Workspace) -> "Stack":
        """
        Creates a new stack using the given workspace, and stack name.
        It fails if a stack with that name already exists.

        :param stack_name: The name identifying the Stack
        :param workspace: The Workspace the Stack was created from.
        :return: Stack
        """
        return Stack(stack_name, workspace, StackInitMode.CREATE)

    @classmethod
    def select(cls, stack_name: str, workspace: Workspace) -> "Stack":
        """
        Selects stack using the given workspace, and stack name.
        It returns an error if the given Stack does not exist.

        :param stack_name: The name identifying the Stack
        :param workspace: The Workspace the Stack was created from.
        :return: Stack
        """
        return Stack(stack_name, workspace, StackInitMode.SELECT)

    @classmethod
    def create_or_select(cls, stack_name: str, workspace: Workspace) -> "Stack":
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
                workspace.select_stack(name)
            except StackNotFoundError:
                workspace.create_stack(name)

    def __repr__(self):
        return f"Stack(stack_name={self.name!r}, workspace={self.workspace!r}, mode={self._mode!r})"

    def __str__(self):
        return f"Stack(stack_name={self.name!r}, workspace={self.workspace!r})"

    def up(
        self,
        parallel: Optional[int] = None,
        message: Optional[str] = None,
        target: Optional[List[str]] = None,
        policy_packs: Optional[List[str]] = None,
        policy_pack_configs: Optional[List[str]] = None,
        expect_no_changes: Optional[bool] = None,
        diff: Optional[bool] = None,
        target_dependents: Optional[bool] = None,
        replace: Optional[List[str]] = None,
        color: Optional[str] = None,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
        program: Optional[PulumiFn] = None,
        plan: Optional[str] = None,
        show_secrets: bool = True,
        log_flow: Optional[bool] = None,
        log_verbosity: Optional[int] = None,
        log_to_std_err: Optional[bool] = None,
        tracing: Optional[str] = None,
        debug: Optional[bool] = None,
        suppress_outputs: Optional[bool] = None,
        suppress_progress: Optional[bool] = None,
        continue_on_error: Optional[bool] = None,
        attach_debugger: Optional[bool] = None,
        refresh: Optional[bool] = None,
    ) -> UpResult:
        """
        Creates or updates the resources in a stack by executing the program in the Workspace.
        https://www.pulumi.com/docs/cli/commands/pulumi_up/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the update operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param policy_packs: Run one or more policy packs as part of this update.
        :param policy_pack_configs: Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag.
        :param diff: Display operation as a rich diff showing the overall change.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :param program: The inline program.
        :param color: Colorize output. Choices are: always, never, raw, auto (default "auto")
        :param plan: Plan specifies the path to an update plan to use for the update.
        :param show_secrets: Include config secrets in the UpResult summary.
        :param log_flow: Flow log settings to child processes (like plugins)
        :param log_verbosity: Enable verbose logging (e.g., v=3); anything >3 is very verbose
        :param log_to_std_err: Log to stderr instead of to files
        :param tracing: Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
        :param debug: Print detailed debugging output during resource operations
        :param suppress_outputs: Suppress display of stack outputs (in case they contain sensitive values)
        :param suppress_progress: Suppress display of periodic progress dots
        :param continue_on_error: Continue to perform the update operation despite the occurrence of errors
        :param attach_debugger: Run the process under a debugger, and pause until a debugger is attached
        :param refresh: Refresh the state of the stack's resources against the cloud provider before running up.
        :returns: UpResult
        """
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["up", "--yes", "--skip-preview"]
        args.extend(extra_args)

        if plan is not None:
            args.append("--plan")
            args.append(plan)

        args.extend(self._remote_args())

        kind = ExecKind.LOCAL.value
        on_exit = None

        if program:
            kind = ExecKind.INLINE.value
            server = grpc.server(
                futures.ThreadPoolExecutor(max_workers=4),
                options=_GRPC_CHANNEL_OPTIONS,
            )
            language_server = LanguageServer(program)
            language_pb2_grpc.add_LanguageRuntimeServicer_to_server(
                language_server, server
            )

            port = server.add_insecure_port(address="127.0.0.1:0")
            server.start()

            def on_exit_fn():
                server.stop(0)

            on_exit = on_exit_fn

            args.append(f"--client=127.0.0.1:{port}")

        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("up")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(
                target=_watch_logs, args=(log_file, on_event)
            )
            log_watcher_thread.start()

        try:
            up_result = self._run_pulumi_cmd_sync(args, on_output)
            outputs = self.outputs()
            # If it's a remote workspace, explicitly set show_secrets to False to prevent attempting to
            # load the project file.
            summary = self.info(show_secrets and not self._remote)
            assert summary is not None
        finally:
            _cleanup(temp_dir, log_watcher_thread, on_exit)

        return UpResult(
            stdout=up_result.stdout,
            stderr=up_result.stderr,
            summary=summary,
            outputs=outputs,
        )

    def preview(
        self,
        parallel: Optional[int] = None,
        message: Optional[str] = None,
        target: Optional[List[str]] = None,
        policy_packs: Optional[List[str]] = None,
        policy_pack_configs: Optional[List[str]] = None,
        expect_no_changes: Optional[bool] = None,
        diff: Optional[bool] = None,
        target_dependents: Optional[bool] = None,
        replace: Optional[List[str]] = None,
        color: Optional[str] = None,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
        program: Optional[PulumiFn] = None,
        plan: Optional[str] = None,
        log_flow: Optional[bool] = None,
        log_verbosity: Optional[int] = None,
        log_to_std_err: Optional[bool] = None,
        tracing: Optional[str] = None,
        debug: Optional[bool] = None,
        suppress_outputs: Optional[bool] = None,
        suppress_progress: Optional[bool] = None,
        import_file: Optional[str] = None,
        attach_debugger: Optional[bool] = None,
        refresh: Optional[bool] = None,
    ) -> PreviewResult:
        """
        Performs a dry-run update to a stack, returning pending changes.
        https://www.pulumi.com/docs/cli/commands/pulumi_preview/

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message to associate with the preview operation.
        :param target: Specify an exclusive list of resource URNs to update.
        :param policy_packs: Run one or more policy packs as part of this update.
        :param policy_pack_configs: Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag.
        :param expect_no_changes: Return an error if any changes occur during this update.
        :param diff: Display operation as a rich diff showing the overall change.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param replace: Specify resources to replace.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :param program: The inline program.
        :param color: Colorize output. Choices are: always, never, raw, auto (default "auto")
        :param plan: Plan specifies the path where the update plan should be saved.
        :param log_flow: Flow log settings to child processes (like plugins)
        :param log_verbosity: Enable verbose logging (e.g., v=3); anything >3 is very verbose
        :param log_to_std_err: Log to stderr instead of to files
        :param tracing: Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
        :param debug: Print detailed debugging output during resource operations
        :param suppress_outputs: Suppress display of stack outputs (in case they contain sensitive values)
        :param suppress_progress: Suppress display of periodic progress dots
        :param import_file: Save any creates seen during the preview into an import file to use with pulumi import
        :param attach_debugger: Run the process under a debugger, and pause until a debugger is attached
        :param refresh: Refresh the state of the stack's resources against the cloud provider before running preview.
        :returns: PreviewResult
        """
        program = program or self.workspace.program
        extra_args = _parse_extra_args(**locals())
        args = ["preview"]
        args.extend(extra_args)

        if import_file is not None:
            args.append("--import-file")
            args.append(import_file)

        if plan is not None:
            args.append("--save-plan")
            args.append(plan)

        args.extend(self._remote_args())

        kind = ExecKind.LOCAL.value
        on_exit = None

        if program:
            kind = ExecKind.INLINE.value
            server = grpc.server(
                futures.ThreadPoolExecutor(max_workers=4),
                options=_GRPC_CHANNEL_OPTIONS,
            )
            language_server = LanguageServer(program)
            language_pb2_grpc.add_LanguageRuntimeServicer_to_server(
                language_server, server
            )

            port = server.add_insecure_port(address="127.0.0.1:0")
            server.start()

            def on_exit_fn():
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
        log_watcher_thread = threading.Thread(
            target=_watch_logs, args=(log_file, on_event_callback)
        )
        log_watcher_thread.start()

        try:
            preview_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread, on_exit)

        if not summary_events:
            raise RuntimeError("summary event never found")

        return PreviewResult(
            stdout=preview_result.stdout,
            stderr=preview_result.stderr,
            change_summary=summary_events[0].resource_changes,
        )

    def refresh(
        self,
        parallel: Optional[int] = None,
        message: Optional[str] = None,
        target: Optional[List[str]] = None,
        expect_no_changes: Optional[bool] = None,
        color: Optional[str] = None,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
        show_secrets: bool = True,
        log_flow: Optional[bool] = None,
        log_verbosity: Optional[int] = None,
        log_to_std_err: Optional[bool] = None,
        tracing: Optional[str] = None,
        debug: Optional[bool] = None,
        suppress_outputs: Optional[bool] = None,
        suppress_progress: Optional[bool] = None,
    ) -> RefreshResult:
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
        :param color: Colorize output. Choices are: always, never, raw, auto (default "auto")
        :param show_secrets: Include config secrets in the RefreshResult summary.
        :param log_flow: Flow log settings to child processes (like plugins)
        :param log_verbosity: Enable verbose logging (e.g., v=3); anything >3 is very verbose
        :param log_to_std_err: Log to stderr instead of to files
        :param tracing: Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
        :param debug: Print detailed debugging output during resource operations
        :param suppress_outputs: Suppress display of stack outputs (in case they contain sensitive values)
        :param suppress_progress: Suppress display of periodic progress dots
        :returns: RefreshResult
        """
        extra_args = _parse_extra_args(**locals())
        args = ["refresh", "--yes", "--skip-preview"]
        args.extend(extra_args)

        args.extend(self._remote_args())

        kind = ExecKind.INLINE.value if self.workspace.program else ExecKind.LOCAL.value
        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("refresh")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(
                target=_watch_logs, args=(log_file, on_event)
            )
            log_watcher_thread.start()

        try:
            refresh_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread)

        # If it's a remote workspace, explicitly set show_secrets to False to prevent attempting to
        # load the project file.
        summary = self.info(show_secrets and not self._remote)
        assert summary is not None
        return RefreshResult(
            stdout=refresh_result.stdout, stderr=refresh_result.stderr, summary=summary
        )

    def destroy(
        self,
        parallel: Optional[int] = None,
        message: Optional[str] = None,
        target: Optional[List[str]] = None,
        target_dependents: Optional[bool] = None,
        color: Optional[str] = None,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
        show_secrets: bool = True,
        log_flow: Optional[bool] = None,
        log_verbosity: Optional[int] = None,
        log_to_std_err: Optional[bool] = None,
        tracing: Optional[str] = None,
        exclude_protected: Optional[bool] = None,
        debug: Optional[bool] = None,
        suppress_outputs: Optional[bool] = None,
        suppress_progress: Optional[bool] = None,
        continue_on_error: Optional[bool] = None,
        remove: Optional[bool] = None,
        refresh: Optional[bool] = None,
    ) -> DestroyResult:
        """
        Destroy deletes all resources in a stack, leaving all history and configuration intact.

        :param parallel: Parallel is the number of resource operations to run in parallel at once.
                         (1 for no parallelism). Defaults to unbounded (2147483647).
        :param message: Message (optional) to associate with the destroy operation.
        :param target: Specify an exclusive list of resource URNs to destroy.
        :param target_dependents: Allows updating of dependent targets discovered but not specified in the Target list.
        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :param color: Colorize output. Choices are: always, never, raw, auto (default "auto")
        :param show_secrets: Include config secrets in the DestroyResult summary.
        :param log_flow: Flow log settings to child processes (like plugins)
        :param log_verbosity: Enable verbose logging (e.g., v=3); anything >3 is very verbose
        :param log_to_std_err: Log to stderr instead of to files
        :param tracing: Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
        :param exclude_protected: Do not destroy protected resources. Destroy all other resources.
        :param debug: Print detailed debugging output during resource operations
        :param suppress_outputs: Suppress display of stack outputs (in case they contain sensitive values)
        :param suppress_progress: Suppress display of periodic progress dots
        :param continue_on_error: Continue to perform the destroy operation despite the occurrence of errors
        :param remove: Remove the stack and its configuration after all resources in the stack have been deleted.
        :param refresh: Refresh the state of the stack's resources against the cloud provider before running destroy.
        :returns: DestroyResult
        """
        extra_args = _parse_extra_args(**locals())
        args = ["destroy", "--yes", "--skip-preview"]
        args.extend(extra_args)

        args.extend(self._remote_args())

        kind = ExecKind.INLINE.value if self.workspace.program else ExecKind.LOCAL.value
        args.extend(["--exec-kind", kind])

        log_watcher_thread = None
        temp_dir = None
        if on_event:
            log_file, temp_dir = _create_log_file("destroy")
            args.extend(["--event-log", log_file])
            log_watcher_thread = threading.Thread(
                target=_watch_logs, args=(log_file, on_event)
            )
            log_watcher_thread.start()

        try:
            destroy_result = self._run_pulumi_cmd_sync(args, on_output)
        finally:
            _cleanup(temp_dir, log_watcher_thread)

        # If it's a remote workspace, explicitly set show_secrets to False to prevent attempting to
        # load the project file.
        summary = self.info(show_secrets and not self._remote)
        assert summary is not None

        # If `remove` was set, remove the stack now. We take this approach
        # rather than passing `--remove` to `pulumi destroy` because the latter
        # would make it impossible for us to retrieve a summary of the operation
        # above for returning to the caller.
        if remove:
            self.workspace.remove_stack(self.name)

        return DestroyResult(
            stdout=destroy_result.stdout, stderr=destroy_result.stderr, summary=summary
        )

    def import_resources(
        self,
        message: Optional[str] = None,
        resources: Optional[List[ImportResource]] = None,
        name_table: Optional[Dict[str, str]] = None,
        protect: Optional[bool] = None,
        generate_code: Optional[bool] = None,
        converter: Optional[str] = None,
        converter_args: Optional[List[str]] = None,
        on_output: Optional[OnOutput] = None,
        show_secrets: bool = True,
    ) -> ImportResult:
        """
        Imports resources into the stack.

        :param message: Message to associate with the import operation.
        :param resources: The resources to import.
        :param nameTable:
            The name table maps language names to parent and provider URNs.
            These names are used in the generated definitions,
            and should match the corresponding declarations
            in the source program. This table is required if any parents or providers are \
            specified by the resources to import.
        :param protect: Whether to protect the imported resources so that they are not deleted
        :param generate_code: Whether to generate code for the imported resources
        :param converter: The converter plugin to use for the import operation
        :param converter_args: Additional arguments to pass to the converter plugin
        :param on_output: A function to process the stdout stream.
        :param show_secrets: Include config secrets in the ImportResult summary.
        """
        args = ["import", "--yes", "--skip-preview"]
        if message is not None:
            args.extend(["--message", message])

        with tempfile.TemporaryDirectory(prefix="pulumi-import-") as temp_dir:
            if resources is not None:
                import_file_path = os.path.join(temp_dir, "import.json")
                with open(import_file_path, mode="w", encoding="utf-8") as import_file:
                    contents = {"resources": resources, "nameTable": name_table}
                    json.dump(contents, import_file)
                    args.extend(["--file", import_file_path])

            if protect is not None:
                value = "true" if protect else "false"
                args.append(f"--protect={value}")

            generated_code_path = os.path.join(temp_dir, "generated_code.txt")
            if generate_code is False:
                args.append("--generate-code=false")
            else:
                args.append(f"--out={generated_code_path}")

            if converter is not None:
                args.extend(["--from", converter])
                if converter_args is not None:
                    args.append("--")
                    args.extend(converter_args)

            import_result = self._run_pulumi_cmd_sync(args, on_output)
            summary = self.info(show_secrets and not self._remote)
            generated_code = ""
            if generate_code is not False:
                with open(generated_code_path, mode="r", encoding="utf-8") as codeFile:
                    generated_code = codeFile.read()

            assert summary is not None
            return ImportResult(
                stdout=import_result.stdout,
                stderr=import_result.stderr,
                generated_code=generated_code,
                summary=summary,
            )

    def add_environments(self, *environment_names: str) -> None:
        """
        Adds environments to the end of a stack's import list. Imported environments are merged in order
        per the ESC merge rules. The list of environments behaves as if it were the import list in an anonymous
        environment.

        :param environment_names: The names of the environments to add.
        """
        return self.workspace.add_environments(self.name, *environment_names)

    def list_environments(self) -> List[str]:
        """
        Returns the list of environments specified in a stack's configuration.
        """
        return self.workspace.list_environments(self.name)

    def remove_environment(self, environment_name: str) -> None:
        """
        Removes an environment from a stack's import list.
        """
        return self.workspace.remove_environment(self.name, environment_name)

    def get_config(self, key: str, *, path: bool = False) -> ConfigValue:
        """
        Returns the config value associated with the specified key.

        :param key: The key for the config item to get.
        :param path: The key contains a path to a property in a map or list to get.
        :returns: ConfigValue
        """
        return self.workspace.get_config(self.name, key, path=path)

    def get_all_config(self) -> ConfigMap:
        """
        Returns the full config map associated with the stack in the Workspace.

        :returns: ConfigMap
        """
        return self.workspace.get_all_config(self.name)

    def set_config(self, key: str, value: ConfigValue, *, path: bool = False) -> None:
        """
        Sets a config key-value pair on the Stack in the associated Workspace.

        :param key: The config key to add.
        :param value: The config value to add.
        :param path: The key contains a path to a property in a map or list to set.
        """
        self.workspace.set_config(self.name, key, value, path=path)

    def set_all_config(self, config: ConfigMap, *, path: bool = False) -> None:
        """
        Sets all specified config values on the stack in the associated Workspace.

        :param config: A mapping of key to ConfigValue to set to config.
        :param path: The keys contain a path to a property in a map or list to set.
        """
        self.workspace.set_all_config(self.name, config, path=path)

    def remove_config(self, key: str, *, path: bool = False) -> None:
        """
        Removes the specified config key from the Stack in the associated Workspace.

        :param key: The key to remove from config.
        :param path: The key contains a path to a property in a map or list to remove.
        """
        self.workspace.remove_config(self.name, key, path=path)

    def remove_all_config(self, keys: List[str], *, path: bool = False) -> None:
        """
        Removes the specified config keys from the Stack in the associated Workspace.

        :param keys: The keys to remove from config.
        :param path: The keys contain a path to a property in a map or list to remove.
        """
        self.workspace.remove_all_config(self.name, keys, path=path)

    def refresh_config(self) -> None:
        """Gets and sets the config map used with the last update."""
        self.workspace.refresh_config(self.name)

    def get_tag(self, key: str) -> str:
        """
        Returns the tag value associated with specified key.

        :param key: The key to use for the tag lookup.
        :returns: str
        """
        return self.workspace.get_tag(self.name, key)

    def set_tag(self, key: str, value: str) -> None:
        """
        Sets a tag key-value pair on the Stack in the associated Workspace.

        :param key: The tag key to set.
        :param value: The tag value to set.
        """
        self.workspace.set_tag(self.name, key, value)

    def remove_tag(self, key: str) -> None:
        """
        Removes the specified key-value pair on the provided stack name.

        :param stack_name: The name of the stack.
        :param key: The tag key to remove.
        """
        self.workspace.remove_tag(self.name, key)

    def list_tags(self) -> TagMap:
        """
        Returns the tag map for the specified tag name, scoped to the Workspace.

        :param stack_name: The name of the stack.
        :returns: TagMap
        """
        return self.workspace.list_tags(self.name)

    def outputs(self) -> OutputMap:
        """
        Gets the current set of Stack outputs from the last Stack.up().

        :returns: OutputMap
        """
        return self.workspace.stack_outputs(self.name)

    def history(
        self,
        page_size: Optional[int] = None,
        page: Optional[int] = None,
        show_secrets: bool = True,
    ) -> List[UpdateSummary]:
        """
        Returns a list summarizing all previous and current results from Stack lifecycle operations
        (up/preview/refresh/destroy).

        :param page_size: Paginate history entries (used in combination with page), defaults to all.
        :param page: Paginate history entries (used in combination with page_size), defaults to all.
        :param show_secrets: Show config secrets when they appear in history.

        :returns: List[UpdateSummary]
        """
        args = ["stack", "history", "--json"]
        if show_secrets:
            args.append("--show-secrets")
        if page_size is not None:
            # default page=1 when page_size is set
            if page is None:
                page = 1
            args.extend(["--page-size", str(page_size), "--page", str(page)])
        result = self._run_pulumi_cmd_sync(args)
        summary_list = json.loads(result.stdout)

        summaries: List[UpdateSummary] = []
        for summary_json in summary_list:
            summary = UpdateSummary(
                kind=summary_json["kind"],
                start_time=datetime.strptime(
                    summary_json["startTime"], _DATETIME_FORMAT
                ),
                message=summary_json["message"],
                environment=summary_json["environment"],
                config=summary_json["config"],
                result=summary_json["result"],
                end_time=(
                    datetime.strptime(summary_json["endTime"], _DATETIME_FORMAT)
                    if "endTime" in summary_json
                    else None
                ),
                version=summary_json["version"] if "version" in summary_json else None,
                deployment=(
                    summary_json["Deployment"] if "Deployment" in summary_json else None
                ),
                resource_changes=(
                    summary_json["resourceChanges"]
                    if "resourceChanges" in summary_json
                    else None
                ),
            )
            summaries.append(summary)
        return summaries

    def info(self, show_secrets=True) -> Optional[UpdateSummary]:
        """
        Returns the current results from Stack lifecycle operations.

        :returns: Optional[UpdateSummary]
        """
        history = self.history(page_size=1, show_secrets=show_secrets)
        if not history:
            return None
        return history[0]

    def cancel(self) -> None:
        """
        Cancel stops a stack's currently running update. It returns an error if no update is currently running.
        Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
        if a resource operation was pending when the update was canceled.
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

    def _run_pulumi_cmd_sync(
        self, args: List[str], on_output: Optional[OnOutput] = None
    ) -> CommandResult:
        envs = {"PULUMI_DEBUG_COMMANDS": "true"}
        if self._remote:
            envs = {**envs, "PULUMI_EXPERIMENTAL": "true"}
        if self.workspace.pulumi_home is not None:
            envs = {**envs, "PULUMI_HOME": self.workspace.pulumi_home}
        envs = {**envs, **self.workspace.env_vars}

        additional_args = self.workspace.serialize_args_for_op(self.name)
        args.extend(additional_args)
        args.extend(["--stack", self.name])
        result = self.workspace.pulumi_command.run(
            args, self.workspace.work_dir, envs, on_output
        )
        self.workspace.post_command_callback(self.name)
        return result

    @property
    def _remote(self) -> bool:
        from pulumi.automation._local_workspace import LocalWorkspace

        return (
            self.workspace._remote
            if isinstance(self.workspace, LocalWorkspace)
            else False
        )

    def _remote_args(self) -> List[str]:
        from pulumi.automation._local_workspace import LocalWorkspace

        return (
            self.workspace._remote_args()
            if isinstance(self.workspace, LocalWorkspace)
            else []
        )


def _parse_extra_args(**kwargs) -> List[str]:
    extra_args: List[str] = []

    message: Optional[str] = kwargs.get("message")
    expect_no_changes: Optional[bool] = kwargs.get("expect_no_changes")
    diff: Optional[bool] = kwargs.get("diff")
    replace: Optional[List[str]] = kwargs.get("replace")
    target: Optional[List[str]] = kwargs.get("target")
    policy_packs: Optional[List[str]] = kwargs.get("policy_packs")
    policy_pack_configs: Optional[List[str]] = kwargs.get("policy_pack_configs")
    target_dependents: Optional[bool] = kwargs.get("target_dependents")
    parallel: Optional[int] = kwargs.get("parallel")
    color: Optional[str] = kwargs.get("color")
    log_flow: Optional[bool] = kwargs.get("log_flow")
    log_verbosity: Optional[int] = kwargs.get("log_verbosity")
    log_to_std_err: Optional[bool] = kwargs.get("log_to_std_err")
    tracing: Optional[str] = kwargs.get("tracing")
    exclude_protected: Optional[bool] = kwargs.get("exclude_protected")
    debug: Optional[bool] = kwargs.get("debug")
    suppress_outputs: Optional[bool] = kwargs.get("suppress_outputs")
    suppress_progress: Optional[bool] = kwargs.get("suppress_progress")
    continue_on_error: Optional[bool] = kwargs.get("continue_on_error")
    attach_debugger: Optional[bool] = kwargs.get("attach_debugger")
    refresh: Optional[bool] = kwargs.get("refresh")

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
    if policy_packs:
        for p in policy_packs:
            extra_args.extend(["--policy-pack", p])
    if policy_pack_configs:
        for p in policy_pack_configs:
            extra_args.extend(["--policy-pack-config", p])
    if target_dependents:
        extra_args.append("--target-dependents")
    if parallel:
        extra_args.extend(["--parallel", str(parallel)])
    if color:
        extra_args.extend(["--color", color])
    if log_flow:
        extra_args.extend(["--logflow"])
    if log_verbosity:
        extra_args.extend(["--verbose", str(log_verbosity)])
    if log_to_std_err:
        extra_args.extend(["--logtostderr"])
    if tracing:
        extra_args.extend(["--tracing", tracing])
    if exclude_protected:
        extra_args.extend(["--exclude-protected"])
    if debug:
        extra_args.extend(["--debug"])
    if suppress_outputs:
        extra_args.extend(["--suppress-outputs"])
    if suppress_progress:
        extra_args.extend(["--suppress-progress"])
    if continue_on_error:
        extra_args.extend(["--continue-on-error"])
    if attach_debugger:
        extra_args.extend(["--attach-debugger"])
    if refresh:
        extra_args.extend(["--refresh"])
    return extra_args


def fully_qualified_stack_name(org: str, project: str, stack: str) -> str:
    """
    Returns a stack name formatted with the greatest possible specificity:
    org/project/stack or user/project/stack

    Using this format avoids ambiguity in stack identity guards creating or selecting the wrong stack.

    Note that legacy diy backends (local file, S3, Azure Blob) do not support stack names in this
    format, and instead only use the stack name without an org/user or project to qualify it.
    See: https://github.com/pulumi/pulumi/issues/2522
    Non-legacy diy backends do support the org/project/stack format but org must be set to "organization".

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
    with open(filepath, "w+", encoding="utf-8"):
        pass
    return filepath, log_dir


def _watch_logs(filename: str, callback: OnEvent):
    partial_line = ""
    with open(filename, encoding="utf-8") as f:
        while True:
            line = f.readline()

            # sleep if file hasn't been updated
            if not line:
                time.sleep(0.1)
                continue

            # we don't have a complete line yet.  sleep and try again.
            if line[-1] != "\n":
                partial_line += line
                time.sleep(0.1)
                continue

            line = partial_line + line
            partial_line = ""

            event = EngineEvent.from_json(json.loads(line))
            callback(event)

            # if this is the cancel event, stop watching logs.
            if event.cancel_event:
                break


def _cleanup(
    temp_dir: Optional[tempfile.TemporaryDirectory],
    thread: Optional[threading.Thread],
    on_exit_fn: Optional[Callable[[], None]] = None,
) -> None:
    # If there's an on_exit function, execute it (used in preview/up to shut down server)
    if on_exit_fn:
        on_exit_fn()
    # If we started a thread to watch logs, wait for it to terminate, timing out after 5 seconds.
    if thread:
        thread.join(5)
    # If we created a temp_dir for the logs, clean up.
    if temp_dir:
        temp_dir.cleanup()
