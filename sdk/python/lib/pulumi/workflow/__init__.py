# Copyright 2016, Pulumi Corporation.
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
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Type, TypeVar, Union, cast, get_type_hints

import grpc
from google.protobuf import json_format
from google.protobuf import struct_pb2
from pulumi.output import Output as PulumiOutput
from pulumi.runtime.sync_await import _sync_await

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc

OnErrorHandler = Callable[
    [List[workflow_pb2.ErrorRecord]], Union[bool, Tuple[bool, str], None]
]
T = TypeVar("T")
U = TypeVar("U")

JobCallback = Callable[..., None]
GraphCallback = Callable[["Context"], None]
FilterCallback = Callable[[Any], bool]
TInput = TypeVar("TInput")
TOutput = TypeVar("TOutput")
TriggerMockCallback = Callable[[List[str]], TOutput]
Output = PulumiOutput[Any]

_WORKFLOW_OUTPUT_PATHS_ATTR = "_pulumi_workflow_output_paths"
_WORKFLOW_OUTPUT_VALUE_ATTR = "_pulumi_workflow_output_value"


@dataclass
class _StepDefinition:
    has_arg: bool
    arg: Any
    fn: Callable[..., Any]
    on_error: Optional[OnErrorHandler]


@dataclass
class _JobDefinition:
    fn: JobCallback
    on_error: Optional[OnErrorHandler]
    inputs: List[Any]


@dataclass
class _TriggerDefinition:
    token: str
    input_type: Type[Any]
    output_type: Type[Any]
    mock: Callable[[List[str]], Any]


@dataclass
class _EvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    graph_path: str
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


class Context:
    """Execution/evaluation context passed to graph functions."""

    def __init__(self, state: _EvalState) -> None:
        self._state = state

    def trigger(
        self,
        name: str,
        trigger_type: str,
        spec: Optional[Any] = None,
        *,
        options: Optional[TriggerOptions] = None,
    ) -> PulumiOutput[Any]:
        """Registers a trigger in the current graph."""

        request = workflow_pb2.RegisterTriggerRequest()
        trigger_path = f"{self._state.graph_path}/{name}"
        filter_fn = options.filter if options is not None else None
        if self._state.target_job_name is not None:
            return _new_workflow_output(
                trigger_path, _workflow_input_value(self._state.context, trigger_path)
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
            trigger_path, _workflow_input_value(self._state.context, trigger_path)
        )

    def job(
        self,
        name: str,
        *inputs_or_fn: Any,
        fn: Optional[JobCallback] = None,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
    ) -> Union[JobCallback, Callable[[JobCallback], JobCallback]]:
        """Registers a job in the current graph."""
        inputs: List[Any] = list(inputs_or_fn)
        if fn is None and len(inputs) == 1 and callable(inputs[0]):
            fn = inputs[0]
            inputs = []

        def register(registered_fn: JobCallback) -> JobCallback:
            if not name:
                raise WorkflowError("job name is required")
            if (
                self._state.target_job_name is not None
                and name != self._state.target_job_name
            ):
                return registered_fn

            job_path = f"{self._state.graph_path}/jobs/{name}"

            request = workflow_pb2.RegisterJobRequest()
            request.context.CopyFrom(self._state.context)
            request.path = job_path
            request.has_on_error = on_error is not None
            request.dependencies.operator = (
                workflow_pb2.DependencyExpression.OPERATOR_ALL
            )
            dependency_paths = set()
            if dependencies:
                for dep in dependencies:
                    dependency_paths.add(
                        _normalize_job_dependency(self._state.graph_path, dep)
                    )
            dependency_paths.update(_workflow_dependency_paths(inputs))
            for dep in sorted(dependency_paths):
                term = request.dependencies.terms.add()
                term.path = dep

            resolved_inputs = [_resolve_job_input(value) for value in inputs]
            self._state.monitor.RegisterJob(request)
            self._state.jobs[job_path] = _JobDefinition(
                fn=registered_fn, on_error=on_error, inputs=resolved_inputs
            )
            return registered_fn

        if fn is not None:
            return register(fn)
        return register


class JobContext:
    """Evaluation context passed to job functions while generating a job."""

    def __init__(self, state: _JobEvalState) -> None:
        self._state = state

    def step(
        self,
        name: str,
        arg: Any = None,
        fn: Optional[Callable[..., Any]] = None,
        *,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
    ) -> Union[Output, Callable[[Callable[..., Any]], Output]]:
        """Registers a step in the current job."""
        if fn is None and callable(arg):
            fn = cast(Callable[..., Any], arg)
            arg = None

        def register(registered_fn: Callable[..., Any]) -> Output:
            if not name:
                raise WorkflowError("step name is required")
            if isinstance(arg, PulumiOutput) and not _is_workflow_output(arg):
                raise WorkflowError(
                    "workflow steps may only accept workflow outputs; resource outputs are not supported"
                )

            step_path = f"{self._state.job_path}/steps/{name}"
            request = workflow_pb2.RegisterStepRequest()
            request.context.CopyFrom(self._state.context)
            if hasattr(request, "path"):
                request.path = step_path
            else:
                request.name = name
                request.job = self._state.job_path
            request.has_on_error = on_error is not None
            request.dependencies.operator = (
                workflow_pb2.DependencyExpression.OPERATOR_ALL
            )
            dependency_paths = set()
            if dependencies:
                for dep in dependencies:
                    dependency_paths.add(
                        _normalize_step_dependency(self._state.job_path, dep)
                    )
            dependency_paths.update(_workflow_output_paths(arg))
            for dep in sorted(dependency_paths):
                term = request.dependencies.terms.add()
                term.path = dep

            self._state.monitor.RegisterStep(request)
            self._state.steps[step_path] = _StepDefinition(
                has_arg=arg is not None, arg=arg, fn=registered_fn, on_error=on_error
            )
            return _new_workflow_output(step_path, None)

        if fn is not None:
            return register(fn)
        return register


class WorkflowError(RuntimeError):
    """Raised for invalid workflow runtime usage."""


@dataclass
class TriggerOptions:
    filter: Optional[FilterCallback] = None


class WorkflowRegistry:
    """Collects exported workflow components before evaluation."""

    def __init__(self, package_name: str) -> None:
        self._package_name = package_name
        self._graphs: Dict[str, GraphCallback] = {}
        self._triggers: Dict[str, _TriggerDefinition] = {}

    def graph(
        self,
        name: str,
        fn: Optional[GraphCallback] = None,
    ) -> Union[GraphCallback, Callable[[GraphCallback], GraphCallback]]:
        """Registers a graph export by simple name."""

        def register(registered_fn: GraphCallback) -> GraphCallback:
            if name in self._graphs:
                raise WorkflowError(f"graph '{name}' is already registered")
            self._graphs[name] = registered_fn
            return registered_fn

        if fn is not None:
            return register(fn)
        return register

    def trigger(
        self,
        token: str,
        input_type: Type[TInput],
        mock: TriggerMockCallback[TOutput],
    ) -> None:
        """Registers an exported trigger by token with typed input/output and mock behavior."""
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

    def resolve_trigger_token(self, token: str) -> str:
        if token in self._triggers:
            return token
        qualified = _qualify_trigger_token(self._package_name, token)
        return qualified


def _normalize_job_dependency(graph_path: str, dependency: str) -> str:
    if "/" in dependency:
        return dependency
    return f"{graph_path}/jobs/{dependency}"


def _normalize_step_dependency(job_path: str, dependency: str) -> str:
    if "/" in dependency:
        return dependency
    return f"{job_path}/steps/{dependency}"


def _to_proto_value(value: Any) -> struct_pb2.Value:
    result = struct_pb2.Value()
    if value is None:
        result.null_value = struct_pb2.NullValue.NULL_VALUE
    elif isinstance(value, bool):
        result.bool_value = value
    elif isinstance(value, (int, float)):
        result.number_value = float(value)
    elif isinstance(value, str):
        result.string_value = value
    elif isinstance(value, dict):
        struct_value = struct_pb2.Struct()
        struct_value.update(_coerce_to_struct_data(value))
        result.struct_value.CopyFrom(struct_value)
    elif isinstance(value, list):
        list_value = struct_pb2.ListValue()
        for item in value:
            list_value.values.add().CopyFrom(_to_proto_value(item))
        result.list_value.CopyFrom(list_value)
    elif _is_record_instance(value):
        struct_value = struct_pb2.Struct()
        struct_value.update(_coerce_to_struct_data(value))
        result.struct_value.CopyFrom(struct_value)
    else:
        result.string_value = str(value)
    return result


def _mock_return_type(mock: Callable[..., Any]) -> Type[Any]:
    hints = get_type_hints(mock)
    output_type = hints.get("return")
    if output_type is None:
        raise WorkflowError("trigger mock must declare a return type annotation")
    if not inspect.isclass(output_type):
        raise WorkflowError("trigger mock return type must be a class/record type")
    return output_type


def _validate_record_type(record_type: Type[Any], label: str) -> None:
    if not inspect.isclass(record_type):
        raise WorkflowError(f"{label} must be a class/record type")
    if record_type in (dict, list, str, int, float, bool):
        raise WorkflowError(f"{label} must not be a primitive/container builtin type")
    if not (is_dataclass(record_type) or hasattr(record_type, "__annotations__")):
        raise WorkflowError(f"{label} must define fields via dataclass or annotations")


def _is_record_instance(value: Any) -> bool:
    if is_dataclass(value):
        return True
    if hasattr(value, "__dict__") and hasattr(type(value), "__annotations__"):
        return True
    return False


def _coerce_to_struct_data(value: Any) -> Dict[str, Any]:
    if is_dataclass(value):
        return asdict(value)
    if isinstance(value, dict):
        return dict(value)
    if _is_record_instance(value):
        return dict(vars(value))
    raise WorkflowError("expected a class/record instance or dict for structured trigger data")


def _type_token(record_type: Type[Any]) -> str:
    return f"{record_type.__module__}.{record_type.__qualname__}"


def _qualify_trigger_token(package_name: str, token: str) -> str:
    if ":" in token:
        return token
    return f"{package_name}:index:{token}"


def _from_proto_value(value: struct_pb2.Value) -> Any:
    kind = value.WhichOneof("kind")
    if kind == "null_value":
        return None
    if kind == "number_value":
        return value.number_value
    if kind == "string_value":
        return value.string_value
    if kind == "bool_value":
        return value.bool_value
    if kind == "struct_value":
        return json_format.MessageToDict(value.struct_value)
    if kind == "list_value":
        return [_from_proto_value(item) for item in value.list_value.values]
    return None


def _workflow_input_value(context: workflow_pb2.WorkflowContext, path: str) -> Any:
    if context.input_path != path:
        return None
    if not context.HasField("input_value"):
        return None
    return _from_proto_value(context.input_value)


def _workflow_output_paths(value: Any) -> Set[str]:
    if not isinstance(value, PulumiOutput):
        return set()
    paths = getattr(value, _WORKFLOW_OUTPUT_PATHS_ATTR, None)
    if paths is None:
        return set()
    return set(paths)


def _is_workflow_output(value: Any) -> bool:
    return isinstance(value, PulumiOutput) and len(_workflow_output_paths(value)) > 0


def _workflow_dependency_paths(values: List[Any]) -> Set[str]:
    paths: Set[str] = set()
    for value in values:
        paths.update(_workflow_output_paths(value))
    return paths


def _new_workflow_output(path: str, value: Any) -> PulumiOutput[Any]:
    _ensure_event_loop()
    output = PulumiOutput.from_input(value)
    setattr(output, _WORKFLOW_OUTPUT_PATHS_ATTR, {path})
    setattr(output, _WORKFLOW_OUTPUT_VALUE_ATTR, value)
    return output


def _resolve_job_input(value: Any) -> Any:
    if isinstance(value, PulumiOutput):
        if not _is_workflow_output(value):
            raise WorkflowError(
                "workflow jobs may only accept workflow outputs; resource outputs are not supported"
            )
        if hasattr(value, _WORKFLOW_OUTPUT_VALUE_ATTR):
            return getattr(value, _WORKFLOW_OUTPUT_VALUE_ATTR)
        _ensure_event_loop()
        return _sync_await(value.future(with_unknowns=True))
    return value


def _resolve_step_arg(arg: Any, step_results_by_path: Dict[str, Any]) -> Any:
    if not isinstance(arg, PulumiOutput):
        return arg
    if not _is_workflow_output(arg):
        raise WorkflowError(
            "workflow steps may only accept workflow outputs; resource outputs are not supported"
        )

    paths = sorted(_workflow_output_paths(arg))
    if len(paths) == 0:
        return None
    if len(paths) == 1:
        path = paths[0]
        if path in step_results_by_path:
            return step_results_by_path[path]
        if hasattr(arg, _WORKFLOW_OUTPUT_VALUE_ATTR):
            return getattr(arg, _WORKFLOW_OUTPUT_VALUE_ATTR)
        return None
    return [step_results_by_path.get(path) for path in paths]


def _invoke_step_fn(fn: Callable[..., Any], arg: Any, *, has_arg: bool) -> Any:
    if not has_arg:
        return fn()

    signature = inspect.signature(fn)
    parameters = list(signature.parameters.values())
    has_var_args = any(
        parameter.kind == inspect.Parameter.VAR_POSITIONAL
        for parameter in parameters
    )
    positional = [
        parameter
        for parameter in parameters
        if parameter.kind
        in (inspect.Parameter.POSITIONAL_ONLY, inspect.Parameter.POSITIONAL_OR_KEYWORD)
    ]
    if has_var_args or len(positional) > 0:
        return fn(arg)
    return fn()


def _ensure_event_loop() -> None:
    try:
        asyncio.get_running_loop()
        return
    except RuntimeError:
        pass

    try:
        asyncio.get_event_loop()
    except RuntimeError:
        asyncio.set_event_loop(asyncio.new_event_loop())


def _evaluate_graph(
    token: str,
    registry: WorkflowRegistry,
    graph_fn: GraphCallback,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
    target_job_name: Optional[str] = None,
) -> Tuple[Dict[str, _JobDefinition], Dict[str, FilterCallback]]:
    if not graph_monitor_address:
        raise WorkflowError("graph monitor address is required")

    with contextlib.ExitStack() as stack:
        graph_channel = stack.enter_context(
            grpc.insecure_channel(graph_monitor_address)
        )
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        register_graph = workflow_pb2.RegisterGraphRequest()
        register_graph.context.CopyFrom(context)
        register_graph.path = token
        register_graph.has_on_error = False
        register_graph.dependencies.operator = (
            workflow_pb2.DependencyExpression.OPERATOR_ALL
        )
        monitor.RegisterGraph(register_graph)

        jobs: Dict[str, _JobDefinition] = {}
        filters: Dict[str, FilterCallback] = {}
        graph_fn(
            Context(
                _EvalState(
                    monitor=monitor,
                    context=context,
                    graph_path=token,
                    registry=registry,
                    jobs=jobs,
                    filters=filters,
                    target_job_name=target_job_name,
                )
            )
        )
        return jobs, filters


def _evaluate_job(
    job_path: str,
    job: _JobDefinition,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
) -> Dict[str, _StepDefinition]:
    if not graph_monitor_address:
        raise WorkflowError("graph monitor address is required")

    with contextlib.ExitStack() as stack:
        graph_channel = stack.enter_context(
            grpc.insecure_channel(graph_monitor_address)
        )
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        steps: Dict[str, _StepDefinition] = {}
        job.fn(
            JobContext(
                _JobEvalState(
                    monitor=monitor, context=context, job_path=job_path, steps=steps
                )
            ),
            *job.inputs,
        )
        return steps


class _WorkflowEvaluatorServer(workflow_pb2_grpc.WorkflowEvaluatorServicer):
    def __init__(
        self,
        package_name: str,
        package_version: str,
        register: Callable[[WorkflowRegistry], None],
    ) -> None:
        self._package_name = package_name
        self._package_version = package_version

        self._workflow_registry = WorkflowRegistry(package_name)
        register(self._workflow_registry)

        self._jobs_by_path: Dict[str, _JobDefinition] = {}
        self._steps_by_path: Dict[str, _StepDefinition] = {}
        self._step_results_by_path: Dict[str, Any] = {}
        self._filters_by_path: Dict[str, FilterCallback] = {}

    def Handshake(
        self,
        request: workflow_pb2.WorkflowHandshakeRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.WorkflowHandshakeResponse:
        _ = request
        _ = context
        return workflow_pb2.WorkflowHandshakeResponse()

    def GetPackageInfo(
        self,
        request: workflow_pb2.GetPackageInfoRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetPackageInfoResponse:
        _ = request
        _ = context
        response = workflow_pb2.GetPackageInfoResponse()
        response.package.name = self._package_name
        response.package.version = self._package_version
        response.package.display_name = self._package_name
        return response

    def GetGraphs(
        self,
        request: workflow_pb2.GetGraphsRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetGraphsResponse:
        _ = request
        _ = context
        response = workflow_pb2.GetGraphsResponse()
        for token in self._workflow_registry._graphs:
            graph = response.graphs.add()
            graph.token = token
            graph.has_on_error = False
        return response

    def GetGraph(
        self,
        request: workflow_pb2.GetGraphRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetGraphResponse:
        if request.token not in self._workflow_registry._graphs:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown graph token {request.token}"
            )
        response = workflow_pb2.GetGraphResponse()
        response.graph.token = request.token
        response.graph.has_on_error = False
        return response

    def GetJobs(
        self,
        request: workflow_pb2.GetJobsRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetJobsResponse:
        _ = request
        _ = context
        return workflow_pb2.GetJobsResponse()

    def GetTriggers(
        self,
        request: workflow_pb2.GetTriggersRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetTriggersResponse:
        _ = request
        _ = context
        response = workflow_pb2.GetTriggersResponse()
        for token in sorted(self._workflow_registry._triggers):
            response.triggers.append(token)
        return response

    def GetTrigger(
        self,
        request: workflow_pb2.GetTriggerRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetTriggerResponse:
        resolved_token = self._workflow_registry.resolve_trigger_token(request.token)
        trigger = self._workflow_registry._triggers.get(resolved_token)
        if trigger is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown trigger token {request.token}"
            )

        response = workflow_pb2.GetTriggerResponse()
        response.input_type.token = _type_token(trigger.input_type)
        response.output_type.token = _type_token(trigger.output_type)
        return response

    def GetJob(
        self,
        request: workflow_pb2.GetJobRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetJobResponse:
        _ = request
        context.abort(grpc.StatusCode.NOT_FOUND, "no jobs exported")

    def RunTriggerMock(
        self,
        request: workflow_pb2.RunTriggerMockRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.RunTriggerMockResponse:
        resolved_token = self._workflow_registry.resolve_trigger_token(request.token)
        trigger = self._workflow_registry._triggers.get(resolved_token)
        if trigger is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown trigger token {request.token}"
            )

        response = workflow_pb2.RunTriggerMockResponse()

        try:
            value = trigger.mock(list(request.args))
            if not isinstance(value, trigger.output_type):
                raise WorkflowError(
                    f"trigger mock for {trigger.token} must return {trigger.output_type.__name__}"
                )
            response.value.update(_coerce_to_struct_data(value))
        except Exception as error:  # pylint: disable=broad-except
            context.abort(grpc.StatusCode.INTERNAL, f"trigger mock failed: {error}")
        return response

    def GenerateGraph(
        self,
        request: workflow_pb2.GenerateGraphRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GenerateNodeResponse:
        graph_fn = self._workflow_registry._graphs.get(request.path)
        if graph_fn is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown graph path {request.path}"
            )

        jobs, filters = _evaluate_graph(
            request.path,
            self._workflow_registry,
            graph_fn,
            request.context,
            request.graph_monitor_address,
        )
        self._jobs_by_path.update(jobs)
        self._filters_by_path.update(filters)
        return workflow_pb2.GenerateNodeResponse()

    def GenerateJob(
        self,
        request: workflow_pb2.GenerateJobRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GenerateNodeResponse:
        if not request.graph_monitor_address:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "graph_monitor_address is required"
            )

        segments = request.path.split("/jobs/", 1)
        if len(segments) != 2 or not segments[0] or not segments[1]:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, f"invalid job path {request.path}"
            )
        graph_path = segments[0]
        job_name = segments[1].split("/", 1)[0]

        graph_fn = self._workflow_registry._graphs.get(graph_path)
        if graph_fn is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown graph path {graph_path}")

        jobs, filters = _evaluate_graph(
            graph_path,
            self._workflow_registry,
            graph_fn,
            request.context,
            request.graph_monitor_address,
            target_job_name=job_name,
        )
        self._jobs_by_path.update(jobs)
        self._filters_by_path.update(filters)

        job = self._jobs_by_path.get(request.path)
        if job is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown job path {request.path}")

        steps = _evaluate_job(
            request.path, job, request.context, request.graph_monitor_address
        )
        self._steps_by_path.update(steps)
        return workflow_pb2.GenerateNodeResponse()

    def RunFilter(
        self,
        request: workflow_pb2.RunFilterRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.RunFilterResponse:
        _ = context
        response = workflow_pb2.RunFilterResponse()

        filter_fn = self._filters_by_path.get(request.path)
        if filter_fn is None:
            setattr(response, "pass", True)
            return response

        value = _from_proto_value(request.value) if request.value is not None else None
        setattr(response, "pass", bool(filter_fn(value)))
        return response

    def RunStep(
        self,
        request: workflow_pb2.RunStepRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.RunStepResponse:
        step = self._steps_by_path.get(request.path)
        if step is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown step path {request.path}"
            )

        response = workflow_pb2.RunStepResponse()
        try:
            if step.has_arg:
                arg_value = _resolve_step_arg(step.arg, self._step_results_by_path)
                result = _invoke_step_fn(step.fn, arg_value, has_arg=True)
            else:
                result = _invoke_step_fn(step.fn, None, has_arg=False)
            self._step_results_by_path[request.path] = result
            response.result.CopyFrom(_to_proto_value(result))
        except Exception as error:  # pylint: disable=broad-except
            response.error.reason = str(error)
            response.error.category = "step_failed"
        return response

    def RunOnError(
        self,
        request: workflow_pb2.RunOnErrorRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.RunOnErrorResponse:
        _ = context

        response = workflow_pb2.RunOnErrorResponse()
        step = self._steps_by_path.get(request.path)
        handler = step.on_error if step is not None else None

        if handler is None:
            response.retry = False
            return response

        try:
            decision = handler(list(request.errors))
        except Exception as error:  # pylint: disable=broad-except
            response.error.reason = str(error)
            response.error.category = "on_error_failed"
            return response

        if isinstance(decision, tuple):
            response.retry = bool(decision[0])
            if len(decision) > 1 and decision[1]:
                response.retry_after = str(decision[1])
            return response

        response.retry = bool(decision)
        return response


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
