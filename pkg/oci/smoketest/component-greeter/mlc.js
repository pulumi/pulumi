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
//
// Crucially, the component registers a *real, provider-backed child*
// (random.RandomPet) rather than just round-tripping a string. That makes the
// component build infrastructure — the whole point of an MLC — and forces the
// engine to lazily start the `random` provider as another pod container *during
// Construct*. That recursive provider-start is what proves the full chain works:
// program -> MLC container -> child RegisterResource -> engine -> provider
// container -> monitor.
"use strict";

const pulumi = require("@pulumi/pulumi");
const provider = require("@pulumi/pulumi/provider");
const random = require("@pulumi/random");

class Greeter extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("greeting:index:Greeter", name, {}, opts);
        const who = (args && args.who) || "world";
        // The child resource: registering it drives the engine to start the
        // `random` provider container recursively, from within this component's
        // Construct. The generated pet name flows back out through the message.
        const pet = new random.RandomPet(`${name}-pet`, {}, { parent: this });
        this.message = pulumi.interpolate`hello, ${who}, from a Node multi-language component (pet: ${pet.id})`;
        this.registerOutputs({ message: this.message });
    }
}

// The component's package schema, served over GetSchema so `pulumi package add
// oci://<ref>` can extract it from the running image and generate a typed SDK.
// Greeter is marked isComponent so generated SDKs register it remotely (Construct).
const schema = {
    name: "greeting",
    version: "0.1.0",
    resources: {
        "greeting:index:Greeter": {
            isComponent: true,
            inputProperties: {
                who: { type: "string", description: "Who to greet." },
            },
            requiredInputs: ["who"],
            properties: {
                message: {
                    type: "string",
                    description: "The greeting, including the child RandomPet's name.",
                },
            },
            required: ["message"],
        },
    },
};

const greetingProvider = {
    version: "0.1.0",
    schema: JSON.stringify(schema),
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
