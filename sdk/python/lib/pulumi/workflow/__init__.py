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

This module is intentionally tiny: enough to register exported graphs and
register graph/trigger shape with a GraphMonitor.
"""

from __future__ import annotations

import contextlib
import os
from dataclasses import dataclass
from typing import Any, Callable, Dict, Optional, Union

import grpc
from google.protobuf import struct_pb2

from pulumi.runtime.proto import workflow_pb2
from pulumi.runtime.proto import workflow_pb2_grpc


@dataclass
class _EvalState:
    monitor: workflow_pb2_grpc.GraphMonitorStub
    context: workflow_pb2.WorkflowContext
    graph_path: str


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
    ) -> None:
        """Registers a trigger in the current graph."""

        request = workflow_pb2.RegisterTriggerRequest()
        request.context.CopyFrom(self._state.context)
        request.path = f"{self._state.graph_path}/triggers/{name}"
        request.type = trigger_type
        request.hasFilter = has_filter
        if spec:
            trigger_spec = struct_pb2.Struct()
            trigger_spec.update(spec)
            request.spec.CopyFrom(trigger_spec)

        self._state.monitor.RegisterTrigger(request)


class WorkflowError(RuntimeError):
    """Raised for invalid workflow runtime usage."""


class WorkflowRegistry:
    """Collects exported workflow components before evaluation."""

    def __init__(self) -> None:
        self._graphs: Dict[str, Callable[[Context], None]] = {}

    def graph(
        self,
        name: str,
        fn: Optional[Callable[[Context], None]] = None,
    ) -> Union[Callable[[Context], None], Callable[[Callable[[Context], None]], Callable[[Context], None]]]:
        """Registers a graph export by name/token."""

        def register(registered_fn: Callable[[Context], None]) -> Callable[[Context], None]:
            if name in self._graphs:
                raise WorkflowError(f"graph '{name}' is already registered")
            self._graphs[name] = registered_fn
            return registered_fn

        if fn is not None:
            return register(fn)
        return register


def run(register: Callable[[WorkflowRegistry], None]) -> None:
    """Executes workflow registration + graph evaluation against monitor services."""

    graph_monitor_address = os.getenv("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS")
    if not graph_monitor_address:
        raise WorkflowError("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS is required")

    workflow_name = os.getenv("PULUMI_WORKFLOW_NAME", "workflow")
    workflow_version = os.getenv("PULUMI_WORKFLOW_VERSION", "dev")
    execution_id = os.getenv("PULUMI_WORKFLOW_EXECUTION_ID", "")

    context = workflow_pb2.WorkflowContext()
    context.workflowName = workflow_name
    context.workflowVersion = workflow_version
    context.executionId = execution_id

    with contextlib.ExitStack() as stack:
        graph_channel = stack.enter_context(grpc.insecure_channel(graph_monitor_address))

        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)

        workflow_registry = WorkflowRegistry()
        register(workflow_registry)

        for token, graph_fn in workflow_registry._graphs.items():
            register_graph = workflow_pb2.RegisterGraphRequest()
            register_graph.context.CopyFrom(context)
            register_graph.path = token
            register_graph.hasOnError = False
            register_graph.dependencies.operator = workflow_pb2.DependencyExpression.OPERATOR_ALL
            monitor.RegisterGraph(register_graph)

            graph_fn(Context(_EvalState(monitor=monitor, context=context, graph_path=token)))


__all__ = [
    "Context",
    "WorkflowRegistry",
    "WorkflowError",
    "run",
]
