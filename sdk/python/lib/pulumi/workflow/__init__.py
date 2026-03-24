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
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Union

import grpc
import pulumi
from google.protobuf import json_format
from google.protobuf import struct_pb2
from pulumi.runtime.sync_await import _sync_await

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc

OnErrorHandler = Callable[
    [List[workflow_pb2.ErrorRecord]], Union[bool, Tuple[bool, str], None]
]
StepCallback = Callable[[], Any]
JobCallback = Callable[..., None]
GraphCallback = Callable[["Context"], None]
Output = pulumi.Output[Any]

_WORKFLOW_OUTPUT_PATHS_ATTR = "_pulumi_workflow_output_paths"
_WORKFLOW_OUTPUT_VALUE_ATTR = "_pulumi_workflow_output_value"
_OUTPUT_PATCHED = False


@dataclass
class _StepDefinition:
    fn: StepCallback
    on_error: Optional[OnErrorHandler]


@dataclass
class _JobDefinition:
    fn: JobCallback
    on_error: Optional[OnErrorHandler]
    inputs: List[Any]


@dataclass
class _EvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    graph_path: str
    jobs: Dict[str, _JobDefinition]
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
        spec: Optional[Dict[str, Any]] = None,
        *,
        has_filter: bool = False,
    ) -> pulumi.Output[Any]:
        """Registers a trigger in the current graph."""

        request = workflow_pb2.RegisterTriggerRequest()
        trigger_path = f"{self._state.graph_path}/{name}"
        if self._state.target_job_name is not None:
            return _new_workflow_output(
                trigger_path, _workflow_input_value(self._state.context, trigger_path)
            )
        request.context.CopyFrom(self._state.context)
        request.path = trigger_path
        request.type = trigger_type
        request.has_filter = has_filter
        if spec:
            trigger_spec = struct_pb2.Struct()
            trigger_spec.update(spec)
            request.spec.CopyFrom(trigger_spec)

        self._state.monitor.RegisterTrigger(request)
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
        fn: Optional[StepCallback] = None,
        *,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
    ) -> Union[StepCallback, Callable[[StepCallback], StepCallback]]:
        def register(registered_fn: StepCallback) -> StepCallback:
            if not name:
                raise WorkflowError("step name is required")

            step_path = f"{self._state.job_path}/steps/{name}"

            request = workflow_pb2.RegisterStepRequest()
            request.context.CopyFrom(self._state.context)
            request.path = step_path
            request.job_path = self._state.job_path
            request.has_on_error = on_error is not None
            request.dependencies.operator = (
                workflow_pb2.DependencyExpression.OPERATOR_ALL
            )
            if dependencies:
                for dep in dependencies:
                    term = request.dependencies.terms.add()
                    term.path = _normalize_step_dependency(self._state.job_path, dep)

            self._state.monitor.RegisterStep(request)
            self._state.steps[step_path] = _StepDefinition(
                fn=registered_fn, on_error=on_error
            )
            return registered_fn

        if fn is not None:
            return register(fn)
        return register


class WorkflowError(RuntimeError):
    """Raised for invalid workflow runtime usage."""


class WorkflowRegistry:
    """Collects exported workflow components before evaluation."""

    def __init__(self) -> None:
        self._graphs: Dict[str, GraphCallback] = {}

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
        struct_value.update(value)
        result.struct_value.CopyFrom(struct_value)
    elif isinstance(value, list):
        list_value = struct_pb2.ListValue()
        for item in value:
            list_value.values.add().CopyFrom(_to_proto_value(item))
        result.list_value.CopyFrom(list_value)
    else:
        result.string_value = str(value)
    return result


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
    if not isinstance(value, pulumi.Output):
        return set()
    paths = getattr(value, _WORKFLOW_OUTPUT_PATHS_ATTR, None)
    if paths is None:
        return set()
    return set(paths)


def _is_workflow_output(value: Any) -> bool:
    return isinstance(value, pulumi.Output) and len(_workflow_output_paths(value)) > 0


def _workflow_dependency_paths(values: List[Any]) -> Set[str]:
    paths: Set[str] = set()
    for value in values:
        paths.update(_workflow_output_paths(value))
    return paths


def _new_workflow_output(path: str, value: Any) -> pulumi.Output[Any]:
    output = pulumi.Output.from_input(value)
    setattr(output, _WORKFLOW_OUTPUT_PATHS_ATTR, {path})
    setattr(output, _WORKFLOW_OUTPUT_VALUE_ATTR, value)
    return output


def _resolve_job_input(value: Any) -> Any:
    if isinstance(value, pulumi.Output):
        if not _is_workflow_output(value):
            raise WorkflowError(
                "workflow jobs may only accept workflow outputs; resource outputs are not supported"
            )
        if hasattr(value, _WORKFLOW_OUTPUT_VALUE_ATTR):
            return getattr(value, _WORKFLOW_OUTPUT_VALUE_ATTR)
        return _sync_await(value.future(with_unknowns=True))
    return value


def _patch_output_apis_for_workflow() -> None:
    global _OUTPUT_PATCHED
    if _OUTPUT_PATCHED:
        return

    original_all = pulumi.Output.all
    original_apply = pulumi.Output.apply

    def workflow_safe_all(*args: Any, **kwargs: Any) -> pulumi.Output[Any]:
        values = list(args) + list(kwargs.values())
        has_workflow_output = any(_is_workflow_output(v) for v in values)
        has_nonworkflow_output = any(
            isinstance(v, pulumi.Output) and not _is_workflow_output(v) for v in values
        )
        if has_workflow_output and has_nonworkflow_output:
            raise WorkflowError("cannot mix workflow outputs with resource outputs")

        result = original_all(*args, **kwargs)
        if has_workflow_output:
            paths = _workflow_dependency_paths(values)
            setattr(result, _WORKFLOW_OUTPUT_PATHS_ATTR, paths)
        return result

    def workflow_safe_apply(
        self: pulumi.Output[Any],
        func: Callable[[Any], Any],
        run_with_unknowns: bool = False,
    ) -> pulumi.Output[Any]:
        result = original_apply(self, func, run_with_unknowns)
        if _is_workflow_output(self):
            setattr(result, _WORKFLOW_OUTPUT_PATHS_ATTR, _workflow_output_paths(self))
        return result

    pulumi.Output.all = staticmethod(workflow_safe_all)  # type: ignore[assignment]
    pulumi.Output.apply = workflow_safe_apply  # type: ignore[assignment]
    _OUTPUT_PATCHED = True


def _evaluate_graph(
    token: str,
    graph_fn: GraphCallback,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
    target_job_name: Optional[str] = None,
) -> Dict[str, _JobDefinition]:
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
        graph_fn(
            Context(
                _EvalState(
                    monitor=monitor,
                    context=context,
                    graph_path=token,
                    jobs=jobs,
                    target_job_name=target_job_name,
                )
            )
        )
        return jobs


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

        self._workflow_registry = WorkflowRegistry()
        register(self._workflow_registry)

        self._jobs_by_path: Dict[str, _JobDefinition] = {}
        self._steps_by_path: Dict[str, _StepDefinition] = {}

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

    def GetJob(
        self,
        request: workflow_pb2.GetJobRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetJobResponse:
        _ = request
        context.abort(grpc.StatusCode.NOT_FOUND, "no jobs exported")

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

        jobs = _evaluate_graph(
            request.path, graph_fn, request.context, request.graph_monitor_address
        )
        self._jobs_by_path.update(jobs)
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

        jobs = _evaluate_graph(
            graph_path,
            graph_fn,
            request.context,
            request.graph_monitor_address,
            target_job_name=job_name,
        )
        self._jobs_by_path.update(jobs)

        job = self._jobs_by_path.get(request.path)
        if job is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown job path {request.path}")

        steps = _evaluate_job(
            request.path, job, request.context, request.graph_monitor_address
        )
        self._steps_by_path.update(steps)
        return workflow_pb2.GenerateNodeResponse()

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
            result = step.fn()
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

    _patch_output_apis_for_workflow()

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
    "WorkflowRegistry",
    "WorkflowError",
    "Output",
    "run",
]
