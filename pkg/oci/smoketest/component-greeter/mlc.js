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

// A minimal Node multi-language component (MLC) for the OCI pod smoke test. It
// runs as a *provider* container: provider.main binds a port, prints it, and
// serves the ResourceProvider gRPC (including Construct + Attach) — exactly like
// the resource providers, so the engine's container host starts and attaches to
// it unchanged. When a (Go) program creates a greeting:index:Greeter, the engine
// calls Construct here; this Node code runs the component and registers it,
// returning the component's outputs. This proves the program=component
// unification: a Go program drives a Node component, both as pod containers.
"use strict";

const pulumi = require("@pulumi/pulumi");
const provider = require("@pulumi/pulumi/provider");

class Greeter extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("greeting:index:Greeter", name, {}, opts);
        const who = (args && args.who) || "world";
        this.message = pulumi.output(`hello, ${who}, from a Node multi-language component`);
        this.registerOutputs({ message: this.message });
    }
}

const greetingProvider = {
    version: "0.1.0",
    construct(name, type, inputs, options) {
        if (type !== "greeting:index:Greeter") {
            throw new Error(`unknown resource type ${type}`);
        }
        const g = new Greeter(name, inputs, options);
        return Promise.resolve({
            urn: g.urn,
            state: { message: g.message },
        });
    },
};

provider.main(greetingProvider, process.argv.slice(2));
