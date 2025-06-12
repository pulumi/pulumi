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
import * as upath from "upath";
import * as fs from "fs";

import {
    CommandResult,
    ConfigMap,
    EngineEvent,
    fullyQualifiedStackName,
    LocalWorkspace,
    LocalWorkspaceOptions,
    OutputMap,
    ProjectSettings,
    ProjectRuntime,
    PulumiCommand,
    Stack,
} from "../../automation";
import { CustomResource, ComponentResource, ComponentResourceOptions, Config, output } from "../../index";
import { getTestOrg, getTestSuffix } from "./util";

const versionRegex = /(\d+\.)(\d+\.)(\d+)(-.*)?/;
const userAgent = "pulumi/pulumi/test";

describe("LocalWorkspace", () => {
    it(`projectSettings from yaml/yml/json`, async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create(
                withTestBackend(
                    { workDir: upath.joinSafe(__dirname, "data", ext) },
                    "testproj",
                    "A minimal Go Pulumi program",
                    "go",
                ),
            );
            const settings = await ws.projectSettings();
            assert.strictEqual(settings.name, "testproj");
            assert.strictEqual(settings.runtime, "go");
            assert.strictEqual(settings.description, "A minimal Go Pulumi program");
        }
    });

    it(`stackSettings from yaml/yml/json`, async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create(
                withTestBackend({ workDir: upath.joinSafe(__dirname, "data", ext) }),
            );
            const settings = await ws.stackSettings("dev");
            assert.strictEqual(settings.secretsProvider, "abc");
            assert.strictEqual(settings.config!["plain"], "plain");
            assert.strictEqual(settings.config!["secure"].secure, "secret");
            await ws.saveStackSettings("dev", settings);
            assert.strictEqual(settings.secretsProvider, "abc");
        }
    });

    it(`fails gracefully for missing local workspace workDir`, async () => {
        try {
            const ws = await LocalWorkspace.create(withTestBackend({ workDir: "invalid-missing-workdir" }));
            assert.fail("expected create with invalid workDir to throw");
        } catch (err) {
            assert.strictEqual(
                err.toString(),
                "Error: Invalid workDir passed to local workspace: 'invalid-missing-workdir' does not exist",
            );
        }
    });

    it(`adds/removes/lists plugins successfully`, async () => {
        const ws = await LocalWorkspace.create(withTestBackend({}));
        await ws.installPlugin("aws", "v6.1.0");
        // See https://github.com/pulumi/pulumi/issues/11013 for why this is disabled
        //await ws.installPluginFromServer("scaleway", "v1.2.0", "github://api.github.com/lbrlabs");
        await ws.removePlugin("aws", "6.1.0");
        await ws.listPlugins();
    });

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
    it(`Config`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);

        const config = {
            plain: { value: "abc" },
            secret: { value: "def", secret: true },
        };
        let caught = 0;

        const plainKey = normalizeConfigKey("plain", projectName);
        const secretKey = normalizeConfigKey("secret", projectName);

        try {
            await stack.getConfig(plainKey);
        } catch (error) {
            caught++;
        }
        assert.strictEqual(caught, 1, "expected config get on empty value to throw");

        let values = await stack.getAllConfig();
        assert.strictEqual(Object.keys(values).length, 0, "expected stack config to be empty");
        await stack.setAllConfig(config);
        values = await stack.getAllConfig();
        assert.strictEqual(values[plainKey].value, "abc");
        assert.strictEqual(values[plainKey].secret, false);
        assert.strictEqual(values[secretKey].value, "def");
        assert.strictEqual(values[secretKey].secret, true);

        await stack.removeConfig("plain");
        values = await stack.getAllConfig();
        assert.strictEqual(Object.keys(values).length, 1, "expected stack config to have 1 value");
        await stack.setConfig("foo", { value: "bar" });
        values = await stack.getAllConfig();
        assert.strictEqual(Object.keys(values).length, 2, "expected stack config to have 2 values");

        await ws.removeStack(stackName);
    });
    it(`config_flag_like`, async () => {
        const projectName = "config_flag_like";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);
        await stack.setConfig("key", { value: "-value" });
        await stack.setConfig("secret-key", { value: "-value", secret: true });
        const values = await stack.getAllConfig();
        assert.strictEqual(values["config_flag_like:key"].value, "-value");
        assert.strictEqual(values["config_flag_like:key"].secret, false);
        assert.strictEqual(values["config_flag_like:secret-key"].value, "-value");
        assert.strictEqual(values["config_flag_like:secret-key"].secret, true);
        await stack.setAllConfig({
            key: { value: "-value2" },
            "secret-key": { value: "-value2", secret: true },
        });
        const values2 = await stack.getAllConfig();
        assert.strictEqual(values2["config_flag_like:key"].value, "-value2");
        assert.strictEqual(values2["config_flag_like:key"].secret, false);
        assert.strictEqual(values2["config_flag_like:secret-key"].value, "-value2");
        assert.strictEqual(values2["config_flag_like:secret-key"].secret, true);
    });
    it(`Config path`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);

        // test backward compatibility
        await stack.setConfig("key1", { value: "value1" });
        // test new flag without subPath
        await stack.setConfig("key2", { value: "value2" }, false);
        // test new flag with subPath
        await stack.setConfig("key3.subKey1", { value: "value3" }, true);
        // test secret
        await stack.setConfig("key4", { value: "value4", secret: true });
        // test subPath and key as secret
        await stack.setConfig("key5.subKey1", { value: "value5", secret: true }, true);
        // test string with dots
        await stack.setConfig("key6.subKey1", { value: "value6", secret: true });
        // test string with dots
        await stack.setConfig("key7.subKey1", { value: "value7", secret: true }, false);
        // test subPath
        await stack.setConfig("key7.subKey2", { value: "value8" }, true);
        // test subPath
        await stack.setConfig("key7.subKey3", { value: "value9" }, true);

        // test backward compatibility
        const cv1 = await stack.getConfig("key1");
        assert.strictEqual(cv1.value, "value1");
        assert.strictEqual(cv1.secret, false);

        // test new flag without subPath
        const cv2 = await stack.getConfig("key2", false);
        assert.strictEqual(cv2.value, "value2");
        assert.strictEqual(cv2.secret, false);

        // test new flag with subPath
        const cv3 = await stack.getConfig("key3.subKey1", true);
        assert.strictEqual(cv3.value, "value3");
        assert.strictEqual(cv3.secret, false);

        // test secret
        const cv4 = await stack.getConfig("key4");
        assert.strictEqual(cv4.value, "value4");
        assert.strictEqual(cv4.secret, true);

        // test subPath and key as secret
        const cv5 = await stack.getConfig("key5.subKey1", true);
        assert.strictEqual(cv5.value, "value5");
        assert.strictEqual(cv5.secret, true);

        // test string with dots
        const cv6 = await stack.getConfig("key6.subKey1");
        assert.strictEqual(cv6.value, "value6");
        assert.strictEqual(cv6.secret, true);

        // test string with dots
        const cv7 = await stack.getConfig("key7.subKey1", false);
        assert.strictEqual(cv7.value, "value7");
        assert.strictEqual(cv7.secret, true);

        // test string with dots
        const cv8 = await stack.getConfig("key7.subKey2", true);
        assert.strictEqual(cv8.value, "value8");
        assert.strictEqual(cv8.secret, false);

        // test string with dots
        const cv9 = await stack.getConfig("key7.subKey3", true);
        assert.strictEqual(cv9.value, "value9");
        assert.strictEqual(cv9.secret, false);

        await stack.removeConfig("key1");
        await stack.removeConfig("key2", false);
        await stack.removeConfig("key3", false);
        await stack.removeConfig("key4", false);
        await stack.removeConfig("key5", false);
        await stack.removeConfig("key6.subKey1", false);
        await stack.removeConfig("key7.subKey1", false);

        const cfg = await stack.getAllConfig();
        assert.strictEqual(cfg["node_test:key7"].value, '{"subKey2":"value8","subKey3":"value9"}');

        await ws.removeStack(stackName);
    });
    // This test requires the existence of a Pulumi.dev.yaml file because we are reading the nested
    // config from the file. This means we can't remove the stack at the end of the test.
    // We should also not include secrets in this config, because the secret encryption is only valid within
    // the context of a stack and org, and running this test in different orgs will fail if there are secrets.
    it(`nested_config`, async () => {
        const stackName = fullyQualifiedStackName(getTestOrg(), "nested_config", "dev");
        const workDir = upath.joinSafe(__dirname, "data", "nested_config");
        const stack = await LocalWorkspace.createOrSelectStack(
            { stackName, workDir },
            withTestBackend({}, "nested_config"),
        );

        const allConfig = await stack.getAllConfig();
        const outerVal = allConfig["nested_config:outer"];
        assert.strictEqual(outerVal.secret, false);
        assert.strictEqual(outerVal.value, '{"inner":"my_value","other":"something_else"}');

        const listVal = allConfig["nested_config:myList"];
        assert.strictEqual(listVal.secret, false);
        assert.strictEqual(listVal.value, '["one","two","three"]');

        const outer = await stack.getConfig("outer");
        assert.strictEqual(outer.secret, false);
        assert.strictEqual(outer.value, '{"inner":"my_value","other":"something_else"}');

        const list = await stack.getConfig("myList");
        assert.strictEqual(list.secret, false);
        assert.strictEqual(list.value, '["one","two","three"]');
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
    it(`returns valid whoami result`, async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const whoAmIResult = await ws.whoAmI();
        assert(whoAmIResult.user !== null);
        assert(whoAmIResult.url !== null);
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
    // TODO[pulumi/pulumi#8220] understand why this test was flaky
    xit(`runs through the stack lifecycle with a local program`, async () => {
        const stackName = fullyQualifiedStackName(getTestOrg(), "testproj", `int_test${getTestSuffix()}`);
        const workDir = upath.joinSafe(__dirname, "data", "testproj");
        const stack = await LocalWorkspace.createStack({ stackName, workDir }, withTestBackend({}));

        const config: ConfigMap = {
            bar: { value: "abc" },
            buzz: { value: "secret", secret: true },
        };
        await stack.setAllConfig(config);

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(Object.keys(upRes.outputs).length, 3);
        assert.strictEqual(upRes.outputs["exp_static"].value, "foo");
        assert.strictEqual(upRes.outputs["exp_static"].secret, false);
        assert.strictEqual(upRes.outputs["exp_cfg"].value, "abc");
        assert.strictEqual(upRes.outputs["exp_cfg"].secret, false);
        assert.strictEqual(upRes.outputs["exp_secret"].value, "secret");
        assert.strictEqual(upRes.outputs["exp_secret"].secret, true);
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi preview
        const preRes = await stack.preview({ userAgent });
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        const refRes = await stack.refresh({ userAgent });
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`previews a refresh without executing it`, async () => {
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            {
                stackName,
                projectName,
                program: async () => {
                    return;
                },
            },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi refresh
        const refRes = await stack.refresh({ userAgent, previewOnly: true });
        assert.strictEqual(refRes.summary.kind, "update");
        assert.strictEqual(refRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`previews a destroy without executing it`, async () => {
        const stackName = fullyQualifiedStackName(getTestOrg(), "testproj_dotnet", `int_test${getTestSuffix()}`);
        const workDir = upath.joinSafe(__dirname, "data", "testproj_dotnet");
        const stack = await LocalWorkspace.createStack(
            { stackName, workDir },
            withTestBackend({}, "testproj_dotnet", "", "dotnet"),
        );

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent, previewOnly: true });
        assert.strictEqual(destroyRes.summary.kind, "update");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`runs through the stack lifecycle with a local dotnet program`, async () => {
        const stackName = fullyQualifiedStackName(getTestOrg(), "testproj_dotnet", `int_test${getTestSuffix()}`);
        const workDir = upath.joinSafe(__dirname, "data", "testproj_dotnet");
        const stack = await LocalWorkspace.createStack(
            { stackName, workDir },
            withTestBackend({}, "testproj_dotnet", "", "dotnet"),
        );

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(Object.keys(upRes.outputs).length, 1);
        assert.strictEqual(upRes.outputs["exp_static"].value, "foo");
        assert.strictEqual(upRes.outputs["exp_static"].secret, false);
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi preview
        const preRes = await stack.preview({ userAgent });
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        const refRes = await stack.refresh({ userAgent });
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`runs through the stack lifecycle with an inline program`, async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        const stackConfig: ConfigMap = {
            bar: { value: "abc" },
            buzz: { value: "secret", secret: true },
        };
        await stack.setAllConfig(stackConfig);

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(Object.keys(upRes.outputs).length, 3);
        assert.strictEqual(upRes.outputs["exp_static"].value, "foo");
        assert.strictEqual(upRes.outputs["exp_static"].secret, false);
        assert.strictEqual(upRes.outputs["exp_cfg"].value, "abc");
        assert.strictEqual(upRes.outputs["exp_cfg"].secret, false);
        assert.strictEqual(upRes.outputs["exp_secret"].value, "secret");
        assert.strictEqual(upRes.outputs["exp_secret"].secret, true);
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi preview
        const preRes = await stack.preview({ userAgent });
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        const refRes = await stack.refresh({ userAgent });
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`listens for error output`, async () => {
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            {
                stackName,
                projectName,
                program: async () => {
                    return;
                },
            },
            withTestBackend({}, "inline_node"),
        );

        // We need to come up with some creative ways to make these commands
        // produce error output. These are the least elaborate ideas I could
        // come up with.

        let error = "";

        // pulumi up
        try {
            await stack.up({
                plan: "halloumi",
                onError: (e) => {
                    error += e;
                },
            });
        } catch (e) {
            assert.notStrictEqual(error, "");
        }

        assert.match(error, /error: open halloumi/);
        error = "";

        const upRes = await stack.up();
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi preview
        try {
            await stack.preview({
                parallel: 1.1,
                onError: (e) => {
                    error += e;
                },
            });
        } catch (e) {
            assert.notStrictEqual(error, "");
        }

        assert.match(error, /error: invalid argument/);
        error = "";

        const preRes = await stack.preview({ userAgent });
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        try {
            await stack.refresh({
                parallel: 2.2,
                onError: (e) => {
                    error += e;
                },
            });
        } catch (e) {
            assert.notStrictEqual(error, "");
        }

        assert.match(error, /error: invalid argument/);
        error = "";

        const refRes = await stack.refresh({ userAgent });
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        try {
            await stack.destroy({
                parallel: 3.3,
                onError: (e) => {
                    error += e;
                },
            });
        } catch (e) {
            assert.notStrictEqual(error, "");
        }

        assert.match(error, /error: invalid argument/);
        error = "";

        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`runs through the stack lifecycle with an inline program, testing removing without destroying`, async () => {
        const program = async () => {
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            new MyResource("res");
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        await stack.up({ userAgent });

        // we shouldn't be able to remove the stack without force
        // since the stack has an active resource
        assert.rejects(stack.workspace.removeStack(stackName));

        await stack.workspace.removeStack(stackName, { force: true });

        // we shouldn't be able to select the stack after it's been removed
        // we expect this error
        assert.rejects(stack.workspace.selectStack(stackName));
    });
    it("runs through the stack lifecycle with an inline program, testing destroy with --remove", async () => {
        // Arrange.
        const program = async () => {
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            new MyResource("res");
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        await stack.up({ userAgent });

        // Act.
        await stack.destroy({ userAgent, remove: true });

        // Assert.
        await assert.rejects(stack.workspace.selectStack(stackName));
    });
    // Regression test for https://github.com/pulumi/pulumi/issues/17613
    it(`does not hang on a failed input`, async function () {
        this.timeout(20 * 1000); // This test hangs indefinitely if it fails

        const program = async () => {
            class MyResource extends CustomResource {
                constructor(name: string, props: any) {
                    super("test:index:MyResource", name, props);
                }
            }

            const failedInput = output(Promise.reject("input rejected"));

            new MyResource("testResource1", {
                failingInput: failedInput,
            });
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `auto_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        await assert.rejects(stack.up(), /input rejected/);

        await stack.destroy();
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
    it(`refreshes with refresh option`, async () => {
        // We create a simple program, and scan the output for an indication
        // that adding refresh: true will perfrom a refresh operation.
        const program = async () => {
            return {
                toggle: true,
            };
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );
        // • First, run Up so we can set the initial state.
        await stack.up({ userAgent });
        // • Next, run preview with refresh and check that the refresh was performed.
        const refresh = true;
        const previewRes = await stack.preview({ userAgent, refresh });
        assert.match(previewRes.stdout, /refreshing/);
        assert.strictEqual(previewRes.changeSummary.same, 1, "preview expected 1 same (the stack)");

        const upRes = await stack.up({ userAgent, refresh });
        assert.match(upRes.stdout, /refreshing/);

        const destroyRes = await stack.destroy({ userAgent, refresh });
        assert.match(destroyRes.stdout, /refreshing/);
    });
    it(`operations accept configFile option`, async () => {
        // We are testing that the configFile option is accepted by the operations.

        const configPath = upath.joinSafe(__dirname, "data", "yaml", "Pulumi.local.yaml");

        const program = async () => {
            const config = new Config();
            return {
                plain: config.get("plain"),
            };
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `configfile_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );
        await stack.up({ userAgent, configFile: configPath });
        const outputs = await stack.outputs();
        assert.strictEqual(outputs["plain"].value, "plain");
        const previewRes = await stack.preview({ userAgent, configFile: configPath });
        assert.strictEqual(previewRes.changeSummary.same, 1);
        const refRes = await stack.refresh({ userAgent, configFile: configPath });
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");
        const destroyRes = await stack.destroy({ userAgent, configFile: configPath });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");
        await stack.workspace.removeStack(stackName);
    });
    it(`destroys an inline program with excludeProtected`, async () => {
        const program = async () => {
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            const config = new Config();
            const protect = config.getBoolean("protect") ?? false;
            new MyResource("first", { protect });
            new MyResource("second");
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        // initial up
        await stack.setConfig("protect", { value: "true" });
        await stack.up({ userAgent });

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent, excludeProtected: true });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");
        assert.match(destroyRes.stdout, /All unprotected resources were destroyed/);

        // unprotected resources
        await stack.removeConfig("protect");
        await stack.up({ userAgent });

        // pulumi destroy to cleanup all resources
        await stack.destroy({ userAgent });

        await stack.workspace.removeStack(stackName);
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
    it(`runs through the stack lifecycle with multiple inline programs in parallel`, async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const projectName = "inline_node";
        const stackNames = Array.from(Array(30).keys()).map((_) =>
            fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`),
        );

        const testStackLifetime = async (stackName: string) => {
            const stack = await LocalWorkspace.createStack(
                { stackName, projectName, program },
                withTestBackend({}, "inline_node"),
            );

            const stackConfig: ConfigMap = {
                bar: { value: "abc" },
                buzz: { value: "secret", secret: true },
            };
            await stack.setAllConfig(stackConfig);

            // pulumi up
            const upRes = await stack.up({ userAgent }); // pulumi up
            assert.strictEqual(Object.keys(upRes.outputs).length, 3);
            assert.strictEqual(upRes.outputs["exp_static"].value, "foo");
            assert.strictEqual(upRes.outputs["exp_static"].secret, false);
            assert.strictEqual(upRes.outputs["exp_cfg"].value, "abc");
            assert.strictEqual(upRes.outputs["exp_cfg"].secret, false);
            assert.strictEqual(upRes.outputs["exp_secret"].value, "secret");
            assert.strictEqual(upRes.outputs["exp_secret"].secret, true);
            assert.strictEqual(upRes.summary.kind, "update");
            assert.strictEqual(upRes.summary.result, "succeeded");

            // pulumi preview
            const preRes = await stack.preview({ userAgent }); // pulumi preview
            assert.strictEqual(preRes.changeSummary.same, 1);

            // pulumi refresh
            const refRes = await stack.refresh({ userAgent });
            assert.strictEqual(refRes.summary.kind, "refresh");
            assert.strictEqual(refRes.summary.result, "succeeded");

            // pulumi destroy
            const destroyRes = await stack.destroy({ userAgent });
            assert.strictEqual(destroyRes.summary.kind, "destroy");
            assert.strictEqual(destroyRes.summary.result, "succeeded");

            await stack.workspace.removeStack(stack.name);
        };
        for (let i = 0; i < stackNames.length; i += 10) {
            const chunk = stackNames.slice(i, i + 10);
            await Promise.all(chunk.map(async (stackName) => await testStackLifetime(stackName)));
        }
    });
    it(`handles events`, async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        const stackConfig: ConfigMap = {
            bar: { value: "abc" },
            buzz: { value: "secret", secret: true },
        };
        await stack.setAllConfig(stackConfig);

        let seenSummaryEvent = false;
        const findSummaryEvent = (event: EngineEvent) => {
            if (event.summaryEvent) {
                seenSummaryEvent = true;
            }
        };

        // pulumi preview
        const preRes = await stack.preview({ onEvent: findSummaryEvent });
        assert.strictEqual(seenSummaryEvent, true, "No SummaryEvent for `preview`");
        assert.strictEqual(preRes.changeSummary.create, 1);

        // pulumi up
        seenSummaryEvent = false;
        const upRes = await stack.up({ onEvent: findSummaryEvent });
        assert.strictEqual(seenSummaryEvent, true, "No SummaryEvent for `up`");
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi preview
        seenSummaryEvent = false;
        const preResAgain = await stack.preview({ onEvent: findSummaryEvent });
        assert.strictEqual(seenSummaryEvent, true, "No SummaryEvent for `preview`");
        assert.strictEqual(preResAgain.changeSummary.same, 1);

        // pulumi refresh
        seenSummaryEvent = false;
        const refRes = await stack.refresh({ onEvent: findSummaryEvent });
        assert.strictEqual(seenSummaryEvent, true, "No SummaryEvent for `refresh`");
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        seenSummaryEvent = false;
        const destroyRes = await stack.destroy({ onEvent: findSummaryEvent });
        assert.strictEqual(seenSummaryEvent, true, "No SummaryEvent for `destroy`");
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    // TODO[pulumi/pulumi#7127]: Re-enabled the warning.
    // Temporarily skipping test until we've re-enabled the warning.
    it.skip(`has secret config warnings`, async () => {
        const program = async () => {
            const config = new Config();

            config.get("plainstr1");
            config.require("plainstr2");
            config.getSecret("plainstr3");
            config.requireSecret("plainstr4");

            config.getBoolean("plainbool1");
            config.requireBoolean("plainbool2");
            config.getSecretBoolean("plainbool3");
            config.requireSecretBoolean("plainbool4");

            config.getNumber("plainnum1");
            config.requireNumber("plainnum2");
            config.getSecretNumber("plainnum3");
            config.requireSecretNumber("plainnum4");

            config.getObject("plainobj1");
            config.requireObject("plainobj2");
            config.getSecretObject("plainobj3");
            config.requireSecretObject("plainobj4");

            config.get("str1");
            config.require("str2");
            config.getSecret("str3");
            config.requireSecret("str4");

            config.getBoolean("bool1");
            config.requireBoolean("bool2");
            config.getSecretBoolean("bool3");
            config.requireSecretBoolean("bool4");

            config.getNumber("num1");
            config.requireNumber("num2");
            config.getSecretNumber("num3");
            config.requireSecretNumber("num4");

            config.getObject("obj1");
            config.requireObject("obj2");
            config.getSecretObject("obj3");
            config.requireSecretObject("obj4");
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        const stackConfig: ConfigMap = {
            plainstr1: { value: "1" },
            plainstr2: { value: "2" },
            plainstr3: { value: "3" },
            plainstr4: { value: "4" },
            plainbool1: { value: "true" },
            plainbool2: { value: "true" },
            plainbool3: { value: "true" },
            plainbool4: { value: "true" },
            plainnum1: { value: "1" },
            plainnum2: { value: "2" },
            plainnum3: { value: "3" },
            plainnum4: { value: "4" },
            plainobj1: { value: "{}" },
            plainobj2: { value: "{}" },
            plainobj3: { value: "{}" },
            plainobj4: { value: "{}" },
            str1: { value: "1", secret: true },
            str2: { value: "2", secret: true },
            str3: { value: "3", secret: true },
            str4: { value: "4", secret: true },
            bool1: { value: "true", secret: true },
            bool2: { value: "true", secret: true },
            bool3: { value: "true", secret: true },
            bool4: { value: "true", secret: true },
            num1: { value: "1", secret: true },
            num2: { value: "2", secret: true },
            num3: { value: "3", secret: true },
            num4: { value: "4", secret: true },
            obj1: { value: "{}", secret: true },
            obj2: { value: "{}", secret: true },
            obj3: { value: "{}", secret: true },
            obj4: { value: "{}", secret: true },
        };
        await stack.setAllConfig(stackConfig);

        let events: string[] = [];
        const findDiagnosticEvents = (event: EngineEvent) => {
            if (event.diagnosticEvent?.severity === "warning") {
                events.push(event.diagnosticEvent.message);
            }
        };

        const expectedWarnings = [
            "Configuration 'inline_node:str1' value is a secret; use `getSecret` instead of `get`",
            "Configuration 'inline_node:str2' value is a secret; use `requireSecret` instead of `require`",
            "Configuration 'inline_node:bool1' value is a secret; use `getSecretBoolean` instead of `getBoolean`",
            "Configuration 'inline_node:bool2' value is a secret; use `requireSecretBoolean` instead of `requireBoolean`",
            "Configuration 'inline_node:num1' value is a secret; use `getSecretNumber` instead of `getNumber`",
            "Configuration 'inline_node:num2' value is a secret; use `requireSecretNumber` instead of `requireNumber`",
            "Configuration 'inline_node:obj1' value is a secret; use `getSecretObject` instead of `getObject`",
            "Configuration 'inline_node:obj2' value is a secret; use `requireSecretObject` instead of `requireObject`",
        ];

        // These keys should not be in any warning messages.
        const unexpectedWarnings = [
            "plainstr1",
            "plainstr2",
            "plainstr3",
            "plainstr4",
            "plainbool1",
            "plainbool2",
            "plainbool3",
            "plainbool4",
            "plainnum1",
            "plainnum2",
            "plainnum3",
            "plainnum4",
            "plainobj1",
            "plainobj2",
            "plainobj3",
            "plainobj4",
            "str3",
            "str4",
            "bool3",
            "bool4",
            "num3",
            "num4",
            "obj3",
            "obj4",
        ];

        const validate = (warnings: string[]) => {
            for (const expected of expectedWarnings) {
                let found = false;
                for (const warning of warnings) {
                    if (warning.includes(expected)) {
                        found = true;
                        break;
                    }
                }
                assert.strictEqual(found, true, `expected warning not found`);
            }
            for (const unexpected of unexpectedWarnings) {
                for (const warning of warnings) {
                    assert.strictEqual(
                        warning.includes(unexpected),
                        false,
                        `Unexpected '${unexpected}' found in warning`,
                    );
                }
            }
        };

        // pulumi preview
        await stack.preview({ onEvent: findDiagnosticEvents });
        validate(events);

        // pulumi up
        events = [];
        await stack.up({ onEvent: findDiagnosticEvents });
        validate(events);

        await stack.workspace.removeStack(stackName);
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
    // TODO[pulumi/pulumi#8061] flaky test
    xit(`supports stack outputs`, async () => {
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

        const assertOutputs = (outputs: OutputMap) => {
            assert.strictEqual(Object.keys(outputs).length, 3, "expected to have 3 outputs");
            assert.strictEqual(outputs["exp_static"].value, "foo");
            assert.strictEqual(outputs["exp_static"].secret, false);
            assert.strictEqual(outputs["exp_cfg"].value, "abc");
            assert.strictEqual(outputs["exp_cfg"].secret, false);
            assert.strictEqual(outputs["exp_secret"].value, "secret");
            assert.strictEqual(outputs["exp_secret"].secret, true);
        };

        try {
            await stack.setAllConfig({
                bar: { value: "abc" },
                buzz: { value: "secret", secret: true },
            });

            const initialOutputs = await stack.outputs();
            assert.strictEqual(Object.keys(initialOutputs).length, 0, "expected initialOutputs to be empty");

            // pulumi up
            const upRes = await stack.up();
            assert.strictEqual(upRes.summary.kind, "update");
            assert.strictEqual(upRes.summary.result, "succeeded");
            assertOutputs(upRes.outputs);

            const outputsAfterUp = await stack.outputs();
            assertOutputs(outputsAfterUp);

            const destroyRes = await stack.destroy();
            assert.strictEqual(destroyRes.summary.kind, "destroy");
            assert.strictEqual(destroyRes.summary.result, "succeeded");

            const outputsAfterDestroy = await stack.outputs();
            assert.strictEqual(Object.keys(outputsAfterDestroy).length, 0, "expected outputsAfterDestroy to be empty");
        } finally {
            await stack.workspace.removeStack(stackName);
        }
    });
    it(`runs an inline program that exits gracefully`, async () => {
        const program = async () => ({});
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up
        await assert.doesNotReject(stack.up());

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`runs an inline program that rejects a promise and exits gracefully`, async () => {
        const program = async () => {
            Promise.reject(new Error());
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up
        await assert.rejects(stack.up());

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`runs successfully after a previous failure`, async () => {
        let shouldFail = true;
        const program = async () => {
            if (shouldFail) {
                Promise.reject(new Error());
            }
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up rejects the first time
        await assert.rejects(stack.up());

        // pulumi up succeeds the 2nd time
        shouldFail = false;
        await assert.doesNotReject(stack.up());

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });

    it("can import resources into a stack using resource definitions", async () => {
        const workDir = upath.joinSafe(__dirname, "data", "import");
        const stackName = fullyQualifiedStackName(getTestOrg(), "node_test", `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack({ workDir, stackName }, withTestBackend({}));
        const pulumiRandomVersion = "4.16.3";
        await stack.workspace.installPlugin("random", pulumiRandomVersion);
        const result = await stack.import({
            protect: false,
            resources: [
                {
                    type: "random:index/randomPassword:RandomPassword",
                    name: "randomPassword",
                    id: "supersecret",
                },
            ],
        });
        let kind = "resource-import";
        if (process.env.PULUMI_ACCESS_TOKEN) {
            // The service handles this slightly differently, and we just get "update" as "kind"
            kind = "update";
        }
        assert.strictEqual(result.summary.kind, kind);
        assert.strictEqual(result.summary.result, "succeeded");

        const expectedGeneratedCode = fs.readFileSync(upath.joinSafe(workDir, "expected_generated_code.txt"), "utf8");
        assert.strictEqual(result.generatedCode, expectedGeneratedCode);
        await stack.destroy();
        await stack.workspace.removeStack(stackName);
        await stack.workspace.removePlugin("random", pulumiRandomVersion);
    });

    it("can import resources into a stack without generating code", async () => {
        const workDir = upath.joinSafe(__dirname, "data", "import");
        const stackName = fullyQualifiedStackName(getTestOrg(), "node_test", `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack({ workDir, stackName }, withTestBackend({}));
        const pulumiRandomVersion = "4.16.3";
        await stack.workspace.installPlugin("random", pulumiRandomVersion);
        const result = await stack.import({
            protect: false,
            generateCode: false,
            resources: [
                {
                    type: "random:index/randomPassword:RandomPassword",
                    name: "randomPassword",
                    id: "supersecret",
                },
            ],
        });
        let kind = "resource-import";
        if (process.env.PULUMI_ACCESS_TOKEN) {
            // The service handles this slightly differently, and we just get "update" as "kind"
            kind = "update";
        }
        assert.strictEqual(result.summary.kind, kind);
        assert.strictEqual(result.summary.result, "succeeded");
        assert.strictEqual(result.generatedCode, "");
        await stack.destroy();
        await stack.workspace.removeStack(stackName);
        await stack.workspace.removePlugin("random", pulumiRandomVersion);
    });

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
    it("fails creation if remote operation is not supported", async () => {
        const mockWithNoRemoteSupport = {
            command: "pulumi",
            version: new semver.SemVer("2.0.0"),
            // We inspect the output of `pulumi preview --help` to determine
            // if the CLI supports remote operations, see
            // `LocalWorkspace.checkRemoteSupport`.
            run: async () => new CommandResult("some output", "", 0),
        };
        await assert.rejects(
            LocalWorkspace.create(withTestBackend({ pulumiCommand: mockWithNoRemoteSupport, remote: true })),
        );
    });
    it("bypasses remote support check", async () => {
        const mockWithNoRemoteSupport = {
            command: "pulumi",
            version: new semver.SemVer("2.0.0"),
            // We inspect the output of `pulumi preview --help` to determine
            // if the CLI supports remote operations, see
            // `LocalWorkspace.checkRemoteSupport`.
            run: async () => new CommandResult("some output", "", 0),
        };
        await assert.doesNotReject(
            LocalWorkspace.create(
                withTestBackend({
                    pulumiCommand: mockWithNoRemoteSupport,
                    remote: true,
                    envVars: {
                        PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK: "true",
                    },
                }),
            ),
        );
    });
    it(`respects existing project settings`, async () => {
        const projectName = "correct_project";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            {
                stackName,
                projectName,
                program: async () => {
                    return;
                },
            },
            withTestBackend(
                { workDir: upath.joinSafe(__dirname, "data", "correct_project") },
                "correct_project",
                "This is a description",
            ),
        );
        const projectSettings = await stack.workspace.projectSettings();
        assert.strictEqual(projectSettings.name, "correct_project");
        // the description check is enough to verify that the stack wasn't overwritten
        assert.strictEqual(projectSettings.description, "This is a description");
        await stack.workspace.removeStack(stackName);
    });
    it(`correctly sets config on multiple stacks concurrently`, async () => {
        const dones = [];
        const stacks = ["dev", "dev2", "dev3", "dev4", "dev5"].map((x) =>
            fullyQualifiedStackName("organization", "concurrent-config", `int_test_${x}_${getTestSuffix()}`),
        );
        const workDir = upath.joinSafe(__dirname, "data", "tcfg");
        const ws = await LocalWorkspace.create({
            workDir,
            projectSettings: {
                name: "concurrent-config",
                runtime: "nodejs",
                backend: { url: "file://~" },
            },
            envVars: {
                PULUMI_CONFIG_PASSPHRASE: "test",
            },
        });
        for (let i = 0; i < stacks.length; i++) {
            await Stack.create(stacks[i], ws);
        }
        for (let i = 0; i < stacks.length; i++) {
            const x = i;
            const s = stacks[i];
            dones.push(
                (async () => {
                    for (let j = 0; j < 20; j++) {
                        await ws.setConfig(s, "var-" + j, { value: (x * 20 + j).toString() });
                    }
                })(),
            );
        }
        await Promise.all(dones);

        for (let i = 0; i < stacks.length; i++) {
            const stack = await LocalWorkspace.selectStack({
                stackName: stacks[i],
                workDir,
            });
            const config = await stack.getAllConfig();
            assert.strictEqual(Object.keys(config).length, 20);
            await stack.workspace.removeStack(stacks[i]);
        }
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

        assert.rejects(() => ws.install());
    });

    it(`install --use-language-version-tools requires version >= 3.130`, async () => {
        const mockCommand = {
            command: "pulumi",
            version: semver.parse("3.129.0"),
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

        assert.rejects(() => ws.install());
    });

    it("sends SIGINT when aborted", async () => {
        const controller = new AbortController();
        const program = async () => {
            await new Promise((f) => setTimeout(f, 60000));
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        new Promise((f) => setTimeout(f, 500)).then(() => controller.abort());
        try {
            // pulumi preview
            const previewRes = await stack.preview({
                signal: controller.signal,
            });
            assert.fail("expected canceled preview to throw");
        } catch (err) {
            assert.match(err.toString(), /stderr: Command was killed with SIGINT|error: preview canceled/);
            assert.match(err.toString(), /CommandError: code: -2/);
        }

        await stack.workspace.removeStack(stackName);
    });
});

const normalizeConfigKey = (key: string, projectName: string) => {
    const parts = key.split(":");
    if (parts.length < 2) {
        return `${projectName}:${key}`;
    }
    return "";
};

/**
 * Augments the provided {@link LocalWorkspaceOptions} so that they reference a
 * either a file backend or a cloud backend, depending on whether PULUMI_ACCESS_TOKEN
 * is set in the environment.
 */
function withTestBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    if (process.env.PULUMI_ACCESS_TOKEN) {
        return withCloudBackend(opts, name, description, runtime);
    }
    return withTemporaryFileBackend(opts, name, description, runtime);
}

function withCloudBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    const backend = {
        url: "https://api.pulumi.com",
    };
    if (name === undefined) {
        name = "node_test";
    }
    if (runtime === undefined) {
        runtime = "nodejs";
    }
    return {
        ...opts,
        projectSettings: {
            // We are obliged to provide a name and runtime if we provide project
            // settings, so we do so, but we spread in the provided project settings
            // afterwards so that the caller can override them if need be.
            name: name,
            runtime: runtime as ProjectRuntime,
            description: description,

            ...opts?.projectSettings,
            backend,
        },
    };
}

function withTemporaryFileBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    const tmpDir = tmp.dirSync({
        prefix: "nodejs-tests-automation-",
        unsafeCleanup: true,
    });

    const backend = { url: `file://${tmpDir.name}` };

    if (name === undefined) {
        name = "node_test";
    }
    if (runtime === undefined) {
        runtime = "nodejs";
    }

    return withTestConfigPassphrase({
        ...opts,
        pulumiHome: tmpDir.name,
        projectSettings: {
            // We are obliged to provide a name and runtime if we provide project
            // settings, so we do so, but we spread in the provided project settings
            // afterwards so that the caller can override them if need be.
            name: name,
            runtime: runtime as ProjectRuntime,
            description: description,

            ...opts?.projectSettings,
            backend,
        },
    });
}

/**
 * Augments the provided {@link LocalWorkspaceOptions} so that they set up an
 * environment containing a test `PULUMI_CONFIG_PASSPHRASE` variable suitable
 * for use with a local file backend.
 */
function withTestConfigPassphrase(opts?: LocalWorkspaceOptions): LocalWorkspaceOptions {
    return {
        ...opts,
        envVars: {
            ...opts?.envVars,
            PULUMI_CONFIG_PASSPHRASE: "test",
        },
    };
}
