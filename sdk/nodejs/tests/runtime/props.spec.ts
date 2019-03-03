// Copyright 2016-2018, Pulumi Corporation.
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

import * as assert from "assert";
import { Inputs, runtime } from "../../index";
import { asyncTest } from "../util";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

describe("runtime", () => {
    describe("transferProperties", () => {
        it("marshals basic properties correctly", asyncTest(async () => {
            const inputs: Inputs = {
                "aNum": 42,
                "bStr": "a string",
                "cUnd": undefined,
                "dArr": Promise.resolve([ "x", 42, Promise.resolve(true), Promise.resolve(undefined) ]),
                "id": "foo",
                "urn": "bar",
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.equal(result.aNum, 42);
            assert.equal(result.bStr, "a string");
            assert.equal(result.cUnd, undefined);
            assert.deepEqual(result.dArr, [ "x", 42, true, null ]);
            assert.equal(result.id, "foo");
            assert.equal(result.urn, "bar");
        }));
    });

    describe("deserializeProperty", () => {
        it("fails on unsupported secret values", () => {
            assert.throws(() => runtime.deserializeProperty({
                [runtime.specialSigKey]: runtime.specialSecretSig,
            }));
        });
        it("fails on unknown signature keys", () => {
            assert.throws(() => runtime.deserializeProperty({
                [runtime.specialSigKey]: "foobar",
            }));
        });
    });
});
