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
import { ComponentResourceOptions, ProviderResource, merge, mergeOptions } from "../resource";
import { asyncTest } from "./util";

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
                assert.strictEqual(result.id, undefined);
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
                assert.deepStrictEqual(result.ignoreChanges, ["a"]);
            }));
            it("does nothing to value from opts1 if given null in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { ignoreChanges: null! });
                assert.deepStrictEqual(result.ignoreChanges, ["a"]);
            }));
            it("does nothing to value from opts1 if given undefined in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { ignoreChanges: undefined });
                assert.deepStrictEqual(result.ignoreChanges, ["a"]);
            }));
            it("merges values from opts1 if given value in opts2", asyncTest(async () => {
                const result = mergeOptions({ ignoreChanges: ["a"] }, { ignoreChanges: ["b"] });
                assert.deepStrictEqual(result.ignoreChanges, ["a", "b"]);
            }));

            describe("including promises", () => {
                it("merges non-promise in opts1 and promise in opts2", asyncTest(async () => {
                    const result = mergeOptions({ aliases: ["a"] }, { aliases: [Promise.resolve("b")] });
                    const aliases = result.aliases!;
                    assert.deepStrictEqual(Array.isArray(aliases), true);
                    assert.deepStrictEqual(aliases.length, 2);
                    assert.deepStrictEqual(aliases[0], "a");
                    assert.deepStrictEqual(aliases[1] instanceof Promise, true);

                    const val1 = await aliases[1];
                    assert.deepStrictEqual(val1, "b");
                }));
                it("merges promise in opts1 and non-promise in opts2", asyncTest(async () => {
                    const result = mergeOptions({ aliases: [Promise.resolve("a")] }, { aliases: ["b"] });
                    const aliases = result.aliases!;
                    assert.deepStrictEqual(Array.isArray(aliases), true);
                    assert.deepStrictEqual(aliases.length, 2);
                    assert.deepStrictEqual(aliases[0] instanceof Promise, true);
                    assert.deepStrictEqual(aliases[1], "b");

                    const val0 = await aliases[0];
                    assert.deepStrictEqual(val0, "a");
                }));
            });
        });

        describe("providers", () => {
            const awsProvider = <ProviderResource>{ getPackage: () => "aws" };
            const azureProvider = <ProviderResource>{ getPackage: () => "azure" };
            const gcpProvider = <ProviderResource>{ getPackage: () => "gcp" };

            it("merges singleton into map", () => {
                const result = mergeOptions({ providers: { aws: awsProvider } }, { provider: azureProvider });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });
            it("merges singleton-array into map", () => {
                const result = mergeOptions({ providers: { aws: awsProvider } }, { providers: [azureProvider] });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });
            it("merges array into map", () => {
                const result = mergeOptions({ providers: { aws: awsProvider } }, { providers: [azureProvider, gcpProvider] });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider, gcp: gcpProvider } });
            });

            it("merges map into singleton", () => {
                const result = mergeOptions({ provider: awsProvider }, { providers: { azure: azureProvider } });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });
            it("merges map into singleton-array", () => {
                const result = mergeOptions({ providers: [awsProvider] }, { providers: { azure: azureProvider } });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });
            it("merges map into array", () => {
                const result = mergeOptions({ providers: [awsProvider, azureProvider] }, { providers: { gcp: gcpProvider } });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider, gcp: gcpProvider } });
            });

            it("merges map into map", () => {
                const result = mergeOptions({ providers: { aws: awsProvider } }, { providers: { azure: azureProvider } });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });

            it("merges array into array", () => {
                const result = mergeOptions({ providers: [awsProvider] }, { providers: [azureProvider] });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });

            it("merges singleton into singleton", () => {
                const result = mergeOptions(<ComponentResourceOptions>{ provider: awsProvider }, { provider: azureProvider });
                assert.deepStrictEqual(result, { providers: { aws: awsProvider, azure: azureProvider } });
            });
        });

        describe("dependsOn", () => {
            function mergeDependsOn(a: any, b: any): any {
                return merge(a, b, /*alwaysCreateArray:*/ true);
            }

            it("merges two scalers into array", () => {
                const result = mergeDependsOn("a", "b");
                assert.deepStrictEqual(result, ["a", "b"]);
            });
            it("merges array and scaler", () => {
                const result = mergeDependsOn(["a"], "b");
                assert.deepStrictEqual(result, ["a", "b"]);
            });
            it("merges scaler and array", () => {
                const result = mergeDependsOn("a", ["b"]);
                assert.deepStrictEqual(result, ["a", "b"]);
            });

            it("merges promise-scaler and scaler into array", async () => {
                const result = mergeDependsOn(Promise.resolve("a"), "b");
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
            it("merges scaler and promise-scaler into array", async () => {
                const result = mergeDependsOn("a", Promise.resolve("b"));
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
            it("merges promise-scaler and promise-scaler into array", async () => {
                const result = mergeDependsOn(Promise.resolve("a"), Promise.resolve("b"));
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });

            it("merges promise-array and scaler into array", async () => {
                const result = mergeDependsOn(Promise.resolve(["a"]), "b");
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
            it("merges promise-scaler and array into array", async () => {
                const result = mergeDependsOn(Promise.resolve("a"), ["b"]);
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
            it("merges promise-scaler and promise-array into array", async () => {
                const result = mergeDependsOn(Promise.resolve("a"), Promise.resolve(["b"]));
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
            it("merges promise-array and promise-array into array", async () => {
                const result = mergeDependsOn(Promise.resolve(["a"]), Promise.resolve(["b"]));
                assert.deepStrictEqual(await result.promise(), ["a", "b"]);
            });
        });
    });
});
