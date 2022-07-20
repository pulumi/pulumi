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

// TODO:
//   • Inspect cmd/pulumi-language-nodejs/main.go cmdutil.InitTracing()
//   • Check to see if we pass any environment variables when booting this subshell.
//   • Find location of shell boot.
//        Looking like newLanguageHost(engineAddress, tracing, ...)
//        https://github.com/pulumi/pulumi/pull/10173/files#diff-9f95bbceb9df8458a048c747ac37385bd37d97c012c640456cad2604aeed85dbR561
//        Located cmd/pulumi-langauge-nodejs/main.go line 123.
//        NodeArgs needs to be adjusted if tracing enabled.
//   • Add --require to shell args if tracing enabled.
//   • Add TraceID as Env Var when booting.
//   • Add Zipkin URI as Env Var when booting.
//   • Accept TraceID and Zipkin URI from the NodeJS side.

import * as opentelemetry from "@opentelemetry/sdk-node";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { diag, DiagConsoleLogger, DiagLogLevel } from "@opentelemetry/api";
import { ZipkinExporter } from "@opentelemetry/exporter-zipkin";
import  { BatchSpanProcessor, BasicTracerProvider } from "@opentelemetry/sdk-trace-base";

// The name is reported to the trace exporter and associates all traces from
// the NodeJS runtime grouping them.
const serviceName = 'pulumi-nodejs-language-host';

// This global variable is initialized with the "start" function. Using a global
// ensures the tracer is not deallocated during the course of execution.
let sdk: opentelemetry.NodeSDK;

// This function starts the tracing engine using Zipkin as a backend.
export function start(destinationUrl: string) {
  
  const zipkin = configureZipkinExporter(destinationUrl);

  // TODO: Initialize:
  //       • TraceProvider
  //       • TraceExporter
  //       • Instrumentation (for gRPC)
  
  // A TraceProvider is a factory for traces. When a new trace is created,
  // either through a library call or automatically as part of gRPC hooks,
  // the trace is created by the TraceProvider.
  const provider = new BasicTracerProvider({
    resource: new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
    }),
  });

  provider.addSpanProcessor(new BatchSpanProcessor(zipkin))
  provider.register();

  // TODO: Remove SDK everything.
  // Initialize the SDK global variable.
  sdk = new opentelemetry.NodeSDK({
    serviceName: serviceName,
    traceExporter: configureZipkinExporter(destinationUrl),
    instrumentations: [getNodeAutoInstrumentations()]
  });

  sdk.start();
}

function configureZipkinExporter(destinationUrl: string): ZipkinExporter {
  const zipkinOptions = {
    url: destinationUrl,
  };
  return new ZipkinExporter(options);
}

function tracerProvider: BasicTracerProvider {
  const provider = 
  });
}

function newTracer(): opentelemetry.NodeSDK {
  return opentelemetry.NodeSDK({
    
  });  
}

// For troubleshooting, set the log level to DiagLogLevel.DEBUG
diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);


// TODO: Span processing
// TODO: Set trace exporter.
// TODO: What is getNodeAutoInstrumentations?



