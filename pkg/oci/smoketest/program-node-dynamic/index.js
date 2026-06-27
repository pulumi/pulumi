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

// A minimal Node Pulumi program that registers a *dynamic* resource, to prove
// dynamic-provider execution in the OCI pod model. The dynamic provider's CRUD
// code is serialized from this program and runs in a separate provider process;
// in the pod model that process is a container started FROM THIS PROGRAM'S IMAGE
// (the SDK's dynamic-provider entrypoint is native to it), so the serialized
// closure resolves against the program's own baked filesystem and dependencies.
//
// The welding is proven with a marker baked into the program image: the
// provider's create reads /program-marker and returns it as the resource's
// output. If the provider had run from any other image the file would be absent
// and create would throw — so a single assertion (stack output == the baked
// marker) proves both "the dynamic resource was created" and "its provider ran
// welded to the program image".
"use strict";

const pulumi = require("@pulumi/pulumi");

const markerProvider = {
    async create(_inputs) {
        // require + read inside the closure so both are captured in the serialized
        // provider and resolve in the provider process — a container from this
        // program's image. /program-marker exists only in that image.
        const fs = require("fs");
        const marker = fs.readFileSync("/program-marker", "utf8").trim();
        return { id: "oci-dynamic-1", outs: { marker } };
    },
    async delete(_id, _props) {
        // Destroy runs with NO program process: the engine starts this provider from
        // the program image and deserializes the closure from state to call delete.
        // Read the marker here too — a successful destroy then proves the provider
        // ran welded to the program image even though the program never ran (a bare
        // image would lack /program-marker, throw, and fail the destroy).
        const fs = require("fs");
        fs.readFileSync("/program-marker", "utf8");
    },
};

class MarkerResource extends pulumi.dynamic.Resource {
    constructor(name, opts) {
        super(markerProvider, name, { marker: undefined }, opts);
    }
}

const resource = new MarkerResource("oci-smoke-dynamic");

// Surface the provider-produced output so the smoke test can assert on it.
exports.marker = resource.marker;
