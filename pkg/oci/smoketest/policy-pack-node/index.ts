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

// A minimal *TypeScript* policy pack, to prove analyzer (policy) execution in the
// OCI pod model. The pack is run as a container started FROM ITS OWN IMAGE; the
// engine drives its Analyzer gRPC surface over the shared loopback. Two things make
// this discriminating (vs. a no-op that would pass from any image):
//
//   1. It is TypeScript. The pack is compiled by ts-node at run time (registered by
//      @pulumi/pulumi's run-policy-pack harness). ts-node + a compatible Node live
//      in THIS image; the engine image (alpine, no Node toolchain) has neither. So
//      the pack runs only because its toolchain is baked into its own container —
//      exactly the ambient-toolchain failure the OCI model fixes for policy packs.
//   2. The violation message carries a marker read from /policy-marker, a file baked
//      into this image alone. Had the policy run ambiently (in the engine image)
//      the read would throw — so the marker appearing in the violation proves the
//      policy logic ran from this image.
//
// The pack flags the dynamic resource the companion program registers
// (pulumi-nodejs:dynamic:Resource), so a normal `up --policy-pack` surfaces the
// violation. Enforcement is advisory, so `up` still succeeds and prints it.

import * as fs from "fs";
import { PolicyPack, ResourceValidationPolicy } from "@pulumi/policy";

const flagDynamic: ResourceValidationPolicy = {
    name: "oci-policy-smoke-flag-dynamic",
    description: "Flags the smoke test's dynamic resource to prove the analyzer ran from its image.",
    enforcementLevel: "advisory",
    validateResource: (args, reportViolation) => {
        if (args.type === "pulumi-nodejs:dynamic:Resource") {
            // Read inside the policy logic so the read proves the *evaluation* ran in
            // this image. /policy-marker exists only here.
            const marker = fs.readFileSync("/policy-marker", "utf8").trim();
            reportViolation(`oci policy ran from its image: marker=${marker}`);
        }
    },
};

new PolicyPack("oci-policy-smoke", {
    policies: [flagDynamic],
});
