// Copyright 2026-2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// IMPORTANT: This file must be required/imported BEFORE any other modules
// that might load @grpc/grpc-js. The OpenTelemetry instrumentation works by
// monkey-patching the gRPC module, which must happen before it's loaded.

import * as opentelemetry from "@opentelemetry/api";

let rootContext: opentelemetry.Context | null = null;
let tracerProvider: any = null;

if (process.env.TRACEPARENT) {
    // These imports are done dynamically inside the if block to avoid
    // loading unnecessary modules when tracing is not enabled
    const { registerInstrumentations } = require("@opentelemetry/instrumentation");
    const { GrpcInstrumentation } = require("@opentelemetry/instrumentation-grpc");
    const { NodeTracerProvider } = require("@opentelemetry/sdk-trace-node");
    const { W3CTraceContextPropagator } = require("@opentelemetry/core");
    const { BatchSpanProcessor } = require("@opentelemetry/sdk-trace-base");
    const { Resource } = require("@opentelemetry/resources");
    const { ATTR_SERVICE_NAME } = require("@opentelemetry/semantic-conventions");

    const provider = new NodeTracerProvider({
        resource: new Resource({
            [ATTR_SERVICE_NAME]: "pulumi-sdk-nodejs",
        }),
    });

    const otlpEndpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
    if (otlpEndpoint) {
        const { OTLPTraceExporter } = require("@opentelemetry/exporter-trace-otlp-grpc");
        process.env.OTEL_EXPORTER_OTLP_INSECURE = "true";
        const exporter = new OTLPTraceExporter({
            url: otlpEndpoint,
        });
        provider.addSpanProcessor(new BatchSpanProcessor(exporter));
    }

    provider.register();
    tracerProvider = provider;

    const propagator = new W3CTraceContextPropagator();
    opentelemetry.propagation.setGlobalPropagator(propagator);

    function captureStackTrace(): string {
        const err = new Error();
        const stack = err.stack || "";
        const lines = stack.split("\n").slice(4);
        return lines
            .map((line) => line.trim().replace(/^at /, ""))
            .filter((line) => line.length > 0)
            .join("\n");
    }

    if (otlpEndpoint) {
        registerInstrumentations({
            instrumentations: [
                new GrpcInstrumentation({
                    // Add stack trace to client spans
                    requestHook: (span: opentelemetry.Span, _requestInfo: unknown) => {
                        span.setAttribute("code.stacktrace", captureStackTrace());
                    },
                }),
            ],
        });
    }

    const envCarrier: Record<string, string> = {};
    envCarrier["traceparent"] = process.env.TRACEPARENT;
    rootContext = opentelemetry.propagation.extract(opentelemetry.context.active(), envCarrier);
}

/**
 * Run a function within the root tracing context.
 * If tracing is not enabled, the function is called directly without wrapping.
 */
export function withRootContext<T>(fn: () => T): T {
    if (rootContext) {
        return opentelemetry.context.with(rootContext, fn);
    }
    return fn();
}

/**
 * Shutdown the tracer provider and flush any pending spans.
 * Should be called before process exit.
 */
export async function shutdownTracing(): Promise<void> {
    if (tracerProvider) {
        await tracerProvider.shutdown();
    }
}
