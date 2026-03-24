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
from concurrent import futures
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
        request.has_filter = has_filter
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


def _new_workflow_context() -> workflow_pb2.WorkflowContext:
    workflow_name = os.getenv("PULUMI_WORKFLOW_NAME", "workflow")
    workflow_version = os.getenv("PULUMI_WORKFLOW_VERSION", "dev")
    execution_id = os.getenv("PULUMI_WORKFLOW_EXECUTION_ID", "")

    context = workflow_pb2.WorkflowContext()
    context.workflow_name = workflow_name
    context.workflow_version = workflow_version
    context.execution_id = execution_id
    return context


def _evaluate_graph(
    token: str,
    graph_fn: Callable[[Context], None],
    context: workflow_pb2.WorkflowContext,
    graph_monitor_address: str,
) -> None:
    if not graph_monitor_address:
        raise WorkflowError("graph monitor address is required")

    with contextlib.ExitStack() as stack:
        graph_channel = stack.enter_context(grpc.insecure_channel(graph_monitor_address))
        monitor = workflow_pb2_grpc.GraphMonitorStub(graph_channel)
        register_graph = workflow_pb2.RegisterGraphRequest()
        register_graph.context.CopyFrom(context)
        register_graph.path = token
        register_graph.has_on_error = False
        register_graph.dependencies.operator = workflow_pb2.DependencyExpression.OPERATOR_ALL
        monitor.RegisterGraph(register_graph)
        graph_fn(Context(_EvalState(monitor=monitor, context=context, graph_path=token)))


class _WorkflowEvaluatorServer(workflow_pb2_grpc.WorkflowEvaluatorServicer):
    def __init__(self, register: Callable[[WorkflowRegistry], None]) -> None:
        self._workflow_registry = WorkflowRegistry()
        register(self._workflow_registry)

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
        response.package.name = os.getenv("PULUMI_WORKFLOW_PACKAGE_NAME", os.getenv("PULUMI_WORKFLOW_NAME", "workflow"))
        response.package.version = os.getenv("PULUMI_WORKFLOW_PACKAGE_VERSION", os.getenv("PULUMI_WORKFLOW_VERSION", "dev"))
        response.package.display_name = os.getenv("PULUMI_WORKFLOW_PACKAGE_DISPLAY_NAME", "Workflow")
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
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown graph token {request.token}")
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
            context.abort(grpc.StatusCode.NOT_FOUND, f"unknown graph path {request.path}")
        _evaluate_graph(request.path, graph_fn, request.context, request.graph_monitor_address)
        return workflow_pb2.GenerateNodeResponse()

    def GenerateJob(
        self,
        request: workflow_pb2.GenerateJobRequest,
        context: grpc.ServicerContext,
    ) -> workflow_pb2.GenerateNodeResponse:
        _ = request
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "GenerateJob is not implemented")


def run(register: Callable[[WorkflowRegistry], None]) -> None:
    """Executes graph evaluation once against the graph monitor."""

    graph_monitor_address = os.getenv("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS")
    if not graph_monitor_address:
        raise WorkflowError("PULUMI_WORKFLOW_GRAPH_MONITOR_ADDRESS is required")

    context = _new_workflow_context()
    workflow_registry = WorkflowRegistry()
    register(workflow_registry)
    for token, graph_fn in workflow_registry._graphs.items():
        _evaluate_graph(token, graph_fn, context, graph_monitor_address)


def run_plugin(register: Callable[[WorkflowRegistry], None]) -> None:
    """Runs a WorkflowEvaluator gRPC server and prints the bound port on stdout."""

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    workflow_pb2_grpc.add_WorkflowEvaluatorServicer_to_server(_WorkflowEvaluatorServer(register), server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    print(port, flush=True)
    server.wait_for_termination()


__all__ = [
    "Context",
    "WorkflowRegistry",
    "WorkflowError",
    "run",
    "run_plugin",
]
