// Copyright 2016-2025, Pulumi Corporation.
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
import * as upath from "upath";
import * as fs from "fs";

import {
    CommandResult,
    ConfigMap,
    ConfigValue,
    fullyQualifiedStackName,
    LocalWorkspace,
    LocalWorkspaceOptions,
    PulumiCommand,
} from "../../automation";
import { getTestOrg, getTestSuffix } from "./util";

describe("Config Options Tests", () => {
    const stackName = "test-stack";

    describe("Command Argument Verification", () => {
        it("passes correct arguments to getConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult('{"value":"test-value","secret":false}', "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.getConfigWithOptions(stackName, "test-key", { path: true, configFile: "/path/to/config.yaml" });

            assert.deepStrictEqual(capturedArgs, [
                "config",
                "get",
                "--path",
                "--config-file",
                "/path/to/config.yaml",
                "test-key",
                "--json",
                "--stack",
                stackName,
            ]);
        });

        it("passes correct arguments to getAllConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.getAllConfigWithOptions(stackName, { showSecrets: true, configFile: "/path/to/config.yaml" });

            assert.deepStrictEqual(capturedArgs, [
                "config",
                "--config-file",
                "/path/to/config.yaml",
                "--show-secrets",
                "--json",
                "--stack",
                stackName,
            ]);
        });

        it("passes correct arguments to setConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult("", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.setConfigWithOptions(
                stackName,
                "test-key",
                { value: "test-value" },
                { path: true, configFile: "/path/to/config.yaml" },
            );

            assert.deepStrictEqual(capturedArgs, [
                "config",
                "set",
                "--path",
                "--config-file",
                "/path/to/config.yaml",
                "test-key",
                "--stack",
                stackName,
                "--plaintext",
                "--non-interactive",
                "--",
                "test-value",
            ]);
        });

        it("passes correct arguments to setAllConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult("", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.setAllConfigWithOptions(
                stackName,
                {
                    key1: { value: "value1" },
                    key2: { value: "value2", secret: true },
                },
                { configFile: "/path/to/config.yaml" },
            );

            // We need to check that the arguments contain the expected values, but the order of the key-value pairs might vary
            assert.ok(capturedArgs.includes("config"));
            assert.ok(capturedArgs.includes("set-all"));
            assert.ok(capturedArgs.includes("--stack"));
            assert.ok(capturedArgs.includes(stackName));
            assert.ok(capturedArgs.includes("--config-file"));
            assert.ok(capturedArgs.includes("/path/to/config.yaml"));
            assert.ok(capturedArgs.includes("--plaintext"));
            assert.ok(capturedArgs.includes("key1=value1"));
            assert.ok(capturedArgs.includes("--secret"));
            assert.ok(capturedArgs.includes("key2=value2"));

            // Test with path parameter (should be ignored for set-all)
            capturedArgs = [];
            await ws.setAllConfigWithOptions(
                stackName,
                {
                    key1: { value: "value1" },
                },
                { path: true, configFile: "/path/to/config.yaml" },
            );

            // Verify --path is not included
            assert.ok(!capturedArgs.includes("--path"));
        });

        it("passes correct arguments to removeConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult("", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.removeConfigWithOptions(stackName, "test-key", { path: true, configFile: "/path/to/config.yaml" });

            assert.deepStrictEqual(capturedArgs, [
                "config",
                "rm",
                "test-key",
                "--stack",
                stackName,
                "--path",
                "--config-file",
                "/path/to/config.yaml",
            ]);
        });

        it("passes correct arguments to removeAllConfigWithOptions", async () => {
            let capturedArgs: string[] = [];
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;
                    return new CommandResult("", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await ws.removeAllConfigWithOptions(stackName, ["key1", "key2"], {
                path: true,
                configFile: "/path/to/config.yaml",
            });

            assert.deepStrictEqual(capturedArgs, [
                "config",
                "rm-all",
                "--stack",
                stackName,
                "--path",
                "--config-file",
                "/path/to/config.yaml",
                "key1",
                "key2",
            ]);
        });
    });

    describe("Error Handling", () => {
        it("properly handles errors in getAllConfigWithOptions", async () => {
            const errorMessage = "error getting config";
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async () => new CommandResult("", errorMessage, 1, new Error("Command failed")),
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await assert.rejects(ws.getAllConfigWithOptions(stackName, { showSecrets: true }), (err: Error) => {
                // The error message should be the stderr output from the command
                return err.message === errorMessage;
            });
        });

        it("handles malformed JSON in getConfigWithOptions response", async () => {
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async () => new CommandResult("invalid json", "", 0),
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            await assert.rejects(ws.getConfigWithOptions(stackName, "test-key", {}), /Unexpected token/);
        });
    });

    describe("Mock Integration Tests", () => {
        it("mocks config file operations - gets config from custom file", async () => {
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    // Check if the command is using the config file
                    if (args.includes("--config-file") && args.includes("from-file")) {
                        return new CommandResult('{"value":"file-value","secret":false}', "", 0);
                    }
                    // Return empty for other commands
                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            const value = await ws.getConfigWithOptions(stackName, "from-file", { configFile: "/path/to/config.yaml" });

            assert.strictEqual(value.value, "file-value");
            assert.strictEqual(value.secret, false);
        });

        it("mocks setting and getting config with custom file", async () => {
            let setConfigCalled = false;
            let configFile: string | undefined;
            let configValue: string | undefined;

            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    if (args.includes("set") && args.includes("--config-file")) {
                        setConfigCalled = true;
                        configFile = args[args.indexOf("--config-file") + 1];
                        configValue = args[args.length - 1];
                        return new CommandResult("", "", 0);
                    }

                    if (args.includes("get") && args.includes("--config-file")) {
                        if (setConfigCalled && args.includes("custom-key")) {
                            return new CommandResult('{"value":"custom-value","secret":false}', "", 0);
                        }
                    }

                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });

            // Set value
            await ws.setConfigWithOptions(
                stackName,
                "custom-key",
                { value: "custom-value" },
                { configFile: "/path/to/config.yaml" },
            );

            // Verify the command was called correctly
            assert.strictEqual(setConfigCalled, true);
            assert.strictEqual(configFile, "/path/to/config.yaml");
            assert.strictEqual(configValue, "custom-value");

            // Get value back
            const value = await ws.getConfigWithOptions(stackName, "custom-key", {
                configFile: "/path/to/config.yaml",
            });

            assert.strictEqual(value.value, "custom-value");
            assert.strictEqual(value.secret, false);
        });

        it("mocks removing config with custom file", async () => {
            let cmdArgs: string[] = [];
            let removeCalled = false;

            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    cmdArgs = args;

                    if (args.includes("get") && args.includes("from-file") && !removeCalled) {
                        return new CommandResult('{"value":"file-value","secret":false}', "", 0);
                    }

                    // Return success for remove operation
                    if (args.includes("rm") && args.includes("from-file")) {
                        removeCalled = true;
                        return new CommandResult("", "", 0);
                    }

                    // If getting the value after removal, return error
                    if (args.includes("get") && args.includes("from-file") && removeCalled) {
                        return new CommandResult("", "error: config not found", 1, new Error("Config not found"));
                    }

                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });

            // First get the value (should succeed)
            const initialValue = await ws.getConfigWithOptions(stackName, "from-file", {
                configFile: "/path/to/config.yaml",
            });
            assert.strictEqual(initialValue.value, "file-value");

            // Remove the value
            await ws.removeConfigWithOptions(stackName, "from-file", { configFile: "/path/to/config.yaml" });

            // Verify the remove command was called with the right args
            assert.ok(cmdArgs.includes("rm"));
            assert.ok(cmdArgs.includes("from-file"));
            assert.ok(cmdArgs.includes("--config-file"));
            assert.ok(cmdArgs.includes("/path/to/config.yaml"));

            // Getting the value should now fail
            try {
                await ws.getConfigWithOptions(stackName, "from-file", { configFile: "/path/to/config.yaml" });
                assert.fail("Expected getConfigWithOptions to throw");
            } catch (err) {
                assert.ok(err);
            }
        });

        it("mocks path parameter for config operations", async () => {
            const configValues: Record<string, string> = {};
            let configDeleted: boolean = false;

            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    // Mock set with path
                    if (args.includes("set") && args.includes("--path")) {
                        const key = args[args.indexOf("set") + 1];
                        const val = args[args.length - 1];
                        configValues[key] = val;
                        return new CommandResult("", "", 0);
                    }

                    // Mock get with path
                    if (args.includes("get") && args.includes("--path")) {
                        const key = args[args.indexOf("get") + 1];
                        if (configValues[key] && !configDeleted) {
                            return new CommandResult(`{"value":"${configValues[key]}","secret":false}`, "", 0);
                        }
                        return new CommandResult("", "error: config not found", 1, new Error("Config not found"));
                    }

                    // Mock remove with path
                    if (args.includes("rm") && args.includes("--path")) {
                        configDeleted = true;
                        return new CommandResult("", "", 0);
                    }

                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });

            // Test nested config with path parameter
            await ws.setConfigWithOptions(stackName, "nested.value", { value: "test-nested" }, { path: true });

            // Get it back
            const value = await ws.getConfigWithOptions(stackName, "nested.value", { path: true });
            assert.strictEqual(value.value, "test-nested");
            assert.strictEqual(value.secret, false);

            // Remove it
            await ws.removeConfigWithOptions(stackName, "nested.value", { path: true });

            // Verify it's gone
            try {
                await ws.getConfigWithOptions(stackName, "nested.value", { path: true });
                assert.fail("Expected getConfigWithOptions to throw");
            } catch (err) {
                assert.ok(err);
            }
        });

        it("mocks combined path and configFile operations", async () => {
            const mockValuesByFile: Record<string, Record<string, string>> = {
                custom: {},
                default: {},
            };

            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    const fileKey = args.includes("--config-file") ? "custom" : "default";

                    if (args.includes("set") && args.includes("--path")) {
                        const key = args[args.indexOf("set") + 1];
                        const value = args[args.length - 1];
                        mockValuesByFile[fileKey][key] = value;
                        return new CommandResult("", "", 0);
                    }

                    if (args.includes("get") && args.includes("--path")) {
                        const key = args[args.indexOf("get") + 1];
                        if (mockValuesByFile[fileKey][key]) {
                            return new CommandResult(
                                `{"value":"${mockValuesByFile[fileKey][key]}","secret":false}`,
                                "",
                                0,
                            );
                        }
                        return new CommandResult("", "error: config not found", 1, new Error("Config not found"));
                    }

                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });

            // Set nested config in the custom file
            await ws.setConfigWithOptions(
                stackName,
                "obj.nested",
                { value: "nested-in-custom-file" },
                { path: true, configFile: "/path/to/config.yaml" },
            );

            // Set similar value in default config
            await ws.setConfigWithOptions(stackName, "obj.nested", { value: "nested-in-default-file" }, { path: true });

            // Verify each file has the correct value
            const customValue = await ws.getConfigWithOptions(stackName, "obj.nested", {
                path: true,
                configFile: "/path/to/config.yaml",
            });
            assert.strictEqual(customValue.value, "nested-in-custom-file");

            const defaultValue = await ws.getConfigWithOptions(stackName, "obj.nested", { path: true });
            assert.strictEqual(defaultValue.value, "nested-in-default-file");
        });

        it("handles special characters in config keys and values", async () => {
            let capturedArgs: string[] = [];

            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async (args: string[]) => {
                    capturedArgs = args;

                    if (args.includes("set")) {
                        return new CommandResult("", "", 0);
                    }

                    if (args.includes("get")) {
                        // When getting the value back, return the special value
                        const testKey = "special:key.with[characters]";
                        const testValue = "value with spaces & special chars !@#$%";

                        if (args.includes(testKey)) {
                            return new CommandResult(`{"value":"${testValue}","secret":false}`, "", 0);
                        }
                    }

                    return new CommandResult("{}", "", 0);
                },
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });

            // Set config with special characters
            const specialKey = "special:key.with[characters]";
            const specialValue = "value with spaces & special chars !@#$%";

            await ws.setConfigWithOptions(stackName, specialKey, { value: specialValue }, { path: true });

            // Verify the key was passed through the command
            assert.ok(capturedArgs.includes(specialKey));
            assert.ok(capturedArgs.includes("--"));
            assert.strictEqual(capturedArgs[capturedArgs.length - 1], specialValue);

            // Get it back
            const configValue = await ws.getConfigWithOptions(stackName, specialKey, { path: true });

            assert.strictEqual(configValue.value, specialValue);
        });
    });

    describe("Edge Cases", () => {
        it("handles empty config object in getAllConfigWithOptions", async () => {
            const mockCommand = {
                command: "pulumi",
                version: semver.parse("3.0.0"),
                run: async () => new CommandResult("{}", "", 0),
            };

            const ws = await LocalWorkspace.create({ pulumiCommand: mockCommand });
            const result = await ws.getAllConfigWithOptions(stackName, {});

            assert.deepStrictEqual(result, {});
        });
    });

    describe("Stack Method Delegation", () => {
        it("stack.getConfigWithOptions delegates to workspace.getConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedKey: string | undefined;
            let capturedOpts: any;

            const mockWorkspace = {
                getConfigWithOptions: async (stackArg: string, key: string, opts?: any) => {
                    capturedStackName = stackArg;
                    capturedKey = key;
                    capturedOpts = opts;
                    return { value: "mocked", secret: false };
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                getConfigWithOptions: async (key: string, opts?: any) => {
                    return mockWorkspace.getConfigWithOptions(stack.name, key, opts);
                },
            };

            await stack.getConfigWithOptions("test-key", { path: true, configFile: "test.yaml" });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.strictEqual(capturedKey, "test-key");
            assert.deepStrictEqual(capturedOpts, { path: true, configFile: "test.yaml" });
        });

        it("stack.getAllConfigWithOptions delegates to workspace.getAllConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedOpts: any;

            const mockWorkspace = {
                getAllConfigWithOptions: async (stackArg: string, opts?: any) => {
                    capturedStackName = stackArg;
                    capturedOpts = opts;
                    return { "test:key": { value: "mocked", secret: false } };
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                getAllConfigWithOptions: async (opts?: any) => {
                    return mockWorkspace.getAllConfigWithOptions(stack.name, opts);
                },
            };

            await stack.getAllConfigWithOptions({ showSecrets: true, configFile: "test.yaml" });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.deepStrictEqual(capturedOpts, { showSecrets: true, configFile: "test.yaml" });
        });

        it("stack.setConfigWithOptions delegates to workspace.setConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedKey: string | undefined;
            let capturedValue: any;
            let capturedOpts: any;

            const mockWorkspace = {
                setConfigWithOptions: async (stackArg: string, key: string, val: any, opts?: any) => {
                    capturedStackName = stackArg;
                    capturedKey = key;
                    capturedValue = val;
                    capturedOpts = opts;
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                setConfigWithOptions: async (key: string, value: any, opts?: any) => {
                    return mockWorkspace.setConfigWithOptions(stack.name, key, value, opts);
                },
            };

            const configValue = { value: "test", secret: true };
            await stack.setConfigWithOptions("test-key", configValue, { path: true });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.strictEqual(capturedKey, "test-key");
            assert.deepStrictEqual(capturedValue, configValue);
            assert.deepStrictEqual(capturedOpts, { path: true });
        });

        it("stack.setAllConfigWithOptions delegates to workspace.setAllConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedConfig: any;
            let capturedOpts: any;

            const mockWorkspace = {
                setAllConfigWithOptions: async (stackArg: string, conf: any, opts?: any) => {
                    capturedStackName = stackArg;
                    capturedConfig = conf;
                    capturedOpts = opts;
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                setAllConfigWithOptions: async (config: any, opts?: any) => {
                    return mockWorkspace.setAllConfigWithOptions(stack.name, config, opts);
                },
            };

            const configMap = {
                key1: { value: "value1" },
                key2: { value: "value2", secret: true },
            };

            await stack.setAllConfigWithOptions(configMap, { configFile: "test.yaml" });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.deepStrictEqual(capturedConfig, configMap);
            assert.deepStrictEqual(capturedOpts, { configFile: "test.yaml" });
        });

        it("stack.removeConfigWithOptions delegates to workspace.removeConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedKey: string | undefined;
            let capturedOpts: any;

            const mockWorkspace = {
                removeConfigWithOptions: async (stackArg: string, k: string, opts?: any) => {
                    capturedStackName = stackArg;
                    capturedKey = k;
                    capturedOpts = opts;
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                removeConfigWithOptions: async (key: string, opts?: any) => {
                    return mockWorkspace.removeConfigWithOptions(stack.name, key, opts);
                },
            };

            await stack.removeConfigWithOptions("test-key", { path: true });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.strictEqual(capturedKey, "test-key");
            assert.deepStrictEqual(capturedOpts, { path: true });
        });

        it("stack.removeAllConfigWithOptions delegates to workspace.removeAllConfigWithOptions", async () => {
            let capturedStackName: string | undefined;
            let capturedKeys: string[] | undefined;
            let capturedOpts: any;

            const mockWorkspace = {
                removeAllConfigWithOptions: async (stackArg: string, keyArray: string[], opts?: any) => {
                    capturedStackName = stackArg;
                    capturedKeys = keyArray;
                    capturedOpts = opts;
                },
            };

            const stack = {
                name: "test-stack",
                workspace: mockWorkspace,
                removeAllConfigWithOptions: async (keysToRemove: string[], opts?: any) => {
                    return mockWorkspace.removeAllConfigWithOptions(stack.name, keysToRemove, opts);
                },
            };

            const keys = ["key1", "key2"];
            await stack.removeAllConfigWithOptions(keys, { path: true, configFile: "test.yaml" });

            assert.strictEqual(capturedStackName, "test-stack");
            assert.deepStrictEqual(capturedKeys, keys);
            assert.deepStrictEqual(capturedOpts, { path: true, configFile: "test.yaml" });
        });
    });
});
