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

// tslint:disable

import * as assert from "assert";
import { Output, concat, interpolate, output } from "../output";
import * as runtime from "../runtime";
import { asyncTest } from "./util";
import { mergeOptions } from "../utils";


describe("options", () => {
    describe("merge", () => {
        describe("scaler", () => {
            it("keeps value from opts1 if not provided in opts2", asyncTest(async () => {
                const result = mergeOptions({ id: "a" }, {});
                assert.strictEqual(result.id, "a");
            }));
            it("keeps value from opts2 if not provided in opts1", asyncTest(async () => {
                const result = mergeOptions({ }, { id: "a" });
                assert.strictEqual(result.id, "a");
            }));
            it("overwrites value from opts1 if given null in opts2", asyncTest(async () => {
                const result = mergeOptions({ id: "a" }, { id: null! });
                assert.strictEqual(result.id, null);
            }));
            it("overwrites value from opts1 if given undefined in opts2", asyncTest(async () => {
                const result = mergeOptions({ id: "a" }, { id: undefined });
                assert.strictEqual(result.id, null);
            }));
            it("overwrites value from opts1 if given value in opts2", asyncTest(async () => {
                const result = mergeOptions({ id: "a" }, { id: "b" });
                assert.strictEqual(result.id, "b");
            }));
        });

        describe("array", () => {
            it("keeps value from opts1 if not provided in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, {});
                assert.deepStrictEqual(result.ignoreChanges, ["a"]);
            }));
            it("keeps value from opts2 if not provided in opts1", asyncTest(async () => {
                const result = mergeOptions({ }, { ignoreChanges: ["a"] });
                assert.strictEqual(result.ignoreChanges, ["a"]);
            }));
            it("overwrites value from opts1 if given null in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { id: null! });
                assert.strictEqual(result.ignoreChanges, null);
            }));
            it("overwrites value from opts1 if given undefined in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { id: undefined });
                assert.strictEqual(result.ignoreChanges, null);
            }));
            it("merges values from opts1 if given value in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { ignoreChanges: ["b"] });
                assert.strictEqual(result.ignoreChanges, ["a", "b"]);
            }));

            describe("including promises", () => {
                it("merges promise in opts1 and non-promise in opts2", asyncTest(async () => {
                    const result = mergeOptions({ aliases: ["a"] }, { aliases: [Promise.resolve("b")] });
                    const aliases = result.aliases!;
                    assert.strictEqual(Array.isArray(aliases), true);
                    assert.strictEqual(aliases.length, 2);
                    assert.strictEqual(aliases[0], "a");
                    assert.strictEqual(result.ignoreChanges, ["a", "b"]);
                }));
            });
        })
    });
});
