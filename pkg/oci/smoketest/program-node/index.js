// Copyright 2026, Pulumi Corporation.
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

// A minimal Node Pulumi program for the language-image smoke test. It registers
// no resources — its only job is to prove that a *Node* program, running as a
// container in the pod, bootstraps the @pulumi/pulumi runtime from PULUMI_* env
// (via the base image's entrypoint shim), connects to the engine's monitor, runs,
// and reports a stack output. This validates the language-image + bootstrap
// contract for a non-Go language before layering on providers or the MLC flow.
"use strict";

const pulumi = require("@pulumi/pulumi");

pulumi.log.info("oci smoke-test: node program running inside a pod container");

exports.greeting = "hello-from-node-in-a-pod";
