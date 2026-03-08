// Copyright 2016-2021, Pulumi Corporation.
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

import assert from "assert";
import * as semver from "semver";
import * as tmp from "tmp";

import { CommandResult, LocalWorkspace, PulumiCommand } from "../../automation";
import { withTestBackend } from "./util";

const versionRegex = /(\d+\.)(\d+\.)(\d+)(-.*)?/;

describe("LocalWorkspace - PulumiCommand", () => {
    it(`sets pulumi version`, async () => {
        const ws = await LocalWorkspace.create(withTestBackend({}));
        assert(ws.pulumiVersion);
        assert.strictEqual(versionRegex.test(ws.pulumiVersion), true);
    });

    it("sets pulumi version when using a custom CLI instance", async () => {
        const tmpDir = tmp.dirSync({ prefix: "automation-test-", unsafeCleanup: true });
        try {
            const cmd = await PulumiCommand.get();
            const ws = await LocalWorkspace.create(withTestBackend({ pulumiCommand: cmd }));
            assert.strictEqual(versionRegex.test(ws.pulumiVersion), true);
        } finally {
            tmpDir.removeCallback();
        }
    });

    it("throws when attempting to retrieve an invalid pulumi version", async () => {
        const mockWithNoVersion = {
            command: "pulumi",
            version: null,
            run: async () => new CommandResult("some output", "", 0),
        };
        const ws = await LocalWorkspace.create(
            withTestBackend({
                pulumiCommand: mockWithNoVersion,
                envVars: {
                    PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK: "true",
                },
            }),
        );
        assert.throws(() => ws.pulumiVersion);
    });

    it(`runs the install command`, async () => {
        let recordedArgs: string[] = [];
        const mockCommand = {
            command: "pulumi",
            // Version high enough to support --use-language-version-tools
            version: semver.parse("3.130.0"),
            run: async (
                args: string[],
                cwd: string,
                additionalEnv: { [key: string]: string },
                onOutput?: (data: string) => void,
            ): Promise<CommandResult> => {
                recordedArgs = args;
                return new CommandResult("some output", "", 0);
            },
        };
        const ws = await LocalWorkspace.create(withTestBackend({ pulumiCommand: mockCommand }));

        await ws.install();
        assert.deepStrictEqual(recordedArgs, ["install"]);

        await ws.install({ noPlugins: true });
        assert.deepStrictEqual(recordedArgs, ["install", "--no-plugins"]);

        await ws.install({ noDependencies: true });
        assert.deepStrictEqual(recordedArgs, ["install", "--no-dependencies"]);

        await ws.install({ reinstall: true });
        assert.deepStrictEqual(recordedArgs, ["install", "--reinstall"]);

        await ws.install({ useLanguageVersionTools: true });
        assert.deepStrictEqual(recordedArgs, ["install", "--use-language-version-tools"]);

        await ws.install({
            noDependencies: true,
            noPlugins: true,
            reinstall: true,
            useLanguageVersionTools: true,
        });
        assert.deepStrictEqual(recordedArgs, [
            "install",
            "--use-language-version-tools",
            "--no-plugins",
            "--no-dependencies",
            "--reinstall",
        ]);
    });

    it(`install requires version >= 3.91`, async () => {
        const mockCommand = {
            command: "pulumi",
            version: semver.parse("3.90.0"),
            run: async (
                args: string[],
                cwd: string,
                additionalEnv: { [key: string]: string },
                onOutput?: (data: string) => void,
            ): Promise<CommandResult> => {
                return new CommandResult("some output", "", 0);
            },
        };
        const ws = await LocalWorkspace.create(withTestBackend({ pulumiCommand: mockCommand }));

        await assert.rejects(() => ws.install());
    });

    it(`install --use-language-version-tools requires version >= 3.130`, async () => {
        const mockCommand = {
            command: "pulumi",
            version: semver.parse("3.129.0"),
            _pulumiVersion: semver.parse("3.129.0"),
            run: async (
                args: string[],
                cwd: string,
                additionalEnv: { [key: string]: string },
                onOutput?: (data: string) => void,
            ): Promise<CommandResult> => {
                return new CommandResult("some output", "", 0);
            },
        };
        const ws = await LocalWorkspace.create(withTestBackend({ pulumiCommand: mockCommand }));

        await assert.rejects(() => ws.install({ useLanguageVersionTools: true }));
    });
});
