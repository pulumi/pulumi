// Copyright 2016-2024, Pulumi Corporation.
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
import * as rpc from "../../runtime/rpc";

const secretUnwrappingTestData: { original: any; unwrapped: any }[] = [
    // primitive value wrapped in a secret
    {
        original: rpc.serializeSecretValue("secret"),
        unwrapped: "secret",
    },
    // object with a secret value,
    {
        original: {
            first: "first",
            second: rpc.serializeSecretValue("second"),
            nested: {
                first: "first",
                second: rpc.serializeSecretValue("secret"),
            },
        },
        unwrapped: {
            first: "first",
            second: "second",
            nested: {
                first: "first",
                second: "secret",
            },
        },
    },
    // inside an array
    {
        original: ["first", rpc.serializeSecretValue("second")],
        unwrapped: ["first", "second"],
    },
    // inside an array inside an object
    {
        original: {
            first: "first",
            second: [{ nested: [rpc.serializeSecretValue("nested")] }],
        },
        unwrapped: {
            first: "first",
            second: [{ nested: ["nested"] }],
        },
    },
];

describe("rpc tests", () => {
    it("unwrapSecretValues works", () => {
        for (const { original, unwrapped } of secretUnwrappingTestData) {
            const [result, containsSecret] = rpc.unwrapSecretValues(original);
            assert.strictEqual(containsSecret, true);
            assert.deepStrictEqual(result, unwrapped);
            const [unwrappedResult, containsSecretAgain] = rpc.unwrapSecretValues(result);
            assert.strictEqual(containsSecretAgain, false);
            assert.deepStrictEqual(unwrappedResult, result);
        }
    });
});
