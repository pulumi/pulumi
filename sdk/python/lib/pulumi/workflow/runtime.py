# Copyright 2026, Pulumi Corporation.
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

"""Minimal workflow runtime scaffolding for preview-time graph registration.

This module is intentionally tiny: enough to register graphs/jobs/steps,
materialize graph/job shape with a GraphMonitor, and execute step callbacks.
"""

from __future__ import annotations

import contextlib
from concurrent import futures
from dataclasses import asdict, dataclass, is_dataclass
import asyncio
import inspect
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Type, TypeVar, Union, cast, get_args, get_origin, get_type_hints, overload

import grpc
from google.protobuf import json_format
from google.protobuf import struct_pb2
from pulumi import Output, Input
from pulumi.output import deferred_output
from pulumi.runtime.sync_await import _sync_await

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc
from .errors import WorkflowError
from .helpers import (
    _WORKFLOW_EXTERNAL_JOB_TOKEN_ATTR,
    _WORKFLOW_EXTERNAL_STEP_TOKEN_ATTR,
    _WORKFLOW_OUTPUT_PATHS_ATTR,
    _coerce_to_struct_data,
    _default_job_name_for_token,
    _default_step_name_for_token,
    _ensure_event_loop,
    _external_job_token,
    _external_step_token,
    _input_bool_filter_callback,
    _is_workflow_output,
    _job_return_output_type,
    _mock_return_type,
    _new_workflow_output,
    _normalize_job_dependency,
    _normalize_step_dependency,
    _qualify_job_token,
    _qualify_step_token,
    _qualify_trigger_token,
    _step_return_type,
    _validate_record_type,
    _validate_step_type,
    _workflow_output_paths,
    _workflow_input_value,
)

OnErrorHandler = Callable[
    [List[workflow_pb2.ErrorRecord]], Union[bool, Tuple[bool, str], None]
]
T = TypeVar("T")
U = TypeVar("U")

JobCallback = Callable[..., Any]
GraphCallback = Callable[["Context"], None]
FilterCallback = Callable[[Any], bool]
TInput = TypeVar("TInput")
TOutput = TypeVar("TOutput")
TriggerMockCallback = Callable[[List[str]], TOutput]
StepCallback = Callable[[Any], Any]

_WORKFLOW_OUTPUT_PATHS_ATTR = "_pulumi_workflow_output_paths"
_WORKFLOW_OUTPUT_VALUE_ATTR = "_pulumi_workflow_output_value"
_WORKFLOW_OUTPUT_MARKER_ATTR = "_pulumi_workflow_output_marker"
_WORKFLOW_EXTERNAL_JOB_TOKEN_ATTR = "_pulumi_workflow_external_job_token"
_WORKFLOW_EXTERNAL_STEP_TOKEN_ATTR = "_pulumi_workflow_external_step_token"


@dataclass
class _StepDefinition:
    has_arg: bool
    arg: Any
    fn: Callable[..., Any]
    on_error: Optional[OnErrorHandler]
    resolve_output: Callable[[Output[Any]], None]
    dependency_paths: Set[str]
    external_token: Optional[str] = None
    expected_output_type: Optional[Type[Any]] = None


@dataclass
class _JobDefinition:
    fn: Optional[JobCallback]
    on_error: Optional[OnErrorHandler]
    dependencies: List[str]
    external_token: Optional[str] = None
    enabled: bool = True


@dataclass
class _ExportedJobDefinition:
    token: str
    input_type: Type[Any]
    output_type: Type[Any]
    fn: Callable[[JobContext, Any], Output[Any]]
    on_error: Optional[OnErrorHandler]


@dataclass
class _TriggerDefinition:
    token: str
    input_type: Type[Any]
    output_type: Type[Any]
    mock: Callable[[List[str]], Any]


@dataclass
class _ExportedStepDefinition:
    token: str
    input_type: Type[Any]
    output_type: Type[Any]
    fn: StepCallback
    on_error: Optional[OnErrorHandler]


@dataclass
class _EvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    graph_path: str
    input_value_path: str
    input_value: Optional[struct_pb2.Struct]
    registry: "WorkflowRegistry"
    jobs: Dict[str, _JobDefinition]
    filters: Dict[str, FilterCallback]
    target_job_name: Optional[str]


@dataclass
class _JobEvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    job_path: str
    steps: Dict[str, _StepDefinition]
    registry: "WorkflowRegistry"
    filters: Dict[str, FilterCallback]


class Context:
    """Execution/evaluation context passed to graph functions."""

    def __init__(self, state: _EvalState) -> None:
        self._state = state

    @property
    def execution_id(self) -> str:
        return self._state.context.execution_id

    @property
    def workflow_version(self) -> str:
        return self._state.context.workflow_version

    def trigger(
        self,
        name: str,
        trigger_type: str,
        spec: Optional[Any] = None,
        *,
        options: Optional[TriggerOptions] = None,
    ) -> Output[Any]:
        """Registers a trigger in the current graph."""

        request = workflow_pb2.RegisterTriggerRequest()
        trigger_path = f"{self._state.graph_path}/{name}"
        filter_fn = options.filter if options is not None else None
        if self._state.target_job_name is not None:
            return _new_workflow_output(
                trigger_path,
                _workflow_input_value(
                    self._state.input_value_path, self._state.input_value, trigger_path
                ),
            )
        request.context.CopyFrom(self._state.context)
        request.path = trigger_path
        request.type = self._state.registry.resolve_trigger_token(trigger_type)
        request.has_filter = filter_fn is not None
        if spec is not None:
            trigger_spec = struct_pb2.Struct()
            trigger_spec.update(_coerce_to_struct_data(spec))
            request.spec.CopyFrom(trigger_spec)

        self._state.monitor.RegisterTrigger(request)
        if filter_fn is not None:
            self._state.filters[trigger_path] = filter_fn
        return _new_workflow_output(
            trigger_path,
            _workflow_input_value(
                self._state.input_value_path, self._state.input_value, trigger_path
            ),
        )

    def job(
        self,
        name: str,
        fn_or_options: Optional[Union[JobCallback, "JobOptions"]] = None,
        fn: Optional[JobCallback] = None,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
        filter: Optional[Input[bool]] = None,
    ) -> Union[Output[Any], Callable[[JobCallback], JobCallback]]:
        """Registers a job in the current graph."""
        options: Optional[JobOptions] = None
        if isinstance(fn_or_options, JobOptions):
            options = fn_or_options
        elif callable(fn_or_options):
            fn = cast(JobCallback, fn_or_options)
        elif fn_or_options is not None:
            raise WorkflowError(
                "job no longer accepts input args; second positional must be a callback or JobOptions"
            )

        if options is not None and dependencies is not None and options.dependencies is not None:
            raise WorkflowError("job dependencies must be set either directly or via JobOptions, not both")
        if options is not None and on_error is not None and options.on_error is not None:
            raise WorkflowError("job on_error must be set either directly or via JobOptions, not both")
        if options is not None and filter is not None and options.filter is not None:
            raise WorkflowError("job filter must be set either directly or via JobOptions, not both")

        effective_dependencies = dependencies
        if effective_dependencies is None and options is not None:
            effective_dependencies = options.dependencies

        effective_on_error = on_error
        if effective_on_error is None and options is not None:
            effective_on_error = options.on_error
        effective_filter = filter
        if effective_filter is None and options is not None:
            effective_filter = options.filter

        referenced_external_token = _external_job_token(fn)
        is_external_job = (fn is None and ":" in name) or referenced_external_token is not None
        registered_name = name
        external_token: Optional[str] = None
        if is_external_job:
            if referenced_external_token is not None:
                external_token = referenced_external_token
            else:
                external_token = self._state.registry.resolve_job_token(name)
            if options is not None and options.name:
                registered_name = options.name
            elif ":" in name:
                registered_name = _default_job_name_for_token(name)

        def register(registered_fn: JobCallback) -> Output[Any]:
            if not registered_name:
                raise WorkflowError("job name is required")
            job_path = f"{self._state.graph_path}/jobs/{registered_name}"
            job_output = _new_workflow_output(
                job_path,
                _workflow_input_value(
                    self._state.input_value_path, self._state.input_value, job_path
                ),
            )
            if (
                self._state.target_job_name is not None
                and registered_name != self._state.target_job_name
            ):
                return job_output

            request = workflow_pb2.RegisterJobRequest()
            request.context.CopyFrom(self._state.context)
            request.path = job_path
            request.has_on_error = effective_on_error is not None
            request.dependencies.operator = (
                workflow_pb2.DependencyExpression.OPERATOR_ALL
            )
            dependency_paths: Set[str] = set()
            if effective_dependencies:
                for dep in effective_dependencies:
                    dependency_paths.add(
                        _normalize_job_dependency(self._state.graph_path, dep)
                    )
            for dep in sorted(dependency_paths):
                term = request.dependencies.terms.add()
                term.path = dep

            if self._state.target_job_name is None:
                self._state.monitor.RegisterJob(request)
            if effective_filter is not None:
                self._state.filters[job_path] = _input_bool_filter_callback(effective_filter)
            self._state.jobs[job_path] = _JobDefinition(
                fn=registered_fn,
                on_error=effective_on_error,
                dependencies=sorted(dependency_paths),
                external_token=external_token,
                enabled=True,
            )
            return job_output

        if is_external_job:
            return register(lambda *_args: None)

        if fn is not None:
            return register(fn)

        def decorator(registered_fn: JobCallback) -> JobCallback:
            register(registered_fn)
            return registered_fn

        return decorator


class JobContext:
    """Evaluation context passed to job functions while generating a job."""

    def __init__(self, state: _JobEvalState) -> None:
        self._state = state

    @property
    def execution_id(self) -> str:
        return self._state.context.execution_id

    @property
    def workflow_version(self) -> str:
        return self._state.context.workflow_version

    @overload
    def step(
        self,
        name: str,
        fn: Callable[[], U],
        *,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
        filter: Optional[Input[bool]] = None,
    ) -> Output[U]: ...

    @overload
    def step(
        self,
        name: str,
        arg: Input[T],
        fn: Union[Callable[[T], U], "StepOptions"],
        *,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
        filter: Optional[Input[bool]] = None,
    ) -> Output[U]: ...

    def step(
        self,
        name: str,
        arg: Optional[Input[T]] = None,
        fn: Optional[Union[Callable[..., U], "StepOptions"]] = None,
        *,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
        filter: Optional[Input[bool]] = None,
    ) -> Union[Output[U], Callable[[Callable[..., U]], Output[U]]]:
        """Registers a step in the current job."""
        options: Optional[StepOptions] = None
        if isinstance(fn, StepOptions):
            options = fn
            fn = None
        if isinstance(arg, StepOptions):
            if options is not None:
                raise WorkflowError("step options may only be provided once")
            options = arg
            arg = None

        if options is not None and dependencies is not None and options.dependencies is not None:
            raise WorkflowError("step dependencies must be set either directly or via StepOptions, not both")
        if options is not None and on_error is not None and options.on_error is not None:
            raise WorkflowError("step on_error must be set either directly or via StepOptions, not both")
        if options is not None and filter is not None and options.filter is not None:
            raise WorkflowError("step filter must be set either directly or via StepOptions, not both")

        effective_dependencies = dependencies
        if effective_dependencies is None and options is not None:
            effective_dependencies = options.dependencies
        effective_on_error = on_error
        if effective_on_error is None and options is not None:
            effective_on_error = options.on_error
        effective_filter = filter
        if effective_filter is None and options is not None:
            effective_filter = options.filter

        if fn is None and callable(arg):
            fn = cast(Callable[[T], U], arg)
            arg = None
        referenced_external_token = _external_step_token(fn)
        is_external_step = (fn is None and ":" in name) or referenced_external_token is not None
        registered_name = name
        external_token: Optional[str] = None
        if is_external_step:
            if referenced_external_token is not None:
                external_token = referenced_external_token
            else:
                external_token = self._state.registry.resolve_step_token(name)
            if options is not None and options.name:
                registered_name = options.name
            elif ":" in name:
                registered_name = _default_step_name_for_token(name)

        def register(registered_fn: Callable[[T], U]) -> Output[U]:
            if not registered_name:
                raise WorkflowError("step name is required")
            if isinstance(arg, Output) and not _is_workflow_output(arg):
                raise WorkflowError(
                    "workflow steps may only accept workflow outputs; resource outputs are not supported"
                )
            step_path = f"{self._state.job_path}/steps/{registered_name}"
            request = workflow_pb2.RegisterStepRequest()
            request.context.CopyFrom(self._state.context)
            if hasattr(request, "path"):
                request.path = step_path
            else:
                request.name = registered_name
                request.job = self._state.job_path
            request.has_on_error = effective_on_error is not None
            request.dependencies.operator = (
                workflow_pb2.DependencyExpression.OPERATOR_ALL
            )
            dependency_paths = set()
            if effective_dependencies:
                for dep in effective_dependencies:
                    dependency_paths.add(
                        _normalize_step_dependency(self._state.job_path, dep)
                    )
            dependency_paths.update(_workflow_output_paths(arg))
            for dep in sorted(dependency_paths):
                term = request.dependencies.terms.add()
                term.path = dep

            self._state.monitor.RegisterStep(request)
            if effective_filter is not None:
                self._state.filters[step_path] = _input_bool_filter_callback(effective_filter)
            _ensure_event_loop()
            step_output, resolve_output = deferred_output()
            setattr(step_output, _WORKFLOW_OUTPUT_PATHS_ATTR, {step_path})
            self._state.steps[step_path] = _StepDefinition(
                has_arg=arg is not None,
                arg=arg,
                fn=registered_fn,
                on_error=effective_on_error,
                resolve_output=resolve_output,
                dependency_paths=set(dependency_paths),
                external_token=external_token,
            )
            return cast(Output[U], step_output)

        if is_external_step:
            return register(cast(Callable[[T], U], lambda _arg=None: None))
        if fn is not None:
            return register(cast(Callable[[T], U], fn))
        return register


@dataclass
class TriggerOptions:
    filter: Optional[FilterCallback] = None


@dataclass
class JobOptions:
    name: Optional[str] = None
    dependencies: Optional[List[str]] = None
    on_error: Optional[OnErrorHandler] = None
    filter: Optional[Input[bool]] = None


@dataclass
class StepOptions:
    name: Optional[str] = None
    dependencies: Optional[List[str]] = None
    on_error: Optional[OnErrorHandler] = None
    filter: Optional[Input[bool]] = None


class WorkflowRegistry:
    """Collects exported workflow components before evaluation."""

    def __init__(self, package_name: str) -> None:
        self._package_name = package_name
        self._graphs: Dict[str, GraphCallback] = {}
        self._jobs: Dict[str, _ExportedJobDefinition] = {}
        self._steps: Dict[str, _ExportedStepDefinition] = {}
        self._triggers: Dict[str, _TriggerDefinition] = {}

    def graph(
        self,
        name: str,
    ) -> Callable[[GraphCallback], GraphCallback]:
        """Registers a graph export by simple name."""

        def register(registered_fn: GraphCallback) -> GraphCallback:
            if name in self._graphs:
                raise WorkflowError(f"graph '{name}' is already registered")
            self._graphs[name] = registered_fn
            return registered_fn

        return register

    def trigger(
        self,
        token: str,
        input_type: Type[TInput],
    ) -> Callable[[TriggerMockCallback[TOutput]], TriggerMockCallback[TOutput]]:
        """Registers an exported trigger by token with typed input/output and mock behavior."""
        def register(mock: TriggerMockCallback[TOutput]) -> TriggerMockCallback[TOutput]:
            if not token:
                raise WorkflowError("trigger token is required")
            resolved_token = self.resolve_trigger_token(token)
            if resolved_token in self._triggers:
                raise WorkflowError(f"trigger '{resolved_token}' is already registered")
            if not callable(mock):
                raise WorkflowError("trigger mock must be callable")

            _validate_record_type(input_type, "trigger input type")
            output_type = _mock_return_type(mock)
            _validate_record_type(output_type, "trigger output type")

            self._triggers[resolved_token] = _TriggerDefinition(
                token=resolved_token,
                input_type=input_type,
                output_type=output_type,
                mock=mock,
            )
            return mock

        return register

    def resolve_trigger_token(self, token: str) -> str:
        if token in self._triggers:
            return token
        qualified = _qualify_trigger_token(self._package_name, token)
        return qualified

    def job(
        self,
        token: str,
        input_type: Type[TInput],
        *,
        on_error: Optional[OnErrorHandler] = None,
    ) -> Callable[[Callable[[JobContext, TInput], Output[TOutput]]], Callable[[JobContext, TInput], Output[TOutput]]]:
        """Registers an exported job.

        The decorated function is also returned with an attached external token marker,
        so it can be passed to `Context.job(...)` when composing graphs.
        """
        def register(fn: Callable[[JobContext, TInput], Output[TOutput]]) -> Callable[[JobContext, TInput], Output[TOutput]]:
            if not token:
                raise WorkflowError("job token is required")
            resolved_token = self.resolve_job_token(token)
            if resolved_token in self._jobs:
                raise WorkflowError(f"job '{resolved_token}' is already registered")
            if not callable(fn):
                raise WorkflowError("job callback must be callable")
            _validate_record_type(input_type, "job input type")

            output_type = _job_return_output_type(fn)
            _validate_step_type(output_type, "job output type")

            self._jobs[resolved_token] = _ExportedJobDefinition(
                token=resolved_token,
                input_type=input_type,
                output_type=output_type,
                fn=cast(Callable[[JobContext, Any], Output[Any]], fn),
                on_error=on_error,
            )
            setattr(fn, _WORKFLOW_EXTERNAL_JOB_TOKEN_ATTR, resolved_token)
            return fn

        return register

    def step(
        self,
        token: str,
        input_type: Type[TInput],
        *,
        on_error: Optional[OnErrorHandler] = None,
    ) -> Callable[[Callable[[TInput], TOutput]], Callable[[TInput], TOutput]]:
        """Registers an exported step.

        The decorated function is also returned with an attached external token marker,
        so it can be passed to `JobContext.step(...)` when composing jobs.
        """
        def register(fn: Callable[[TInput], TOutput]) -> Callable[[TInput], TOutput]:
            if not token:
                raise WorkflowError("step token is required")
            resolved_token = self.resolve_step_token(token)
            if resolved_token in self._steps:
                raise WorkflowError(f"step '{resolved_token}' is already registered")
            if not callable(fn):
                raise WorkflowError("step callback must be callable")

            _validate_step_type(input_type, "step input type")
            output_type = _step_return_type(fn)
            _validate_step_type(output_type, "step output type")

            self._steps[resolved_token] = _ExportedStepDefinition(
                token=resolved_token,
                input_type=input_type,
                output_type=output_type,
                fn=cast(StepCallback, fn),
                on_error=on_error,
            )
            setattr(fn, _WORKFLOW_EXTERNAL_STEP_TOKEN_ATTR, resolved_token)
            return fn

        return register

    def resolve_job_token(self, token: str) -> str:
        if token in self._jobs:
            return token
        qualified = _qualify_job_token(self._package_name, token)
        return qualified

    def resolve_step_token(self, token: str) -> str:
        if token in self._steps:
            return token
        qualified = _qualify_step_token(self._package_name, token)
        return qualified




def run(
    package_name: str,
    package_version: str,
    register: Callable[[WorkflowRegistry], None],
) -> None:
    """Runs a WorkflowEvaluator gRPC server and prints the bound port on stdout."""

    if not package_name:
        raise WorkflowError("package_name is required")
    if not package_version:
        raise WorkflowError("package_version is required")

    from .evaluator import _WorkflowEvaluatorServer

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    workflow_pb2_grpc.add_WorkflowEvaluatorServicer_to_server(
        _WorkflowEvaluatorServer(package_name, package_version, register),
        server,
    )
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    print(port, flush=True)
    server.wait_for_termination()


__all__ = [
    "Context",
    "JobContext",
    "TriggerOptions",
    "WorkflowRegistry",
    "WorkflowError",
    "Output",
    "run",
]
