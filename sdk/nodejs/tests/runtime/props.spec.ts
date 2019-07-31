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
        it("pushed secretness up correctly", () => {
            const secretValue = {
                [runtime.specialSigKey]: runtime.specialSecretSig,
                "value": "a secret value",
            };

            const props = gstruct.Struct.fromJavaScript({
                "regular": "a normal value",
                "list": [ "a normal value", "another value", secretValue ],
                "map":  { "regular": "a normal value", "secret": secretValue },
                "mapWithList": {
                    "regular": "a normal value",
                    "list": [ "a normal value", secretValue ],
                },
                "listWithMap": [{
                    "regular": "a normal value",
                    "secret": secretValue,
                }],
            });

            const result = runtime.deserializeProperties(props);

            console.log(JSON.stringify(result));

            // Regular had no secrets in it, so it is returned as is.
            assert.equal(result.regular, "a normal value");

            // One of the elements in the list was a secret, so the secretness is promoted to top level.
            assert.equal(result.list[runtime.specialSigKey], runtime.specialSecretSig);
            assert.equal(result.list.value[0], "a normal value");
            assert.equal(result.list.value[1], "another value");
            assert.equal(result.list.value[2], "a secret value");

            // One of the values of the map was a secret, so the secretness is promoted to top level.
            assert.equal(result.map[runtime.specialSigKey], runtime.specialSecretSig);
            assert.equal(result.map.value.regular, "a normal value");
            assert.equal(result.map.value.secret, "a secret value");

            // The nested map had a secret in one of the values, so the entire thing becomes a secret.
            assert.equal(result.mapWithList[runtime.specialSigKey], runtime.specialSecretSig);
            assert.equal(result.mapWithList.value.regular, "a normal value");
            assert.equal(result.mapWithList.value.list[0], "a normal value");
            assert.equal(result.mapWithList.value.list[1], "a secret value");

            // An array element contained a secret (via a nested map), so the entrie array becomes a secret.
            assert.equal(result.listWithMap[runtime.specialSigKey], runtime.specialSecretSig);
            assert.equal(result.listWithMap.value[0].regular, "a normal value");
            assert.equal(result.listWithMap.value[0].secret, "a secret value");
        });
    });
});
