// Copyright 2016-2022, Pulumi Corporation.
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
import * as semver from "semver";
import * as os from "os";
import * as tmp from "tmp";
import * as upath from "upath";
import { Pulumi } from "../../automation";

describe("automation/cmd", () => {
    it("calls onOutput when provided to runPulumiCmd", async () => {
        let output = "";
        let numCalls = 0;
        const pulumi = await Pulumi.get();
        await pulumi.run(["--help"], ".", {}, (data: string) => {
            output += data;
            numCalls += 1;
        });
        assert.ok(numCalls > 0, `expected numCalls > 0, got ${numCalls}`);
        assert.match(output, new RegExp("Usage[:]"));
        assert.match(output, new RegExp("[-][-]verbose"));
    });

    describe("CLI installation", () => {
        it("installs the requested version", async () => {
            const tmpDir = tmp.dirSync({ prefix: "automation-test-", unsafeCleanup: true });
            try {
                const pulumi = await Pulumi.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                assert.doesNotThrow(() => fs.statSync(upath.join(tmpDir.name, "bin", "pulumi")));
                const { stdout } = await pulumi.run(["version"], ".", {});
                assert.strictEqual(stdout.trim(), "3.97.0");
            } finally {
                tmpDir.removeCallback();
            }
        });

        it("does not re-install the version if it already exists", async () => {
            const tmpDir = tmp.dirSync({ prefix: "automation-test-", unsafeCleanup: true });
            try {
                await Pulumi.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                const binary1 = fs.statSync(upath.join(tmpDir.name, "bin", "pulumi"));
                await Pulumi.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                const binary2 = fs.statSync(upath.join(tmpDir.name, "bin", "pulumi"));
                assert.strictEqual(binary1.ino, binary2.ino);
            } finally {
                tmpDir.removeCallback();
            }
        });

        it("defaults to $HOME/.pulumi/versions/$VERSION if no root is provided", async () => {
            const version = new semver.SemVer("3.97.0");
            const pulumi = await Pulumi.install({ version });
            assert.doesNotThrow(() =>
                fs.statSync(upath.join(os.homedir(), ".pulumi", "versions", `${version}`, "bin", "pulumi")),
            );
            const { stdout } = await pulumi.run(["version"], ".", {});
            assert.strictEqual(stdout.trim(), `${version}`);
        });
    });
});
