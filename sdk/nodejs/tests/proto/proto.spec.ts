// Copyright 2016-2025, Pulumi Corporation.
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

import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as path from "node:path";

describe("proto", () => {
    // Find all top-level *_pb.js files in the proto directory (excluding grpc files)
    const protoDir = path.join(__dirname, "../../proto");
    const protoFiles = fs.readdirSync(protoDir)
        .filter(file => file.endsWith("_pb.js") && !file.endsWith("_grpc_pb.js"));

    protoFiles.forEach((protoFile) => {
        it(`${protoFile} does not pollute global state`, () => {
            // This test ensures that importing proto files does not add pulumirpc types to global.proto.
            // The pulumirpc namespace should be module-local, not attached to the global object.
            // This test is ensuring the script (proto/generate.sh) used to generate the JS proto files
            // is correctly applying the find/replace tweaks that avoids polluting global state.

            // Dynamically import the proto module.
            const protoModule = require(path.join(protoDir, protoFile));

            // Access the module to ensure it's actually imported and not tree-shaken.
            assert(protoModule !== undefined, `${protoFile} should be imported`);

            // It's ok if global.proto exists (from other proto files), but pulumirpc should not be on it.
            if ("proto" in globalThis) {
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                const globalProto = (globalThis as any).proto;
                assert(!("pulumirpc" in globalProto),
                    `global.proto.pulumirpc shouldn't exist after importing ${protoFile}`);
            }
        });
    });
});
