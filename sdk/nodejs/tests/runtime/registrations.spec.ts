// Copyright 2016-2020, Pulumi Corporation.
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

import { Resource } from "../../index";
import { runtime } from "../../index";

const rt = "registrationType";

function construct(name: string, type: string, urn: string): Resource {
    throw new Error("unimplemented");
}

describe("runtime", () => {
    describe("registrations", () => {
        describe("register", () => {
            const tests = [
                { name: "wildcard version", version: undefined },
                { name: "version", version: "1.2.3" },
                { name: "alpha version", version: "1.0.0-alpha1" },
            ];
            for (const { name, version } of tests) {
                it(`ignores registration on same ${name}`, () => {
                    const source = new Map<string, runtime.ResourceModule[]>();
                    assert.strictEqual(runtime.register(source, rt, "test", { version, construct }), true);
                    assert.strictEqual(runtime.register(source, rt, "test", { version, construct }), false);
                });
            }
        });

        describe("getRegistration", () => {
            const source = new Map<string, runtime.ResourceModule[]>();
            runtime.register(source, rt, "test", { version: "1.0.1-alpha1", construct });
            runtime.register(source, rt, "test", { version: "1.0.2", construct });
            runtime.register(source, rt, "test", { version: "2.2.0", construct });
            runtime.register(source, rt, "unrelated", { version: "1.0.3", construct });
            runtime.register(source, rt, "wild", { version: undefined, construct });
            runtime.register(source, rt, "unreleased", { version: "1.0.0-alpha1", construct });
            runtime.register(source, rt, "unreleased", { version: "1.0.0-beta1", construct });

            it("throws on invalid version", () => {
                assert.throws(() => runtime.getRegistration(source, "test", "invalid"));
            });

            it("unknown not found", () => {
                assert.strictEqual(runtime.getRegistration(source, "unknown", ""), undefined);
                assert.strictEqual(runtime.getRegistration(source, "unknown", "0.0.1"), undefined);
            });

            it("different major version not found", () => {
                assert.strictEqual(runtime.getRegistration(source, "test", "0.0.1"), undefined);
                assert.strictEqual(runtime.getRegistration(source, "test", "3.0.0"), undefined);
            });

            const tests = [
                { name: "blank returns highest version", key: "test", version: "", expected: "2.2.0" },
                { name: "major version respected 1.0.0", key: "test", version: "1.0.0", expected: "1.0.2" },
                { name: "major version respected 2.0.0", key: "test", version: "2.0.0", expected: "2.2.0" },
                { name: "blank returns wild", key: "wild", version: "", expected: undefined },
                { name: "any returns wild", key: "wild", version: "1.2.3", expected: undefined },
                { name: "unreleased returns beta", key: "unreleased", version: "1.0.0", expected: "1.0.0-beta1" },
            ];
            for (const { name, key, version, expected } of tests) {
                it(name, () => {
                    const module = runtime.getRegistration(source, key, version);
                    assert.notStrictEqual(module, undefined);
                    assert.strictEqual(module!.version, expected);
                });
            }
        });
    });
});
