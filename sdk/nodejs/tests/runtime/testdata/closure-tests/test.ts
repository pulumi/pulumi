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
import * as fs from "fs/promises";
import { readdirSync } from "fs";
import * as path from "path";
import * as semver from "semver";
import * as typescript from "typescript";
// @ts-ignore: The test installs @pulumi/pulumi
import { runtime } from "@pulumi/pulumi";

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
                    assert.strictEqual((<Error>err).message, expectedError);
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
