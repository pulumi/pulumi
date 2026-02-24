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

import { CommandResult, fullyQualifiedStackName, LocalWorkspace, ProjectSettings, Stack } from "../../automation";
import { getTestOrg, getTestSuffix, withTestBackend } from "./util";
import { ComponentResource, ComponentResourceOptions } from "../../resource";
import { Config } from "../../config";

const userAgent = "pulumi/pulumi/test";

describe("LocalWorkspace - Stack", () => {
    it(`create/select/remove LocalWorkspace stack`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        await ws.createStack(stackName);
        await ws.selectStack(stackName);
        await ws.removeStack(stackName);
    });

    it(`create/select/createOrSelect Stack`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        await Stack.create(stackName, ws);
        await Stack.select(stackName, ws);
        await Stack.createOrSelect(stackName, ws);
        await ws.removeStack(stackName);
    });

    describe("Tag methods: get/set/remove/list", () => {
        const projectName = "testProjectName";
        const runtime = "nodejs";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime,
        };
        let workspace: LocalWorkspace;
        beforeEach(async () => {
            workspace = await LocalWorkspace.create(
                withTestBackend({
                    projectSettings: projectSettings,
                }),
            );
            await workspace.createStack(stackName);
        });

        it("lists tag values", async () => {
            if (!process.env.PULUMI_ACCESS_TOKEN) {
                console.log('Skipping "list tag values values" test');
                // Skip the test because the local backend doesn't support tags
                return;
            }
            const result = await workspace.listTags(stackName);
            assert.strictEqual(result["pulumi:project"], projectName);
            assert.strictEqual(result["pulumi:runtime"], runtime);
        });

        it("sets and removes tag values", async () => {
            if (!process.env.PULUMI_ACCESS_TOKEN) {
                console.log('Skipping "sets and removes tag values" test');
                // Skip the test because the local backend doesn't support tags
                return;
            }
            // sets
            await workspace.setTag(stackName, "foo", "bar");
            const actualValue = await workspace.getTag(stackName, "foo");
            assert.strictEqual(actualValue, "bar");
            // removes
            await workspace.removeTag(stackName, "foo");
            const actualTags = await workspace.listTags(stackName);
            assert.strictEqual(actualTags["foo"], undefined);
        });

        it("gets a single tag value", async () => {
            if (!process.env.PULUMI_ACCESS_TOKEN) {
                console.log('Skipping "gets a single tag value" test');
                // Skip the test because the local backend doesn't support tags
                return;
            }
            const actualValue = await workspace.getTag(stackName, "pulumi:project");
            assert.strictEqual(actualValue, actualValue.trim());
            assert.strictEqual(actualValue, projectName);
        });

        afterEach(async () => {
            await workspace.removeStack(stackName);
        });
    });

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

    it(`Environment functions`, async function () {
        // Skipping test because the required environments are in the moolumi org.
        if (getTestOrg() !== "moolumi") {
            this.skip();
            return;
        }
        const projectName = "node_env_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);

        // Adding non-existent env should fail.
        await assert.rejects(
            stack.addEnvironments("non-existent-env"),
            "stack.addEnvironments('non-existent-env') did not reject",
        );

        // Adding existing envs should succeed.
        await stack.addEnvironments("automation-api-test-env", "automation-api-test-env-2");

        let envs = await stack.listEnvironments();
        assert.deepStrictEqual(envs, ["automation-api-test-env", "automation-api-test-env-2"]);

        const config = await stack.getAllConfig();
        assert.strictEqual(config["node_env_test:new_key"].value, "test_value");
        assert.strictEqual(config["node_env_test:also"].value, "business");

        // Removing existing env should succeed.
        await stack.removeEnvironment("automation-api-test-env");
        envs = await stack.listEnvironments();
        assert.deepStrictEqual(envs, ["automation-api-test-env-2"]);

        const alsoConfig = await stack.getConfig("also");
        assert.strictEqual(alsoConfig.value, "business");
        await assert.rejects(stack.getConfig("new_key"), "stack.getConfig('new_key') did not reject");

        await stack.removeEnvironment("automation-api-test-env-2");
        envs = await stack.listEnvironments();
        assert.strictEqual(envs.length, 0);
        await assert.rejects(stack.getConfig("also"), "stack.getConfig('also') did not reject");

        await ws.removeStack(stackName);
    });

    it(`can list stacks and currently selected stack`, async () => {
        const projectName = `node_list_test${getTestSuffix()}`;
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackNamer = () => `int_test${getTestSuffix()}`;
        const stackNames: string[] = [];
        for (let i = 0; i < 2; i++) {
            const stackName = fullyQualifiedStackName(getTestOrg(), projectName, stackNamer());
            stackNames[i] = stackName;
            await Stack.create(stackName, ws);
            const stackSummary = await ws.stack();
            assert.strictEqual(stackSummary?.current, true);
            const stacks = await ws.listStacks();
            assert.strictEqual(stacks.length, i + 1);
        }

        for (const name of stackNames) {
            await ws.removeStack(name);
        }
    });

    it(`stack status methods`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);
        const history = await stack.history();
        assert.strictEqual(history.length, 0);
        const info = await stack.info();
        assert.strictEqual(typeof info, "undefined");
        await ws.removeStack(stackName);
    });

    it(`renames a stack`, async () => {
        const program = async () => {
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            new MyResource("res");
            return {};
        };

        const suffix = `int_test${getTestSuffix()}`;

        const stackName = fullyQualifiedStackName(getTestOrg(), "inline_node", suffix);
        let shortName = getTestOrg() + "/" + suffix;
        if (!process.env.PULUMI_ACCESS_TOKEN) {
            // If we are running with a filestate backend, there's no prefix in the name
            shortName = suffix;
        }

        const stackRenamed = stackName + "_renamed";
        const shortRenamed = shortName + "_renamed";

        const stack = await LocalWorkspace.createStack(
            { stackName, projectName: "inline_node", program },
            withTestBackend({}, "inline_node"),
        );

        await stack.up({ userAgent });
        stack.workspace.selectStack(stackName);

        let returned = "";
        const renameRes = await stack.rename({
            stackName: stackRenamed,
            onOutput: (e) => {
                returned += e;
            },
        });

        const after = (await stack.workspace.listStacks()).find((x) => x.name.startsWith(shortName));

        assert.strictEqual(returned, `Renamed ${shortName} to ${shortRenamed}\n`);
        assert.strictEqual(after?.name, shortRenamed);

        if (process.env.PULUMI_ACCESS_TOKEN) {
            // TODO: We don't have the right summary.kind for rename operations in the filestate backend.
            assert.strictEqual(renameRes.summary.kind, "rename");
        }
        assert.strictEqual(renameRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackRenamed);
    });

    it(`successfully initializes multiple stacks`, async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const projectName = "inline_node";
        const stackNames = Array.from(Array(10).keys()).map((_) =>
            fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`),
        );
        const stacks = await Promise.all(
            stackNames.map(async (stackName) =>
                LocalWorkspace.createStack({ stackName, projectName, program }, withTestBackend({}, "inline_node")),
            ),
        );
        await stacks.map((stack) => stack.workspace.removeStack(stack.name));
    });

    it(`imports and exports stacks`, async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const projectName = "import_export_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "import_export_node"),
        );

        try {
            await stack.setAllConfig({
                bar: { value: "abc" },
                buzz: { value: "secret", secret: true },
            });
            await stack.up();

            // export stack
            const state = await stack.exportStack();

            // import stack
            await stack.importStack(state);
            const configVal = await stack.getConfig("bar");
            assert.strictEqual(configVal.value, "abc");
        } finally {
            const destroyRes = await stack.destroy();
            assert.strictEqual(destroyRes.summary.kind, "destroy");
            assert.strictEqual(destroyRes.summary.result, "succeeded");
            await stack.workspace.removeStack(stackName);
        }
    });
});
