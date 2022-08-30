// Copyright 2016-2022, Pulumi Corporation.
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
'use strict';

import * as packageJson from "../package.json";
import * as opentelemetry from "@opentelemetry/api";
import { Resource } from "@opentelemetry/resources";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";
import { BatchSpanProcessor, SimpleSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { ZipkinExporter } from "@opentelemetry/exporter-zipkin";
import { GrpcInstrumentation } from "@opentelemetry/instrumentation-grpc";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { registerInstrumentations } from "@opentelemetry/instrumentation";

let exporter: ZipkinExporter;
let rootSpan: opentelemetry.Span;

// serviceName is the name of this service in the Pulumi
// distributed system, and the name of the tracer we're using.
const serviceName = "nodejs-runtime";

export function start(destinationUrl: string) {
  // Set up gRPC auto-instrumentation.  
  registerInstrumentations({
    instrumentations: [new GrpcInstrumentation()],
  });

  // Tag traces from this program with metadata about their source.
  const resource = Resource.default().merge(
    new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
      [SemanticResourceAttributes.SERVICE_VERSION]: packageJson.version,
    })
  );

  const provider = new NodeTracerProvider({
    resource: resource,
  });
  
  // Configure span processor to send spans to the exporter
  // exporter = new ZipkinExporter({url: destinationUrl});
  exporter = new ZipkinExporter();
  provider.addSpanProcessor(new SimpleSpanProcessor(exporter));
  // provider.addSpanProcessor(new BatchSpanProcessor(exporter));
  console.log("Registering provider with url: ", destinationUrl);

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
  console.log("Shutting down tracer.");
  // Be sure to end the span.
  rootSpan.end();

  // flush and close the connection.
  exporter.shutdown();
}

export function newSpan(name: string): opentelemetry.Span {
  const tracer = opentelemetry.trace.getTracer(serviceName);
  const parentSpan = opentelemetry.trace.getActiveSpan() ?? rootSpan;
  const activeCtx = opentelemetry.context.active();
  const ctx = opentelemetry.trace.setSpan(activeCtx, parentSpan);
  const childSpan = tracer.startSpan(name, undefined, ctx);
  return childSpan;
}
