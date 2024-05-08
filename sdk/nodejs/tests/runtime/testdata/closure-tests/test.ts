// Copyright 2024-2024, Pulumi Corporation.
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
import execa from "execa";
import * as fs from "fs/promises";
import { readdirSync } from "fs";
import * as path from "path";
import * as semver from "semver";
import * as typescript from "typescript";
// @ts-ignore: The test installs @pulumi/pulumi
import { runtime } from "@pulumi/pulumi";
// @ts-ignore: The test installs @pulumi/pulumi
import * as pkg from "@pulumi/pulumi/runtime/closure/package";


const platformIndependentEOL = /\r\n|\r|\n/g;

// Load the snapshot with a version range that satisfies the typescript version.
async function getSnapshot(testCase: string, typescriptVersion: string): Promise<string> {
    const files = await fs.readdir(`./cases/${testCase}`);
    for (const file of files) {
        if (file.startsWith("snapshot.") && file.endsWith(".txt")) {
            const range = file.slice("snapshot.".length, -".txt".length);
            if (range) {
                if (semver.satisfies(typescriptVersion, range)) {
                    return fs.readFile(path.join("cases", testCase, `snapshot.${range}.txt`), "utf-8");
                }
            }
        }
    }
    return fs.readFile(path.join("cases", testCase, "snapshot.txt"), "utf-8");
}

// This test validates that the typescript version used by the closure tests
// is the same as the one used by the pulumi package and that we are testing
// what we think we are testing ...
it(`resolve to the correct typescript version within the pulumi package`,
    async function () {
        const { stdout } = await execa("npm", ["ls", "typescript", "--json"], { cwd: __dirname, reject: false });
        const deps = JSON.parse(stdout);
        const version = deps.dependencies["@pulumi/pulumi"].dependencies.typescript.version;
        assert.strictEqual(version, typescript.version);
    });

describe(`closure tests (TypeScript ${typescript.version})`, function () {
    const cases = readdirSync("cases"); // describe does not support async functions
    for (const testCase of cases) {
        const { func, isFactoryFunction, error: expectedError, description, allowSecrets, after } = require(`./cases/${testCase}`);

        const nodeMajor = parseInt(process.version.split(".")[0].slice(1));
        if (description === "Use webcrypto via global.crypto" && nodeMajor < 19) {
            // This test uses global.crypto, which is only available in Node 19 and later.
            continue;
        }

        it(`${description} (TypeScript ${typescript.version})`, async () => {
            if (expectedError) {
                await assert.rejects(async () => {
                    await runtime.serializeFunction(func, {
                        allowSecrets: allowSecrets ?? false,
                        isFactoryFunction: isFactoryFunction ?? false,
                    });
                }, err => {
                    const actual = anonymizeFunctionNames((<Error>err).message);
                    assert.strictEqual(actual, expectedError);
                    return true;
                });
            } else {
                const sf = await runtime.serializeFunction(func, {
                    allowSecrets: allowSecrets ?? false,
                    isFactoryFunction: isFactoryFunction ?? false,
                });
                if (after) {
                    after();
                }
                let snapshot = await getSnapshot(testCase, typescript.version);
                // Replace all new lines with \n to make the comparison platform independent.
                snapshot = snapshot.replace(platformIndependentEOL, "\n")
                const actual = sf.text.replace(platformIndependentEOL, "\n")
                assert.strictEqual(actual, snapshot);
            }
        });
    }
});

function anonymizeFunctionNames(text: string): string {
    return text.replace(/function '.+'/g, "function '<anonymous>'");
}

describe("mock package", () => {
    describe("module", () => {
        it("remaps exports correctly for mockpackage", () => {
            assert.strictEqual(pkg.getModuleFromPath("mockpackage/lib/index.js"), "mockpackage");
        });
        it("should return undefined on unexported members", () => {
            assert.throws(() => pkg.getModuleFromPath("mockpackage/lib/external.js"));
        });
    });
    describe("disregard null targets", () => {
        // ./node_modules/es-module-package/package.json
        const packagedef = {
            name: "es-module-package",
            exports: {
                "./features/private-internal-b/*": null,
                "./features/*": "./src/features/*.js",
                "./features/private-internal/*": null,
            },
        };
        it(`handles wildcard paths`, () => {
            assert.strictEqual(
                pkg.getModuleFromPath("es-module-package/src/features/private-inter.js", packagedef),
                "es-module-package/features/private-inter",
            );
            assert.strictEqual(
                pkg.getModuleFromPath("es-module-package/src/features/x.js", packagedef),
                "es-module-package/features/x",
            );
            assert.strictEqual(
                pkg.getModuleFromPath("es-module-package/src/features/y/z/foo/bar/baz.js", packagedef),
                "es-module-package/features/y/z/foo/bar/baz",
            );
        });
        it(`handles whitelisting blacklisted directories`, () => {
            assert.strictEqual(
                pkg.getModuleFromPath("es-module-package/features/internal/public/index.js", {
                    name: "es-module-package",
                    exports: {
                        "./features/internal/*": null,
                        ".": "./features/internal/public/index.js",
                    },
                }),
                "es-module-package",
            );
        });
    });
    describe("basic package exports", () => {
        // https://nodejs.org/api/packages.html#package-entry-points
        const packagedef = {
            name: "my-mod",
            exports: {
                ".": "./lib/index.js",
                "./lib": "./lib/index.js",
                "./lib/index": "./lib/index.js",
                "./lib/index.js": "./lib/index.js",
                "./feature": "./feature/index.js",
                "./feature/index.js": "./feature/index.js",
                "./package.json": "./package.json",
            },
        };
        it("handles multiple aliases 1", () => {
            assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/index.js", packagedef), "my-mod/lib/index.js");
        });
        it("handles multiple aliases 2", () => {
            assert.strictEqual(pkg.getModuleFromPath("my-mod/feature/index.js", packagedef), "my-mod/feature/index.js");
        });
        it("returns with no modification", () => {
            assert.strictEqual(pkg.getModuleFromPath("my-mod/package.json", packagedef), "my-mod/package.json");
        });
    });
    describe("wildcard package exports", () => {
        const packagedef = {
            name: "my-mod",
            exports: {
                ".": "./lib/index.js",
                "./lib": "./lib/index.js",
                "./lib/*": "./lib/*.js",
                "./feature": "./feature/index.js",
                "./feature/*": "./feature/*.js",
                "./package.json": "./package.json",
            },
        };
        it("wildcard module", () => {
            assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/foobar.js", packagedef), "my-mod/lib/foobar");
            assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/foo.js.js", packagedef), "my-mod/lib/foo.js"); // check
            assert.strictEqual(pkg.getModuleFromPath("my-mod/feature/foobar.js", packagedef), "my-mod/feature/foobar");
        });
        it("manual regression tests", () => {
            assert.strictEqual(
                pkg.getModuleFromPath("my-mod/internal/public/index.js.js", {
                    name: "my-mod",
                    exports: {
                        ".": "./internal/public/index.js",
                        "./public/*": "./internal/public/*.js",
                    },
                }),
                "my-mod/public/index.js",
            );
            assert.strictEqual(
                pkg.getModuleFromPath("my-mod/internal/public/index.js", {
                    name: "my-mod",
                    exports: {
                        ".": "./internal/public/index.js",
                        "./public/*": "./internal/public/*",
                    },
                }),
                "my-mod",
            );
        });
    });
    describe("conditional import/require package exports", () => {
        const packagedef = {
            // package.json
            name: "that-mod",
            exports: {
                ".": "./main.js",
                "./feature": {
                    node: "./feature-node.js",
                    default: "./feature.js",
                },
            },
            type: "module",
        };
        it("remaps conditional node/default nested packages", () => {
            assert.strictEqual(pkg.getModuleFromPath("that-mod/main.js", packagedef), "that-mod");
            assert.strictEqual(pkg.getModuleFromPath("that-mod/feature-node.js", packagedef), "that-mod/feature");
            assert.strictEqual(pkg.getModuleFromPath("that-mod/feature.js", packagedef), "that-mod/feature");
        });
    });
    describe("conditional import/require package exports", () => {
        const packagedef = {
            // package.json
            name: "this-mod",
            main: "./main-require.cjs",
            exports: {
                import: "./main-module.js",
                require: "./main-require.cjs",
            },
            type: "module",
        };
        it("remaps to main pkg", () => {
            assert.throws(() => pkg.getModuleFromPath("this-mod/main-module.js", packagedef));
            assert.strictEqual(pkg.getModuleFromPath("this-mod/main-require.cjs", packagedef), "this-mod");
        });
    });

    describe("error cases", () => {
        it("returns the original module if package.json not found", () => {
            assert.strictEqual(pkg.getModuleFromPath("this-mod/main-require.cjs"), "this-mod/main-require.cjs");
        });
    });
});
