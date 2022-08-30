'use strict';

import * as opentelemetry from "@opentelemetry/api";
import { Resource } from "@opentelemetry/resources";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";
import { BasicTracerProvider, ConsoleSpanExporter, SimpleSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { ZipkinExporter } from "@opentelemetry/exporter-zipkin";

export function initGlobalTracer() {
  console.log("Running global tracer.");
  
  const provider = new BasicTracerProvider({
    resource: new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: 'nodejs-runtime',
    }),
  });

  // Configure span processor to send spans to the exporter
  const exporter = new ZipkinExporter();

  provider.addSpanProcessor(new SimpleSpanProcessor(exporter));
  provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));

  /**
   * Initialize the OpenTelemetry APIs to use the BasicTracerProvider bindings.
   *
   * This registers the tracer provider with the OpenTelemetry API as the global
   * tracer provider. This means when you call API methods like
   * `opentelemetry.trace.getTracer`, they will use this tracer provider. If you
   * do not register a global tracer provider, instrumentation which calls these
   * methods will receive no-op implementations.
   */
  provider.register();
  const tracer = opentelemetry.trace.getTracer('nodejs-runtime');

  // Create a span. A span must be closed.
  const parentSpan = tracer.startSpan('main');
  for (let i = 0; i < 10; i += 1) {
    doWork(parentSpan);
  }
  // Be sure to end the span.
  parentSpan.end();

  // flush and close the connection.
  exporter.shutdown();

  function doWork(parent: any) {
      // Start another span. In this example, the main method already started a
      // span, so that'll be the parent span, and this will be a child span.
      const ctx = opentelemetry.trace.setSpan(opentelemetry.context.active(), parent);
      const span = tracer.startSpan('doWork', undefined, ctx);

      // Set attributes to the span.
      span.setAttribute('robbie-key', 'robbie-value');
   
      // simulate some random work.
      for (let i = 0; i <= 100; i += 1) {
          // empty
          // console.log("RUNNING SIMULATED WORK");
      }
   
      // Set attributes to the span.
      span.setAttribute('robbie-key-two', 'robbie-value-two');

       // Annotate our span to capture metadata about our operation
      span.addEvent('invoking doWork');
  
      span.end();
  }
}

