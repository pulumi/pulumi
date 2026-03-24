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

This module is intentionally tiny: enough to register exported graphs with a
WorkflowRegistry and register graph/trigger shape with a GraphMonitor.
"""

from __future__ import annotations

import contextlib
import os
from dataclasses import dataclass
from typing import Any, Callable, Dict, Optional

import grpc
from google.protobuf import struct_pb2

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc


@dataclass
class _EvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    graph_path: str


_graphs: Dict[str, Callable[["Context"], None]] = {}
_active_eval: Optional[_EvalState] = None


class Context:
    """Execution/evaluation context passed to graph functions."""


class WorkflowError(RuntimeError):
    """Raised for invalid workflow runtime usage."""


def graph(name: str) -> Callable[[Callable[[Context], None]], Callable[[Context], None]]:
    """Registers a graph export by name/token."""

    def decorator(fn: Callable[[Context], None]) -> Callable[[Context], None]:
        if name in _graphs:
            raise WorkflowError(f"graph '{name}' is already registered")
        _graphs[name] = fn
        return fn

    return decorator


def trigger(name: str, trigger_type: str, spec: Optional[Dict[str, Any]] = None, *, has_filter: bool = False) -> None:
    """Registers a trigger in the currently-evaluated graph."""

    if _active_eval is None:
        raise WorkflowError("trigger() must be called while a graph is being evaluated via run()")

    request = workflow_pb2.RegisterTriggerRequest()
    request.context.CopyFrom(_active_eval.context)
    request.path = f"{_active_eval.graph_path}/triggers/{name}"
    request.type = trigger_type
    request.hasFilter = has_filter
    if spec:
        trigger_spec = struct_pb2.Struct()
        trigger_spec.update(spec)
        request.spec.CopyFrom(trigger_spec)

    _active_eval.monitor.RegisterTrigger(request)


def run() -> None:
    """Executes workflow registration + graph evaluation against monitor services."""

    registry_address = os.getenv("PULUMI_WORKFLOW_REGISTRY_ADDRESS")
    graph_monitor_address = os.getenv("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS")
    if not registry_address:
        raise WorkflowError("PULUMI_WORKFLOW_REGISTRY_ADDRESS is required")
    if not graph_monitor_address:
        raise WorkflowError("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS is required")

    engine_address = os.getenv("PULUMI_WORKFLOW_ENGINE_ADDRESS", "")
    workflow_name = os.getenv("PULUMI_WORKFLOW_NAME", "workflow")
    workflow_version = os.getenv("PULUMI_WORKFLOW_VERSION", "dev")
    execution_id = os.getenv("PULUMI_WORKFLOW_EXECUTION_ID", "")
    root_directory = os.getenv("PULUMI_WORKFLOW_ROOT_DIRECTORY")
    program_directory = os.getenv("PULUMI_WORKFLOW_PROGRAM_DIRECTORY")
    graph_monitor_context_token = os.getenv("PULUMI_WORKFLOW_GRAPH_MONITOR_CONTEXT_TOKEN", "")

    context = workflow_pb2.WorkflowContext()
    context.workflowName = workflow_name
    context.workflowVersion = workflow_version
    context.executionId = execution_id

    with contextlib.ExitStack() as stack:
        registry_channel = stack.enter_context(grpc.insecure_channel(registry_address))
        graph_channel = stack.enter_context(grpc.insecure_channel(graph_monitor_address))

        registry = workflow_pb2_grpc.WorkflowRegistryStub(registry_channel)
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        handshake = workflow_pb2.WorkflowRegistryHandshakeRequest()
        handshake.engine_address = engine_address
        handshake.graph_monitor_address = graph_monitor_address
        handshake.graph_monitor_context_token = graph_monitor_context_token
        if root_directory:
            handshake.root_directory = root_directory
        if program_directory:
            handshake.program_directory = program_directory
        registry.Handshake(handshake)

        for token in _graphs:
            register = workflow_pb2.RegisterComponentRequest()
            register.context.CopyFrom(context)
            register.token = token
            register.kind = workflow_pb2.WORKFLOW_COMPONENT_KIND_GRAPH
            register.metadata.CopyFrom(struct_pb2.Struct())
            registry.RegisterComponent(register)

        global _active_eval
        for token, graph_fn in _graphs.items():
            register_graph = workflow_pb2.RegisterGraphRequest()
            register_graph.context.CopyFrom(context)
            register_graph.path = token
            register_graph.hasOnError = False
            register_graph.dependencies.operator = workflow_pb2.DependencyExpression.OPERATOR_ALL
            monitor.RegisterGraph(register_graph)

            _active_eval = _EvalState(monitor=monitor, context=context, graph_path=token)
            try:
                graph_fn(Context())
            finally:
                _active_eval = None


__all__ = [
    "Context",
    "WorkflowError",
    "graph",
    "trigger",
    "run",
]
