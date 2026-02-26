# Copyright 2026-2026, Pulumi Corporation.
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

"""
OpenTelemetry instrumentation for Pulumi Python SDK.

IMPORTANT: This module must be imported BEFORE any other modules that might
load grpc. The OpenTelemetry instrumentation works by monkey-patching the
gRPC module, which must happen before it's loaded.

This module sets up OpenTelemetry tracing when TRACEPARENT environment variable
is present, enabling distributed tracing across Pulumi components.
"""

from __future__ import annotations

import atexit
import os
import traceback
from typing import Optional

from opentelemetry import context as otel_context
from opentelemetry import propagate
from opentelemetry import trace
from opentelemetry.context import Context

_root_context: Optional[Context] = None
_tracer_provider: Optional[trace.TracerProvider] = None


def _capture_stack_trace() -> str:
    """Capture the current stack trace, excluding instrumentation frames."""
    stack = traceback.extract_stack()
    stack = stack[:-4]
    return "\n".join(
        f"{frame.filename}:{frame.lineno} in {frame.name}"
        for frame in stack
    )


def _initialize_tracing() -> None:
    """Initialize OpenTelemetry tracing if TRACEPARENT is present."""
    global _root_context, _tracer_provider

    traceparent = os.environ.get("TRACEPARENT")
    otlp_endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT")

    if not traceparent:
        return

    # IMPORTANT: Instrument gRPC BEFORE importing anything that uses grpc
    # (like the OTLP exporter). The instrumentation wraps grpc functions,
    # which must happen before grpc is loaded.
    if otlp_endpoint:
        from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient

        def client_request_hook(span, request):
            span.set_attribute("code.stacktrace", _capture_stack_trace())

        grpc_client_instrumentor = GrpcInstrumentorClient()
        grpc_client_instrumentor.instrument(request_hook=client_request_hook)

    from opentelemetry.sdk.resources import Resource, SERVICE_NAME
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    resource = Resource.create({SERVICE_NAME: "pulumi-sdk-python"})
    provider = TracerProvider(resource=resource)

    if otlp_endpoint:
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
            OTLPSpanExporter,
        )

        os.environ["OTEL_EXPORTER_OTLP_INSECURE"] = "true"
        exporter = OTLPSpanExporter(endpoint=otlp_endpoint)
        provider.add_span_processor(BatchSpanProcessor(exporter))

    trace.set_tracer_provider(provider)
    _tracer_provider = provider

    carrier = {"traceparent": traceparent}
    _root_context = propagate.extract(carrier)

    otel_context.attach(_root_context)

    def _shutdown_on_exit():
        if _tracer_provider is not None:
            _tracer_provider.shutdown()

    atexit.register(_shutdown_on_exit)


_initialize_tracing()


def wrap_with_context(fn):
    """Wrap a callable so it runs with the current OTel context.

    Use this when passing callables to run_in_executor, since thread pool
    threads do not inherit the OTel context from the calling thread.
    """
    ctx = otel_context.get_current()

    def wrapper(*args, **kwargs):
        token = otel_context.attach(ctx)
        try:
            return fn(*args, **kwargs)
        finally:
            otel_context.detach(token)

    return wrapper


async def shutdown_tracing() -> None:
    """
    Shutdown the tracer provider and flush any pending spans.
    Should be called before process exit.
    """
    if _tracer_provider is not None:
        _tracer_provider.shutdown()
