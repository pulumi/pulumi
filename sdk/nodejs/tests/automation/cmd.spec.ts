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
import { PulumiCommand, parseAndValidatePulumiVersion } from "../../automation";

describe("automation/cmd", () => {
    it("calls onOutput when provided to runPulumiCmd", async () => {
        let output = "";
        let numCalls = 0;
        const pulumi = await PulumiCommand.get();
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
                const cmd = await PulumiCommand.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                assert.doesNotThrow(() => fs.statSync(upath.join(tmpDir.name, "bin", "pulumi")));
                const { stdout } = await cmd.run(["version"], ".", {});
                assert.strictEqual(stdout.trim(), "3.97.0");
            } finally {
                tmpDir.removeCallback();
            }
        });

        it("does not re-install the version if it already exists", async () => {
            const tmpDir = tmp.dirSync({ prefix: "automation-test-", unsafeCleanup: true });
            try {
                await PulumiCommand.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                const binary1 = fs.statSync(upath.join(tmpDir.name, "bin", "pulumi"));
                await PulumiCommand.install({ root: tmpDir.name, version: new semver.SemVer("3.97.0") });
                const binary2 = fs.statSync(upath.join(tmpDir.name, "bin", "pulumi"));
                assert.strictEqual(binary1.ino, binary2.ino);
            } finally {
                tmpDir.removeCallback();
            }
        });

        it("defaults to $HOME/.pulumi/versions/$VERSION if no root is provided", async () => {
            const version = new semver.SemVer("3.97.0");
            const cmd = await PulumiCommand.install({ version });
            assert.doesNotThrow(() =>
                fs.statSync(upath.join(os.homedir(), ".pulumi", "versions", `${version}`, "bin", "pulumi")),
            );
            const { stdout } = await cmd.run(["version"], ".", {});
            assert.strictEqual(stdout.trim(), `${version}`);
        });

        it("throws if the found version is not compatible with the requested version", async () => {
            const installedVersion = new semver.SemVer("3.97.0");
            await PulumiCommand.install({ version: installedVersion });
            const requestedVersion = new semver.SemVer("3.98.0");
            assert.rejects(PulumiCommand.get({ version: requestedVersion }));
            assert.doesNotReject(PulumiCommand.get({ version: installedVersion, skipVersionCheck: true }));
        });
    });

    describe(`checkVersionIsValid`, () => {
        const MAJOR = /Major version mismatch./;
        const MINIMUM = /Minimum version requirement failed./;
        const PARSE = /Failed to parse/;
        const versionTests = [
            {
                name: "higher_major",
                currentVersion: "100.0.0",
                expectError: MAJOR,
                optOut: false,
            },
            {
                name: "lower_major",
                currentVersion: "1.0.0",
                expectError: MINIMUM,
                optOut: false,
            },
            {
                name: "higher_minor",
                currentVersion: "v2.22.0",
                expectError: null,
                optOut: false,
            },
            {
                name: "lower_minor",
                currentVersion: "v2.1.0",
                expectError: MINIMUM,
                optOut: false,
            },
            {
                name: "equal_minor_higher_patch",
                currentVersion: "v2.21.2",
                expectError: null,
                optOut: false,
            },
            {
                name: "equal_minor_equal_patch",
                currentVersion: "v2.21.1",
                expectError: null,
                optOut: false,
            },
            {
                name: "equal_minor_lower_patch",
                currentVersion: "v2.21.0",
                expectError: MINIMUM,
                optOut: false,
            },
            {
                name: "equal_minor_equal_patch_prerelease",
                // Note that prerelease < release so this case will error
                currentVersion: "v2.21.1-alpha.1234",
                expectError: MINIMUM,
                optOut: false,
            },
            {
                name: "opt_out_of_check_would_fail_otherwise",
                currentVersion: "v2.20.0",
                expectError: null,
                optOut: true,
            },
            {
                name: "opt_out_of_check_would_succeed_otherwise",
                currentVersion: "v2.22.0",
                expectError: null,
                optOut: true,
            },
            {
                name: "invalid_version",
                currentVersion: "invalid",
                expectError: PARSE,
                optOut: false,
            },
            {
                name: "invalid_version_opt_out",
                currentVersion: "invalid",
                expectError: null,
                optOut: true,
            },
        ];
        const minVersion = new semver.SemVer("v2.21.1");

        versionTests.forEach((test) => {
            it(`validates ${test.name} (${test.currentVersion})`, () => {
                const validate = () => parseAndValidatePulumiVersion(minVersion, test.currentVersion, test.optOut);
                if (test.expectError) {
                    assert.throws(validate, test.expectError);
                } else {
                    assert.doesNotThrow(validate);
                }
            });
        });
    });
});
