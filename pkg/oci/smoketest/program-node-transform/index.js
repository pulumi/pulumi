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

// The Node twin of program-transform/ (Go): a program that registers a RESOURCE
// TRANSFORM, which is the one place a program serves an inbound RPC instead of only
// dialing out.
//
// registerResourceTransform stands up a callback gRPC server in this process and hands
// the engine its address, so the engine dials back here for every resource
// registration. Whether that address is reachable is the whole point of the test: it is
// built from PULUMI_CALLBACKS_ADVERTISE_HOST when the program runs in its own network
// namespace, and defaults to loopback when it shares one with the engine.
//
// The transform sets `prefix`, so the resulting pet name is self-evidently transformed
// or not — a name starting with "transformed-" proves the engine reached back in.
"use strict";

const pulumi = require("@pulumi/pulumi");
const random = require("@pulumi/random");

// Injected by the transform; visible in the stack output, so "the transform ran" and
// "the transform was skipped" cannot be confused.
const TRANSFORMED_PREFIX = "transformed";

pulumi.runtime.registerResourceTransform((args) => {
    if (args.type !== "random:index/randomPet:RandomPet") {
        return undefined;
    }
    return {
        props: { ...args.props, prefix: TRANSFORMED_PREFIX },
        opts: args.opts,
    };
});

pulumi.log.info("oci transform program (node) registered a resource transform (engine must dial back)");

const pet = new random.RandomPet("pet", {});

exports.petName = pet.id;
