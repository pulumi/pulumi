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

from dataclasses import asdict, is_dataclass
import asyncio
import inspect
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Type, Union, cast, get_args, get_origin, get_type_hints

from google.protobuf import json_format
from google.protobuf import struct_pb2
from pulumi import Output
from pulumi.runtime.sync_await import _sync_await

from .errors import WorkflowError
from pulumi.runtime.proto import workflow_pb2

_WORKFLOW_OUTPUT_PATHS_ATTR = "_pulumi_workflow_output_paths"
_WORKFLOW_OUTPUT_VALUE_ATTR = "_pulumi_workflow_output_value"
_WORKFLOW_OUTPUT_MARKER_ATTR = "_pulumi_workflow_output_marker"
_WORKFLOW_EXTERNAL_JOB_TOKEN_ATTR = "_pulumi_workflow_external_job_token"
_WORKFLOW_EXTERNAL_STEP_TOKEN_ATTR = "_pulumi_workflow_external_step_token"


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
    if record_type is bool:
        return "bool"
    if record_type is int:
        return "int"
    if record_type is float:
        return "float"
    if record_type is str:
        return "string"
    if record_type is list:
        return "list"
    if record_type is dict:
        return "object"
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


def _external_job_token(value: Any) -> Optional[str]:
    if value is None:
        return None
    token = getattr(value, _WORKFLOW_EXTERNAL_JOB_TOKEN_ATTR, None)
    if isinstance(token, str) and token != "":
        return token
    return None


def _external_step_token(value: Any) -> Optional[str]:
    if value is None:
        return None
    token = getattr(value, _WORKFLOW_EXTERNAL_STEP_TOKEN_ATTR, None)
    if isinstance(token, str) and token != "":
        return token
    return None


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
        raise WorkflowError("job callback output type T must be a class/record or primitive type")
    return cast(Type[Any], output_type)


def _job_input_properties(record_type: Type[Any]) -> List[Dict[str, Any]]:
    annotations = get_type_hints(record_type)
    defaults: Dict[str, bool] = {}
    if is_dataclass(record_type):
        from dataclasses import MISSING, fields

        for field in fields(record_type):
            has_default = field.default is not MISSING or field.default_factory is not MISSING
            defaults[field.name] = has_default

    properties: List[Dict[str, Any]] = []
    for name, annotation in annotations.items():
        property_type, optional = _annotation_to_property_type(annotation)
        required = not optional and not defaults.get(name, False)
        properties.append(
            {
                "name": name,
                "type": property_type,
                "required": required,
            }
        )
    return properties


def _annotation_to_property_type(annotation: Any) -> Tuple[str, bool]:
    origin = get_origin(annotation)
    if origin is Union:
        args = get_args(annotation)
        non_none = [arg for arg in args if arg is not type(None)]
        if len(non_none) == 1 and len(non_none) != len(args):
            property_type, _ = _annotation_to_property_type(non_none[0])
            return property_type, True

    if annotation is str:
        return "string", False
    if annotation is int:
        return "integer", False
    if annotation is float:
        return "number", False
    if annotation is bool:
        return "boolean", False
    return "object", False


def _step_return_type(fn: Callable[..., Any]) -> Type[Any]:
    hints = get_type_hints(fn)
    return_type = hints.get("return")
    if return_type is None:
        raise WorkflowError("step callback must declare a return type annotation")
    if get_origin(return_type) is Output:
        raise WorkflowError("step callback return type must be plain T, not Output[T]")
    if not inspect.isclass(return_type):
        raise WorkflowError(
            "step callback output type must be a class/record or primitive type"
        )
    return cast(Type[Any], return_type)


def _coerce_record_instance(record_type: Type[Any], value: Any, label: str) -> Any:
    if value is None:
        try:
            return record_type()
        except TypeError:
            pass
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


def _is_primitive_type(t: Type[Any]) -> bool:
    return t in (bool, int, float, str, list, dict)


def _validate_step_type(type_value: Type[Any], label: str) -> None:
    if _is_primitive_type(type_value):
        return
    _validate_record_type(type_value, label)


def _coerce_typed_value(expected_type: Type[Any], value: Any, label: str) -> Any:
    if _is_primitive_type(expected_type):
        if expected_type is bool:
            if isinstance(value, bool):
                return value
            raise WorkflowError(f"{label} must decode to bool (got {type(value).__name__})")
        if expected_type is int:
            if isinstance(value, bool):
                raise WorkflowError(f"{label} must decode to int (got bool)")
            if isinstance(value, int):
                return value
            if isinstance(value, float) and value.is_integer():
                return int(value)
            raise WorkflowError(f"{label} must decode to int (got {type(value).__name__})")
        if expected_type is float:
            if isinstance(value, bool):
                raise WorkflowError(f"{label} must decode to float (got bool)")
            if isinstance(value, (int, float)):
                return float(value)
            raise WorkflowError(f"{label} must decode to float (got {type(value).__name__})")
        if expected_type is str:
            if isinstance(value, str):
                return value
            raise WorkflowError(f"{label} must decode to str (got {type(value).__name__})")
        if expected_type is list:
            if isinstance(value, list):
                return value
            raise WorkflowError(f"{label} must decode to list (got {type(value).__name__})")
        if expected_type is dict:
            if isinstance(value, dict):
                return value
            raise WorkflowError(f"{label} must decode to dict (got {type(value).__name__})")
    return _coerce_record_instance(expected_type, value, label)


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
    input_value_path: str, input_value: Optional[struct_pb2.Struct], path: str
) -> Any:
    if input_value_path != path:
        return None
    if input_value is None:
        return None
    return json_format.MessageToDict(input_value)


def _workflow_output_paths(value: Any) -> Set[str]:
    if not isinstance(value, Output):
        return set()
    paths = _get_output_internal_attr(value, _WORKFLOW_OUTPUT_PATHS_ATTR)
    if paths is None:
        return set()
    return set(paths)


def _is_workflow_output(value: Any) -> bool:
    if not isinstance(value, Output):
        return False
    marker = _get_output_internal_attr(value, _WORKFLOW_OUTPUT_MARKER_ATTR)
    if marker is True:
        return True
    return len(_workflow_output_paths(value)) > 0


def _workflow_dependency_paths(values: List[Any]) -> Set[str]:
    paths: Set[str] = set()
    for value in values:
        paths.update(_workflow_output_paths(value))
    return paths


def _infer_input_value_path_for_job(job_path: str, job: Any) -> str:
    candidates: Set[str] = set(job.dependencies)
    if job.fn is not None:
        try:
            closure = inspect.getclosurevars(job.fn)
            for value in closure.nonlocals.values():
                candidates.update(_workflow_output_paths(value))
        except TypeError:
            pass

    if len(candidates) == 1:
        return next(iter(candidates))
    if len(candidates) == 0:
        return job_path
    raise WorkflowError("input_value for graph jobs with multiple dependencies is ambiguous")


def _new_workflow_output(path: str, value: Any) -> Output[Any]:
    _ensure_event_loop()
    output = Output.from_input(value)
    setattr(output, _WORKFLOW_OUTPUT_PATHS_ATTR, {path})
    setattr(output, _WORKFLOW_OUTPUT_VALUE_ATTR, value)
    setattr(output, _WORKFLOW_OUTPUT_MARKER_ATTR, True)
    return output


def _input_bool_filter_callback(value: Any) -> Callable[[Any], bool]:
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


def _invoke_job_fn(
    fn: Callable[..., Any],
    job_ctx: Any,
) -> Any:
    return fn(job_ctx)


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
