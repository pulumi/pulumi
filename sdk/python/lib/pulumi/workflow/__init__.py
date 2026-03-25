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
StepCallback = Callable[[Any], Any]

_WORKFLOW_OUTPUT_PATHS_ATTR = "_pulumi_workflow_output_paths"
_WORKFLOW_OUTPUT_VALUE_ATTR = "_pulumi_workflow_output_value"


@dataclass
class _StepDefinition:
    has_arg: bool
    arg: Any
    fn: Callable[..., Any]
    on_error: Optional[OnErrorHandler]
    resolve_output: Callable[[Output[Any]], None]
    external_token: Optional[str] = None
    expected_output_type: Optional[Type[Any]] = None


@dataclass
class _JobDefinition:
    fn: Optional[JobCallback]
    on_error: Optional[OnErrorHandler]
    inputs: List[Any]
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
    input_path: str
    input_value: Optional[struct_pb2.Value]
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
                    self._state.input_path, self._state.input_value, trigger_path
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
                self._state.input_path, self._state.input_value, trigger_path
            ),
        )

    def job(
        self,
        name: str,
        *inputs_or_fn: Any,
        fn: Optional[JobCallback] = None,
        dependencies: Optional[List[str]] = None,
        on_error: Optional[OnErrorHandler] = None,
        filter: Optional[Input[bool]] = None,
    ) -> Union[Output[Any], Callable[[JobCallback], JobCallback]]:
        """Registers a job in the current graph."""
        inputs: List[Any] = list(inputs_or_fn)
        options: Optional[JobOptions] = None
        if len(inputs) > 0 and isinstance(inputs[-1], JobOptions):
            options = cast(JobOptions, inputs.pop())

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

        if fn is None and len(inputs) == 1 and callable(inputs[0]):
            fn = inputs[0]
            inputs = []

        is_external_job = fn is None and ":" in name
        registered_name = name
        external_token: Optional[str] = None
        if is_external_job:
            external_token = self._state.registry.resolve_job_token(name)
            if options is not None and options.name:
                registered_name = options.name
            else:
                registered_name = _default_job_name_for_token(name)

        def register(registered_fn: JobCallback) -> Output[Any]:
            if not registered_name:
                raise WorkflowError("job name is required")
            job_path = f"{self._state.graph_path}/jobs/{registered_name}"
            job_output = _new_workflow_output(
                job_path,
                _workflow_input_value(
                    self._state.input_path, self._state.input_value, job_path
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
            dependency_paths = set()
            if effective_dependencies:
                for dep in effective_dependencies:
                    dependency_paths.add(
                        _normalize_job_dependency(self._state.graph_path, dep)
                    )
            dependency_paths.update(_workflow_dependency_paths(inputs))
            for dep in sorted(dependency_paths):
                term = request.dependencies.terms.add()
                term.path = dep

            resolved_inputs = [_resolve_job_input(value) for value in inputs]
            self._state.monitor.RegisterJob(request)
            if effective_filter is not None:
                self._state.filters[job_path] = _input_bool_filter_callback(effective_filter)
            self._state.jobs[job_path] = _JobDefinition(
                fn=registered_fn,
                on_error=effective_on_error,
                inputs=resolved_inputs,
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
        is_external_step = fn is None and ":" in name
        registered_name = name
        external_token: Optional[str] = None
        if is_external_step:
            external_token = self._state.registry.resolve_step_token(name)
            if options is not None and options.name:
                registered_name = options.name
            else:
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
                external_token=external_token,
            )
            return cast(Output[U], step_output)

        if is_external_step:
            return register(cast(Callable[[T], U], lambda _arg=None: None))
        if fn is not None:
            return register(cast(Callable[[T], U], fn))
        return register


class WorkflowError(RuntimeError):
    """Raised for invalid workflow runtime usage."""


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

    def job(
        self,
        token: str,
        input_type: Type[TInput],
        fn: Callable[[JobContext, TInput], Output[TOutput]],
        *,
        on_error: Optional[OnErrorHandler] = None,
    ) -> None:
        if not token:
            raise WorkflowError("job token is required")
        resolved_token = self.resolve_job_token(token)
        if resolved_token in self._jobs:
            raise WorkflowError(f"job '{resolved_token}' is already registered")
        if not callable(fn):
            raise WorkflowError("job callback must be callable")

        _validate_record_type(input_type, "job input type")
        output_type = _job_return_output_type(fn)
        _validate_record_type(output_type, "job output type")

        self._jobs[resolved_token] = _ExportedJobDefinition(
            token=resolved_token,
            input_type=input_type,
            output_type=output_type,
            fn=cast(Callable[[JobContext, Any], Output[Any]], fn),
            on_error=on_error,
        )

    def step(
        self,
        token: str,
        input_type: Type[TInput],
        fn: Callable[[TInput], TOutput],
        *,
        on_error: Optional[OnErrorHandler] = None,
    ) -> None:
        if not token:
            raise WorkflowError("step token is required")
        resolved_token = self.resolve_step_token(token)
        if resolved_token in self._steps:
            raise WorkflowError(f"step '{resolved_token}' is already registered")
        if not callable(fn):
            raise WorkflowError("step callback must be callable")

        _validate_record_type(input_type, "step input type")
        output_type = _step_return_type(fn)
        _validate_record_type(output_type, "step output type")

        self._steps[resolved_token] = _ExportedStepDefinition(
            token=resolved_token,
            input_type=input_type,
            output_type=output_type,
            fn=cast(StepCallback, fn),
            on_error=on_error,
        )

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


def _qualify_job_token(package_name: str, token: str) -> str:
    if token.count(":") >= 2:
        return token
    if token.count(":") == 1:
        package, name = token.split(":", 1)
        return f"{package}:index:{name}"
    return f"{package_name}:index:{token}"


def _default_job_name_for_token(token: str) -> str:
    parts = token.split(":")
    return parts[-1] if len(parts) > 0 else token


def _qualify_step_token(package_name: str, token: str) -> str:
    if token.count(":") >= 2:
        return token
    if token.count(":") == 1:
        package, name = token.split(":", 1)
        return f"{package}:index:{name}"
    return f"{package_name}:index:{token}"


def _default_step_name_for_token(token: str) -> str:
    parts = token.split(":")
    return parts[-1] if len(parts) > 0 else token


def _job_return_output_type(fn: Callable[..., Any]) -> Type[Any]:
    hints = get_type_hints(fn)
    return_type = hints.get("return")
    if return_type is None:
        raise WorkflowError("job callback must declare a return type annotation")
    origin = get_origin(return_type)
    if origin is not Output:
        raise WorkflowError("job callback return type must be Output[T]")
    args = get_args(return_type)
    if len(args) != 1:
        raise WorkflowError("job callback return type must be Output[T]")
    output_type = args[0]
    if not inspect.isclass(output_type):
        raise WorkflowError("job callback output type T must be a class/record type")
    return cast(Type[Any], output_type)


def _step_return_type(fn: Callable[..., Any]) -> Type[Any]:
    hints = get_type_hints(fn)
    return_type = hints.get("return")
    if return_type is None:
        raise WorkflowError("step callback must declare a return type annotation")
    if get_origin(return_type) is Output:
        raise WorkflowError("step callback return type must be plain T, not Output[T]")
    if not inspect.isclass(return_type):
        raise WorkflowError("step callback output type must be a class/record type")
    return cast(Type[Any], return_type)


def _coerce_record_instance(record_type: Type[Any], value: Any, label: str) -> Any:
    if isinstance(value, record_type):
        return value
    if isinstance(value, dict):
        annotations = get_type_hints(record_type)
        normalized = dict(value)
        for field_name, field_type in annotations.items():
            if field_name not in normalized:
                continue
            field_value = normalized[field_name]
            if field_type is int and isinstance(field_value, float):
                if field_value.is_integer():
                    normalized[field_name] = int(field_value)
                else:
                    raise WorkflowError(
                        f"invalid {label}: field '{field_name}' requires int, got non-integral float"
                    )
        try:
            return record_type(**normalized)
        except TypeError as error:
            raise WorkflowError(f"invalid {label}: {error}") from error
    raise WorkflowError(
        f"{label} must decode to {record_type.__name__} (got {type(value).__name__})"
    )


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


def _workflow_input_value(
    input_path: str, input_value: Optional[struct_pb2.Value], path: str
) -> Any:
    if input_path != path:
        return None
    if input_value is None:
        return None
    return _from_proto_value(input_value)


def _workflow_output_paths(value: Any) -> Set[str]:
    if not isinstance(value, Output):
        return set()
    paths = _get_output_internal_attr(value, _WORKFLOW_OUTPUT_PATHS_ATTR)
    if paths is None:
        return set()
    return set(paths)


def _is_workflow_output(value: Any) -> bool:
    return isinstance(value, Output) and len(_workflow_output_paths(value)) > 0


def _workflow_dependency_paths(values: List[Any]) -> Set[str]:
    paths: Set[str] = set()
    for value in values:
        paths.update(_workflow_output_paths(value))
    return paths


def _new_workflow_output(path: str, value: Any) -> Output[Any]:
    _ensure_event_loop()
    output = Output.from_input(value)
    setattr(output, _WORKFLOW_OUTPUT_PATHS_ATTR, {path})
    setattr(output, _WORKFLOW_OUTPUT_VALUE_ATTR, value)
    return output


def _resolve_job_input(value: Any) -> Any:
    if isinstance(value, Output):
        if not _is_workflow_output(value):
            raise WorkflowError(
                "workflow jobs may only accept workflow outputs; resource outputs are not supported"
            )
        embedded = _get_output_internal_attr(value, _WORKFLOW_OUTPUT_VALUE_ATTR)
        if embedded is not None:
            return embedded
        _ensure_event_loop()
        return _sync_await(value.future(with_unknowns=True))
    return value


def _input_bool_filter_callback(value: Input[bool]) -> FilterCallback:
    def callback(_unused: Any) -> bool:
        resolved = _resolve_filter_input(value)
        if isinstance(resolved, bool):
            return resolved
        raise WorkflowError(f"filter must resolve to bool (got {type(resolved).__name__})")

    return callback


def _resolve_filter_input(value: Any) -> Any:
    if isinstance(value, Output):
        _ensure_event_loop()
        return _sync_await(value.future(with_unknowns=True))
    return value


def _get_output_internal_attr(output: Output[Any], name: str) -> Any:
    try:
        return object.__getattribute__(output, name)
    except AttributeError:
        return None


def _resolve_step_arg(arg: Any) -> Any:
    if not isinstance(arg, Output):
        return arg
    if not _is_workflow_output(arg):
        raise WorkflowError(
            "workflow steps may only accept workflow outputs; resource outputs are not supported"
        )
    _ensure_event_loop()
    return _sync_await(arg.future(with_unknowns=True))


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


def _with_default_workflow_version(
    context: workflow_pb2.WorkflowContext, default_version: str
) -> workflow_pb2.WorkflowContext:
    effective = workflow_pb2.WorkflowContext()
    effective.CopyFrom(context)
    if not effective.workflow_version:
        effective.workflow_version = default_version
    return effective


def _evaluate_graph(
    token: str,
    registry: WorkflowRegistry,
    graph_fn: GraphCallback,
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
    target_job_name: Optional[str] = None,
    input_path: str = "",
    input_value: Optional[struct_pb2.Value] = None,
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
                    input_path=input_path,
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
        job_output = job.fn(
            JobContext(
                _JobEvalState(
                    monitor=monitor,
                    context=effective_context,
                    job_path=job_path,
                    steps=steps,
                    registry=registry,
                    filters=filters,
                )
            ),
            *job.inputs,
        )
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
        if len(job.inputs) != 1:
            raise WorkflowError(
                f"external graph job {request_name} must have exactly one input argument"
            )

        coerced_input = _coerce_record_instance(
            exported.input_type,
            job.inputs[0],
            f"job input for {request_name}",
        )
        return _JobDefinition(
            fn=lambda job_ctx: exported.fn(job_ctx, coerced_input),
            on_error=job.on_error,
            inputs=[],
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
                raise WorkflowError(
                    f"unknown external step token {step.external_token}"
                )
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
                coerced = _coerce_record_instance(
                    exported_step.input_type,
                    arg_value,
                    f"step input for {exported_path}",
                )
                result = exported_step.fn(coerced)
                if not isinstance(result, exported_step.output_type):
                    raise WorkflowError(
                        f"external step {exported_step.token} must return {exported_step.output_type.__name__}"
                    )
                return result

            materialized[step_path] = _StepDefinition(
                has_arg=True,
                arg=step.arg,
                fn=_call_exported,
                on_error=step.on_error if step.on_error is not None else exported.on_error,
                resolve_output=step.resolve_output,
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
        response.job.input_type.token = _type_token(job.input_type)
        response.job.output_type.token = _type_token(job.output_type)
        response.job.has_on_error = job.on_error is not None
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
                input_path=request.input_path,
                input_value=request.input_value if request.HasField("input_value") else None,
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

        if request.input_path and request.input_path != request.name:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "input_path for exported jobs must match request.name",
            )
        input_value = (
            _from_proto_value(request.input_value)
            if request.HasField("input_value")
            else None
        )
        coerced_input = _coerce_record_instance(
            exported.input_type, input_value, f"job input for {request.name}"
        )

        synthetic_job = _JobDefinition(
            fn=lambda job_ctx: exported.fn(job_ctx, coerced_input),
            on_error=exported.on_error,
            inputs=[],
        )
        steps, step_filters, job_output = _evaluate_job(
            resolved_token,
            synthetic_job,
            self._workflow_registry,
            request.context,
            request.graph_monitor_address,
            default_workflow_version=self._package_version,
        )
        self._steps_by_path.update(self._materialize_job_steps(request.name, steps))
        self._filters_by_path.update(step_filters)
        self._job_outputs_by_path[resolved_token] = job_output
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
