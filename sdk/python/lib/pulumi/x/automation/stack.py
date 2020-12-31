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

import json
import grpc
from concurrent import futures
from enum import Enum
from datetime import datetime
from dataclasses import dataclass
from typing import List, Any, Mapping, MutableMapping, Literal, Optional, Callable

from .cmd import CommandResult, _run_pulumi_cmd
from .config import ConfigValue, ConfigMap
from .errors import StackAlreadyExistsError
from .server import LanguageServer
from .workspace import Workspace, PulumiFn, Deployment
from ...runtime.settings import _GRPC_CHANNEL_OPTIONS
from ...runtime.proto import language_pb2_grpc

_SECRET_SENTINEL = "[secret]"


class ExecKind(str, Enum):
    LOCAL = "auto.local"
    INLINE = "auto.inline"


@dataclass
class OutputValue:
    value: Any
    secret: bool


OutputMap = MutableMapping[str, OutputValue]

UpdateKind = Literal["update", "preview", "refresh", "rename", "destroy", "import"]
"""The kind of update that was performed on the stack."""

UpdateResult = Literal["not-started", "in-progress", "succeeded", "failed"]
"""Represents the current status of a given update."""

OpType = Literal["same", "create", "update", "delete", "replace", "create-replacement", "deleted-replaced"]
"""The granular CRUD operation performed on a particular resource during an update."""

OpMap = MutableMapping[OpType, int]


@dataclass
class UpdateSummary:
    # pre-update info
    kind: UpdateKind
    start_time: datetime
    message: str
    environment: Mapping[str, str]
    config: ConfigMap

    # post-update info
    result: UpdateResult
    end_time: datetime
    version: Optional[int]
    Deployment: Optional[str]
    resource_changes: Optional[OpMap]

    def __init__(self,
                 kind: UpdateKind,
                 startTime: str,
                 message: str,
                 environment: Mapping[str, str],
                 config: Mapping[str, dict],
                 result: UpdateResult,
                 endTime: str,
                 version: Optional[int] = None,
                 Deployment: Optional[str] = None,
                 resourceChanges: Optional[OpMap] = None):
        self.kind = kind
        self.start_time = datetime.strptime(startTime[:-5], "%Y-%m-%dT%H:%M:%S")
        self.end_time = datetime.strptime(endTime[:-5], "%Y-%m-%dT%H:%M:%S")
        self.message = message
        self.environment = environment
        self.result = result
        self.Deployment = Deployment
        self.resource_changes = resourceChanges
        self.version = version
        self.config: ConfigMap = {}
        for key in config:
            self.config[key] = ConfigValue(**config[key])


@dataclass
class BaseResult:
    stdout: str
    stderr: str
    summary: UpdateSummary


@dataclass
class UpResult(BaseResult):
    outputs: OutputMap


@dataclass
class PreviewResult(BaseResult):
    pass


@dataclass
class RefreshResult(BaseResult):
    pass


@dataclass
class DestroyResult(BaseResult):
    pass


StackInitMode = Literal["create", "select", "create_or_select"]


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
        return Stack(stack_name, workspace, "create")

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
        return Stack(stack_name, workspace, "select")

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
        return Stack(stack_name, workspace, "create_or_select")

    def __init__(self, name: str, workspace: Workspace, mode: StackInitMode) -> None:
        self.name = name
        self.workspace = workspace

        if mode == "create":
            workspace.create_stack(name)
        elif mode == "select":
            workspace.select_stack(name)
        elif mode == "create_or_select":
            try:
                workspace.create_stack(name)
            except StackAlreadyExistsError:
                workspace.select_stack(name)
        else:
            raise ValueError(f"unexpected Stack creation mode: {mode}")

    def up(self,
           parallel: Optional[int] = None,
           message: Optional[str] = None,
           target: Optional[List[str]] = None,
           expect_no_changes: Optional[bool] = None,
           target_dependents: Optional[bool] = None,
           replace: Optional[List[str]] = None,
           on_output: Optional[Callable[[str], None]] = None,
           program: Optional[PulumiFn] = None) -> UpResult:
        """
        Creates or updates the resources in a stack by executing the program in the Workspace.
        https://www.pulumi.com/docs/reference/cli/pulumi_up/
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

            def on_exit(code: int):
                language_server.on_pulumi_exit(code, preview=False)
                server.stop(0)
            args.append(f"--client=127.0.0.1:{port}")

        args.extend(["--exec-kind", kind])

        up_result = self._run_pulumi_cmd_sync(args, on_output)
        if on_exit is not None:
            on_exit(up_result.code)

        outputs = self.outputs()
        summary = self.info()
        assert(summary is not None)
        return UpResult(stdout=up_result.stdout, stderr=up_result.stderr, summary=summary, outputs=outputs)

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

            def on_exit(code: int):
                language_server.on_pulumi_exit(code, preview=True)
                server.stop(0)
            args.append(f"--client=127.0.0.1:{port}")
        args.extend(["--exec-kind", kind])

        preview_result = self._run_pulumi_cmd_sync(args)
        if on_exit is not None:
            on_exit(preview_result.code)

        summary = self.info()
        assert(summary is not None)
        return PreviewResult(stdout=preview_result.stdout, stderr=preview_result.stderr, summary=summary)

    def refresh(self,
                parallel: Optional[int] = None,
                message: Optional[str] = None,
                target: Optional[List[str]] = None,
                expect_no_changes: Optional[bool] = None,
                on_output: Optional[Callable[[str], None]] = None) -> RefreshResult:
        """
        Compares the current stack’s resource state with the state known to exist in the actual
        cloud provider. Any such changes are adopted into the current stack.
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
                on_output: Callable[[str], None] = None) -> DestroyResult:
        """Destroy deletes all resources in a stack, leaving all history and configuration intact."""
        extra_args = _parse_extra_args(**locals())
        args = ["destroy", "--yes", "--skip-preview"]
        args.extend(extra_args)

        self.workspace.select_stack(self.name)
        destroy_result = self._run_pulumi_cmd_sync(args, on_output)
        summary = self.info()
        assert(summary is not None)
        return DestroyResult(stdout=destroy_result.stdout, stderr=destroy_result.stderr, summary=summary)

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

    def outputs(self) -> OutputMap:
        """Gets the current set of Stack outputs from the last Stack.up()."""
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
        """
        result = self._run_pulumi_cmd_sync(["history", "--json", "--show-secrets"])
        summary_json = json.loads(result.stdout)

        summaries: List[UpdateSummary] = []
        for summary in summary_json:
            summaries.append(UpdateSummary(**summary))
        return summaries

    def info(self) -> Optional[UpdateSummary]:
        """
        Returns the current results from Stack lifecycle operations.
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
        """
        return self.workspace.export_stack(self.name)

    def import_stack(self, state: Deployment) -> None:
        """
        import_stack imports the specified deployment state into a pre-existing stack.
        This can be combined with Stack.export_state to edit a stack's state (such as recovery from failed deployments).
        """
        return self.workspace.import_stack(self.name, state)

    def _run_pulumi_cmd_sync(self,
                             args: List[str],
                             on_output: Optional[Callable[[str], None]] = None) -> CommandResult:
        envs = {"PULUMI_HOME": self.workspace.pulumi_home} if self.workspace.pulumi_home else {}
        envs = {**envs, **self.workspace.env_vars}

        additional_args = self.workspace.serialize_args_for_op(self.name)
        args.extend(additional_args)

        result = _run_pulumi_cmd(args, self.workspace.work_dir, envs)
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
    """
    return f"{org}/{project}/{stack}"
