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
import { Inputs, runtime, secret } from "../../index";
import { asyncTest } from "../util";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

// tslint:disable-next-line:variable-name
const TestStrEnum = {
    Foo: "foo",
    Bar: "bar",
} as const;

type TestStrEnum = (typeof TestStrEnum)[keyof typeof TestStrEnum];

// tslint:disable-next-line:variable-name
const TestIntEnum = {
    One: 1,
    Zero: 0,
} as const;

type TestIntEnum = (typeof TestIntEnum)[keyof typeof TestIntEnum];

// tslint:disable-next-line:variable-name
const TestNumEnum = {
    One: 1.0,
    ZeroPointOne: 0.1,
} as const;

type TestNumEnum = (typeof TestNumEnum)[keyof typeof TestNumEnum];

// tslint:disable-next-line:variable-name
const TestBoolEnum = {
    One: true,
    Zero: false,
} as const;

type TestBoolEnum = (typeof TestBoolEnum)[keyof typeof TestBoolEnum];

interface TestInputs {
    aNum: number;
    bStr: string;
    cUnd: undefined;
    dArr: Promise<Array<any>>;
    id: string;
    urn: string;
    strEnum: TestStrEnum;
    intEnum: TestIntEnum;
    numEnum: TestNumEnum;
    boolEnum: TestBoolEnum;
}

describe("runtime", () => {
    describe("transferProperties", () => {
        it("marshals basic properties correctly", asyncTest(async () => {
            const inputs: TestInputs = {
                "aNum": 42,
                "bStr": "a string",
                "cUnd": undefined,
                "dArr": Promise.resolve([ "x", 42, Promise.resolve(true), Promise.resolve(undefined) ]),
                "id": "foo",
                "urn": "bar",
                "strEnum": TestStrEnum.Foo,
                "intEnum": TestIntEnum.One,
                "numEnum": TestNumEnum.One,
                "boolEnum": TestBoolEnum.One,
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.strictEqual(result.aNum, 42);
            assert.strictEqual(result.bStr, "a string");
            assert.strictEqual(result.cUnd, undefined);
            assert.deepStrictEqual(result.dArr, [ "x", 42, true, null ]);
            assert.strictEqual(result.id, "foo");
            assert.strictEqual(result.urn, "bar");
            assert.strictEqual(result.strEnum, TestStrEnum.Foo);
            assert.strictEqual(result.intEnum, TestIntEnum.One);
            assert.strictEqual(result.numEnum, TestNumEnum.One);
            assert.strictEqual(result.boolEnum, TestBoolEnum.One);
        }));
        it("marshals secrets correctly", asyncTest(async () => {
            runtime._setTestModeEnabled(true);
            const inputs: Inputs = {
                "secret1": secret(1),
                "secret2": secret(undefined),
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.strictEqual(result.secret1, 1);
            assert.strictEqual(result.secret2, undefined);
            runtime._setTestModeEnabled(false);
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

            // Regular had no secrets in it, so it is returned as is.
            assert.strictEqual(result.regular, "a normal value");

            // One of the elements in the list was a secret, so the secretness is promoted to top level.
            assert.strictEqual(result.list[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.list.value[0], "a normal value");
            assert.strictEqual(result.list.value[1], "another value");
            assert.strictEqual(result.list.value[2], "a secret value");

            // One of the values of the map was a secret, so the secretness is promoted to top level.
            assert.strictEqual(result.map[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.map.value.regular, "a normal value");
            assert.strictEqual(result.map.value.secret, "a secret value");

            // The nested map had a secret in one of the values, so the entire thing becomes a secret.
            assert.strictEqual(result.mapWithList[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.mapWithList.value.regular, "a normal value");
            assert.strictEqual(result.mapWithList.value.list[0], "a normal value");
            assert.strictEqual(result.mapWithList.value.list[1], "a secret value");

            // An array element contained a secret (via a nested map), so the entrie array becomes a secret.
            assert.strictEqual(result.listWithMap[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.listWithMap.value[0].regular, "a normal value");
            assert.strictEqual(result.listWithMap.value[0].secret, "a secret value");
        });
    });
});
