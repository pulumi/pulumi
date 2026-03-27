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

from __future__ import annotations

import contextlib
from typing import Any, Callable, Dict, Optional, Set, Tuple, cast, get_type_hints

import grpc
from google.protobuf import json_format
from google.protobuf import struct_pb2
from pulumi import Output
from pulumi.runtime.sync_await import _sync_await

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc

from .errors import WorkflowError
from .helpers import (
    _coerce_record_instance,
    _coerce_to_struct_data,
    _coerce_typed_value,
    _ensure_event_loop,
    _from_proto_value,
    _infer_input_value_path_for_job,
    _invoke_job_fn,
    _invoke_step_fn,
    _job_input_properties,
    _resolve_step_arg,
    _to_proto_value,
    _type_token,
    _with_default_workflow_version,
)
from .runtime import (
    Context,
    JobContext,
    WorkflowRegistry,
    _EvalState,
    _ExportedStepDefinition,
    _JobDefinition,
    _JobEvalState,
    _StepDefinition,
    FilterCallback,
    GraphCallback,
)


def _primitive_type_token(t: Any) -> Optional[str]:
    if t is bool:
        return "bool"
    if t is int:
        return "int"
    if t is float:
        return "number"
    if t is str:
        return "string"
    if t is list:
        return "list"
    if t is dict:
        return "object"
    return None


def _populate_type_reference(ref: workflow_pb2.TypeReference, t: Any) -> None:
    primitive = _primitive_type_token(t)
    if primitive is not None:
        ref.token = primitive
        return

    annotations = get_type_hints(t)
    if annotations:
        for name, annotation in annotations.items():
            prop = ref.object.properties[name]
            prop.type = _primitive_type_token(annotation) or "object"
        return

    ref.token = _type_token(t)

def _evaluate_graph(
    token: str,
    registry: WorkflowRegistry,
    graph_fn: GraphCallback,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
    target_job_name: Optional[str] = None,
    input_value_path: str = "",
    input_value: Optional[struct_pb2.Struct] = None,
    default_workflow_version: str = "",
) -> Tuple[Dict[str, _JobDefinition], Dict[str, FilterCallback]]:
    if not graph_monitor_address:
        raise WorkflowError("graph monitor address is required")

    with contextlib.ExitStack() as stack:
        effective_context = _with_default_workflow_version(
            context, default_workflow_version
        )
        graph_channel = stack.enter_context(
            grpc.insecure_channel(graph_monitor_address)
        )
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        register_graph = workflow_pb2.RegisterGraphRequest()
        register_graph.context.CopyFrom(effective_context)
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
                    context=effective_context,
                    graph_path=token,
                    input_value_path=input_value_path,
                    input_value=input_value,
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
    registry: WorkflowRegistry,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
    default_workflow_version: str = "",
) -> Tuple[Dict[str, _StepDefinition], Dict[str, FilterCallback], Output[Any]]:
    if not graph_monitor_address:
        raise WorkflowError("graph monitor address is required")

    with contextlib.ExitStack() as stack:
        effective_context = _with_default_workflow_version(
            context, default_workflow_version
        )
        graph_channel = stack.enter_context(
            grpc.insecure_channel(graph_monitor_address)
        )
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        steps: Dict[str, _StepDefinition] = {}
        filters: Dict[str, FilterCallback] = {}
        if job.fn is None:
            raise WorkflowError(f"job {job_path} has no evaluator callback")
        job_ctx = JobContext(
            _JobEvalState(
                monitor=monitor,
                context=effective_context,
                job_path=job_path,
                steps=steps,
                registry=registry,
                filters=filters,
            )
        )
        _ensure_event_loop()
        job_output = _invoke_job_fn(
            job.fn,
            job_ctx,
        )

        inferred_dependencies: Set[str] = set(job.dependencies)
        for step in steps.values():
            for dep in step.dependency_paths:
                dep_job = dep.split("/steps/", 1)[0] if "/steps/" in dep else dep
                if dep_job and dep_job != job_path:
                    inferred_dependencies.add(dep_job)

        register_job = workflow_pb2.RegisterJobRequest()
        register_job.context.CopyFrom(effective_context)
        register_job.path = job_path
        register_job.has_on_error = job.on_error is not None
        register_job.dependencies.operator = (
            workflow_pb2.DependencyExpression.OPERATOR_ALL
        )
        for dep in sorted(inferred_dependencies):
            term = register_job.dependencies.terms.add()
            term.path = dep
        monitor.RegisterJob(register_job)

        if isinstance(job_output, Output):
            resolved_job_output = cast(Output[Any], job_output)
        elif job_output is None:
            resolved_job_output = Output.from_input(None)
        else:
            resolved_job_output = Output.from_input(job_output)
        return steps, filters, resolved_job_output


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
        self._filters_by_path: Dict[str, FilterCallback] = {}
        self._job_outputs_by_path: Dict[str, Output[Any]] = {}

    def _materialize_graph_job(
        self,
        request_name: str,
        job: _JobDefinition,
    ) -> _JobDefinition:
        if job.external_token is None:
            return job

        exported = self._workflow_registry._jobs.get(job.external_token)
        if exported is None:
            raise WorkflowError(f"unknown external job token {job.external_token}")
        return _JobDefinition(
            fn=lambda job_ctx: exported.fn(job_ctx),
            on_error=job.on_error,
            dependencies=job.dependencies,
            external_token=job.external_token,
        )

    def _materialize_job_steps(
        self,
        request_name: str,
        steps: Dict[str, _StepDefinition],
    ) -> Dict[str, _StepDefinition]:
        materialized: Dict[str, _StepDefinition] = {}
        for step_path, step in steps.items():
            if step.external_token is None:
                materialized[step_path] = step
                continue

            exported = self._workflow_registry._steps.get(step.external_token)
            if exported is None:
                token_parts = step.external_token.split(":")
                fallback = token_parts[-1] if token_parts else step.external_token
                fallback_token = self._workflow_registry.resolve_step_token(fallback)
                exported = self._workflow_registry._steps.get(fallback_token)
            if exported is None:
                raise WorkflowError(f"unknown external step token {step.external_token}")
            if not step.has_arg:
                raise WorkflowError(
                    f"external step {request_name} must have one input argument"
                )

            def _call_exported(
                arg_value: Any,
                *,
                exported_step: _ExportedStepDefinition = exported,
                exported_path: str = step_path,
            ) -> Any:
                coerced = _coerce_typed_value(
                    exported_step.input_type,
                    arg_value,
                    f"step input for {exported_path}",
                )
                result = exported_step.fn(coerced)
                return _coerce_typed_value(
                    exported_step.output_type,
                    result,
                    f"external step {exported_step.token} output",
                )

            materialized[step_path] = _StepDefinition(
                has_arg=True,
                arg=step.arg,
                fn=_call_exported,
                on_error=step.on_error if step.on_error is not None else exported.on_error,
                resolve_output=step.resolve_output,
                dependency_paths=set(step.dependency_paths),
                external_token=step.external_token,
                expected_output_type=exported.output_type,
            )
        return materialized

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
        response = workflow_pb2.GetJobsResponse()
        for token in sorted(self._workflow_registry._jobs):
            job = self._workflow_registry._jobs[token]
            info = response.jobs.add()
            info.token = token
            info.input_type.token = _type_token(job.input_type)
            info.output_type.token = _type_token(job.output_type)
            info.has_on_error = job.on_error is not None
        return response

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
        resolved_token = self._workflow_registry.resolve_job_token(request.token)
        job = self._workflow_registry._jobs.get(resolved_token)
        if job is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown job token {request.token}"
            )
        response = workflow_pb2.GetJobResponse()
        response.job.token = job.token
        _populate_type_reference(response.job.input_type, job.input_type)
        _populate_type_reference(response.job.output_type, job.output_type)
        response.job.has_on_error = job.on_error is not None
        for property_spec in _job_input_properties(job.input_type):
            prop = response.input_properties.add()
            prop.name = str(property_spec["name"])
            prop.type = str(property_spec["type"])
            prop.required = bool(property_spec["required"])
        return response

    def GetSteps(
        self,
        request: workflow_pb2.GetStepsRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetStepsResponse:
        _ = request
        _ = context
        response = workflow_pb2.GetStepsResponse()
        for token in sorted(self._workflow_registry._steps):
            response.steps.append(token)
        return response

    def GetStep(
        self,
        request: workflow_pb2.GetStepRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GetStepResponse:
        resolved_token = self._workflow_registry.resolve_step_token(request.token)
        step = self._workflow_registry._steps.get(resolved_token)
        if step is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown step token {request.token}"
            )
        response = workflow_pb2.GetStepResponse()
        _populate_type_reference(response.input_type, step.input_type)
        _populate_type_reference(response.output_type, step.output_type)
        return response

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
            default_workflow_version=self._package_version,
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

        if request.path:
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
                default_workflow_version=self._package_version,
            )
            input_value = request.input_value if request.HasField("input_value") else None
            if input_value is not None:
                target_job_path = request.path
                target_job = jobs.get(target_job_path)
                if target_job is None:
                    context.abort(
                        grpc.StatusCode.NOT_FOUND,
                        f"unknown job path {target_job_path}",
                    )
                try:
                    input_value_path = _infer_input_value_path_for_job(
                        target_job_path, target_job
                    )
                except WorkflowError as error:
                    context.abort(
                        grpc.StatusCode.INVALID_ARGUMENT,
                        str(error),
                    )
                jobs, filters = _evaluate_graph(
                    graph_path,
                    self._workflow_registry,
                    graph_fn,
                    request.context,
                    request.graph_monitor_address,
                    target_job_name=job_name,
                    input_value_path=input_value_path,
                    input_value=input_value,
                    default_workflow_version=self._package_version,
                )
            self._jobs_by_path.update(jobs)
            self._filters_by_path.update(filters)

            job = self._jobs_by_path.get(request.path)
            if job is None:
                context.abort(grpc.StatusCode.NOT_FOUND, f"unknown job path {request.path}")

            materialized_job = self._materialize_graph_job(request.path, job)
            steps, step_filters, job_output = _evaluate_job(
                request.path,
                materialized_job,
                self._workflow_registry,
                request.context,
                request.graph_monitor_address,
                default_workflow_version=self._package_version,
            )
            self._steps_by_path.update(self._materialize_job_steps(request.path, steps))
            self._filters_by_path.update(step_filters)
            self._job_outputs_by_path[request.path] = job_output
            return workflow_pb2.GenerateNodeResponse()

        if not request.name:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "either path (inline graph job) or name (exported job) is required",
            )

        resolved_token = self._workflow_registry.resolve_job_token(request.name)
        exported = self._workflow_registry._jobs.get(resolved_token)
        if exported is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown job token {request.name}")

        input_value = request.input_value if request.HasField("input_value") else struct_pb2.Struct()
        try:
            coerced_input = _coerce_record_instance(
                exported.input_type,
                json_format.MessageToDict(input_value),
                f"job input for {resolved_token}",
            )
        except WorkflowError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))

        runtime_job_path = request.path if request.path else resolved_token
        synthetic_job = _JobDefinition(
            fn=lambda job_ctx: exported.fn(job_ctx, coerced_input),
            on_error=exported.on_error,
            dependencies=[],
        )
        steps, step_filters, job_output = _evaluate_job(
            runtime_job_path,
            synthetic_job,
            self._workflow_registry,
            request.context,
            request.graph_monitor_address,
            default_workflow_version=self._package_version,
        )
        self._steps_by_path.update(self._materialize_job_steps(runtime_job_path, steps))
        self._filters_by_path.update(step_filters)
        self._job_outputs_by_path[runtime_job_path] = job_output
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
            resolved_token = self._workflow_registry.resolve_step_token(request.path)
            exported = self._workflow_registry._steps.get(resolved_token)
            if exported is not None:
                input_value = _from_proto_value(request.input) if request.HasField("input") else None
                try:
                    coerced_input = _coerce_typed_value(
                        exported.input_type,
                        input_value,
                        f"step input for {resolved_token}",
                    )
                    result_value = exported.fn(coerced_input)
                    result = _coerce_typed_value(
                        exported.output_type,
                        result_value,
                        f"step output for {resolved_token}",
                    )
                except Exception as error:  # pylint: disable=broad-except
                    response = workflow_pb2.RunStepResponse()
                    response.error.reason = str(error)
                    response.error.category = "step_failed"
                    return response

                response = workflow_pb2.RunStepResponse()
                response.result.CopyFrom(_to_proto_value(result))
                return response

            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown step path {request.path}"
            )

        response = workflow_pb2.RunStepResponse()
        try:
            if step.has_arg:
                if request.HasField("input"):
                    arg_value = _from_proto_value(request.input)
                    if (
                        step.external_token is None
                        and isinstance(arg_value, dict)
                        and "input" in arg_value
                        and len(arg_value) == 1
                    ):
                        arg_value = arg_value["input"]
                else:
                    arg_value = _resolve_step_arg(step.arg)
                result = _invoke_step_fn(step.fn, arg_value, has_arg=True)
            else:
                result = _invoke_step_fn(step.fn, None, has_arg=False)
            step.resolve_output(Output.from_input(result))
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

    def ResolveJobResult(
        self,
        request: workflow_pb2.ResolveJobResultRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.ResolveJobResultResponse:
        _ = context

        job_output = self._job_outputs_by_path.get(request.path)
        if job_output is None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, f"unknown job path {request.path}"
            )

        response = workflow_pb2.ResolveJobResultResponse()
        try:
            _ensure_event_loop()
            value = _sync_await(job_output.future(with_unknowns=True))
            response.result.CopyFrom(_to_proto_value(value))
        except Exception as error:  # pylint: disable=broad-except
            response.error.reason = str(error)
            response.error.category = "resolve_job_result_failed"
        return response
