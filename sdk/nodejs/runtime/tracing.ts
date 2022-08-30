'use strict';

import * as opentelemetry from "@opentelemetry/api";
import { Resource } from "@opentelemetry/resources";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";
import { BasicTracerProvider, ConsoleSpanExporter, SimpleSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { ZipkinExporter } from "@opentelemetry/exporter-zipkin";

let exporter: ZipkinExporter;
let rootSpan: opentelemetry.Span;

// name is the name of the tracer we're using.
const tracerName = "nodejs-runtime";

export function start() {
  // TODO: Replace BasicTracer with something more sophisticated?
  // TODO: Add more resource fields.
  const provider = new BasicTracerProvider({
    resource: new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: 'nodejs-runtime',
    }),
  });
  // Configure span processor to send spans to the exporter
  exporter = new ZipkinExporter();

  // TODO: Replaec SimpleSpanProcessor with BatchProcesses
  provider.addSpanProcessor(new SimpleSpanProcessor(exporter));

  /**
   * Taken from OpenTelemetry Examples (Apache 2 License):
   * https://github.com/open-telemetry/opentelemetry-js/blob/a8d39317b5daad727f2116ca314db0d1420ec488/examples/basic-tracer-node/index.js
   * Initialize the OpenTelemetry APIs to use the BatchTracerProvider bindings.
   *
   * A "tracer provider" is a factory for tracers. By registering the provider,
   * we allow tracers of the given type to be globally contructed.
   * As a result, when you call API methods like
   * `opentelemetry.trace.getTracer`, the tracer is generated via the tracer provder
   * registered here.
   */
  provider.register();
  const tracer = opentelemetry.trace.getTracer('nodejs-runtime');
  // Create a root span, which must be closed.
  rootSpan = tracer.startSpan('nodejs-runtime-root');
}

export function stop() {
  // Be sure to end the span.
  rootSpan.end();

  // flush and close the connection.
  exporter.shutdown();
}

export function newSpan(name: string): opentelemetry.Span {
  const tracer = opentelemetry.trace.getTracer(tracerName);
  const parentSpan = opentelemetry.trace.getActiveSpan() ?? rootSpan;
  const activeCtx = opentelemetry.context.active();
  const ctx = opentelemetry.trace.setSpan(activeCtx, parentSpan);
  const childSpan = tracer.startSpan(name, undefined, ctx);
  return childSpan;
}
