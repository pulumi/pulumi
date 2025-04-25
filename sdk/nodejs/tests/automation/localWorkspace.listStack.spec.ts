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

import assert from "assert";

import {
    CommandResult,
    LocalWorkspace,
} from "../../automation";

import { withTestBackend } from "./localWorkspace.spec";

describe("ListStack Methods", async () => {
    describe("ListStacks", async () => {
        const stackJson = `[
                    {
                        "name": "testorg1/testproj1/teststack1",
                        "current": false,
                        "url": "https://app.pulumi.com/testorg1/testproj1/teststack1"
                    },
                    {
                        "name": "testorg1/testproj1/teststack2",
                        "current": false,
                        "url": "https://app.pulumi.com/testorg1/testproj1/teststack2"
                    }
                ]`;
        it(`should handle stacks correctly for listStacks`, async () => {
            const mockWithReturnedStacks = {
                command: "pulumi",
                version: null,
                run: async (args: string[], cwd: string, additionalEnv: { [key: string]: string }) => {
                    return new CommandResult(stackJson, "", 0);
                },
            };

            const workspace = await LocalWorkspace.create(
                withTestBackend({ pulumiCommand: mockWithReturnedStacks }),
            );
            const stacks = await workspace.listStacks();

            assert.strictEqual(stacks.length, 2);
            assert.strictEqual(stacks[0].name, "testorg1/testproj1/teststack1");
            assert.strictEqual(stacks[0].current, false);
            assert.strictEqual(stacks[0].url, "https://app.pulumi.com/testorg1/testproj1/teststack1");
            assert.strictEqual(stacks[1].name, "testorg1/testproj1/teststack2");
            assert.strictEqual(stacks[1].current, false);
            assert.strictEqual(stacks[1].url, "https://app.pulumi.com/testorg1/testproj1/teststack2");
        });

        it(`should use correct args for listStacks`, async () => {
            let capturedArgs: string[] = [];
            const mockPulumiCommand = {
                command: "pulumi",
                version: null,
                run: async (args: string[], cwd: string, additionalEnv: { [key: string]: string }) => {
                    capturedArgs = args;
                    return new CommandResult(stackJson, "", 0);
                },
            };
            const workspace = await LocalWorkspace.create(
                withTestBackend({
                    pulumiCommand: mockPulumiCommand,
                }),
            );
            await workspace.listStacks();
            assert.deepStrictEqual(capturedArgs, ["stack", "ls", "--json"]);
        });
    });

    describe("ListStacks with all", async () => {
        const stackJson = `[
                    {
                        "name": "testorg1/testproj1/teststack1",
                        "current": false,
                        "url": "https://app.pulumi.com/testorg1/testproj1/teststack1"
                    },
                    {
                        "name": "testorg1/testproj2/teststack2",
                        "current": false,
                        "url": "https://app.pulumi.com/testorg1/testproj2/teststack2"
                    }
                ]`;
        it(`should handle stacks correctly for listStacks when all is set`, async () => {
            const mockWithReturnedStacks = {
                command: "pulumi",
                version: null,
                run: async () => new CommandResult(stackJson, "", 0),
            };
            const workspace = await LocalWorkspace.create(
                withTestBackend({
                    pulumiCommand: mockWithReturnedStacks,
                }),
            );
            const stacks = await workspace.listStacks({ all: true });
            assert.strictEqual(stacks.length, 2);
            assert.strictEqual(stacks[0].name, "testorg1/testproj1/teststack1");
            assert.strictEqual(stacks[0].current, false);
            assert.strictEqual(stacks[0].url, "https://app.pulumi.com/testorg1/testproj1/teststack1");
            assert.strictEqual(stacks[1].name, "testorg1/testproj2/teststack2");
            assert.strictEqual(stacks[1].current, false);
            assert.strictEqual(stacks[1].url, "https://app.pulumi.com/testorg1/testproj2/teststack2");
        });

        it(`should use correct args for listStacks when all is set`, async () => {
            let capturedArgs: string[] = [];
            const mockPuluiCommand = {
                command: "pulumi",
                version: null,
                run: async (args: string[], cwd: string, additionalEnv: { [key: string]: string }) => {
                    capturedArgs = args;
                    return new CommandResult(stackJson, "", 0);
                },
            };
            const workspace = await LocalWorkspace.create(
                withTestBackend({
                    pulumiCommand: mockPuluiCommand,
                }),
            );
            await workspace.listStacks({ all: true });
            assert.deepStrictEqual(capturedArgs, ["stack", "ls", "--json", "--all"]);
        });
    });
});
