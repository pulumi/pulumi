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

// For troubleshooting, set the log level to DiagLogLevel.DEBUG
diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);

const sdk = new opentelemetry.NodeSDK({
  traceExporter: new opentelemetry.tracing.ConsoleSpanExporter(),
  instrumentations: [getNodeAutoInstrumentations()]
});

sdk.start()
