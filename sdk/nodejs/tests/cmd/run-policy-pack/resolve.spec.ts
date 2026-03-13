// Copyright 2025, Pulumi Corporation.
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
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { resolveProgramPath } from "../../../cmd/run-policy-pack/run";

/**
 * Tests for resolveProgramPath from run-policy-pack/run.ts.
 * Verifies that a directory policy pack is resolved correctly even when
 * a sibling .json file exists. See https://github.com/pulumi/pulumi/issues/4280
 */
describe("policy pack directory resolution", () => {
    let tmpDir: string;

    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "policy-resolve-test-"));
    });

    afterEach(() => {
        fs.rmSync(tmpDir, { recursive: true, force: true });
    });

    it("resolves directory with package.json main field", () => {
        const policyDir = path.join(tmpDir, "policy");
        fs.mkdirSync(policyDir);
        fs.writeFileSync(
            path.join(policyDir, "package.json"),
            JSON.stringify({ main: "lib/index.js" }),
        );
        // Create sibling .json file (the trigger for #4280)
        fs.writeFileSync(path.join(tmpDir, "policy.json"), "{}");

        const resolved = resolveProgramPath(policyDir);
        assert.strictEqual(resolved, path.join(policyDir, "lib", "index.js"));
    });

    it("resolves directory without main field to index", () => {
        const policyDir = path.join(tmpDir, "policy");
        fs.mkdirSync(policyDir);
        fs.writeFileSync(
            path.join(policyDir, "package.json"),
            JSON.stringify({ name: "my-policy" }),
        );
        fs.writeFileSync(path.join(tmpDir, "policy.json"), "{}");

        const resolved = resolveProgramPath(policyDir);
        assert.strictEqual(resolved, path.join(policyDir, "index"));
    });

    it("resolves directory without package.json to index", () => {
        const policyDir = path.join(tmpDir, "policy");
        fs.mkdirSync(policyDir);
        fs.writeFileSync(path.join(tmpDir, "policy.json"), "{}");

        const resolved = resolveProgramPath(policyDir);
        assert.strictEqual(resolved, path.join(policyDir, "index"));
    });

    it("leaves non-directory paths unchanged", () => {
        const filePath = path.join(tmpDir, "policy.js");
        fs.writeFileSync(filePath, "module.exports = {}");

        const resolved = resolveProgramPath(filePath);
        assert.strictEqual(resolved, filePath);
    });

    it("leaves non-existent paths unchanged", () => {
        const missingPath = path.join(tmpDir, "nonexistent");

        const resolved = resolveProgramPath(missingPath);
        assert.strictEqual(resolved, missingPath);
    });
});
