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
import * as upath from "upath";
import * as fs from "fs";

import {
    CommandResult,
    ConfigMap,
    EngineEvent,
    fullyQualifiedStackName,
    LocalWorkspace,
    OutputMap,
    ProjectSettings,
} from "../../automation";
import { CustomResource, ComponentResource, ComponentResourceOptions, Config, output, ResourceHook } from "../../index";
import { getTestOrg, getTestSuffix, withTestBackend } from "./util";

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
        const refRes = await stack.previewRefresh({ userAgent });
        assert.deepStrictEqual(refRes.changeSummary, { same: 1 });

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`previews a refresh with resources without executing it`, async () => {
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            {
                stackName,
                projectName,
                program: async () => {
                    class MyResource extends ComponentResource {
                        constructor(name: string, opts?: ComponentResourceOptions) {
                            super("my:module:MyResource", name, {}, opts);
                        }
                    }
                    new MyResource("res");
                    return {};
                },
            },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi refresh
        const refRes = await stack.previewRefresh({ userAgent });
        assert.deepStrictEqual(refRes.changeSummary, { same: 2 });

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

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

        // pulumi destroy --preview-only
        const previewDestroyRes = await stack.previewDestroy({ userAgent });
        assert.deepStrictEqual(previewDestroyRes.changeSummary, { delete: 1 });

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.deepStrictEqual(destroyRes.summary.resourceChanges, { delete: 1 });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    });
    it(`previews a destroy with inline program`, async () => {
        const program = async () => {
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            new MyResource("res");
            return {};
        };

        const stackName = fullyQualifiedStackName(getTestOrg(), "inline_node", `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName: "inline_node", program },
            withTestBackend({}, "inline_node"),
        );

        // pulumi up
        const upRes = await stack.up({ userAgent });
        assert.strictEqual(upRes.summary.kind, "update");
        assert.strictEqual(upRes.summary.result, "succeeded");

        // pulumi destroy --preview-only
        const previewDestroyRes = await stack.previewDestroy({ userAgent });
        assert.deepStrictEqual(previewDestroyRes.changeSummary, { delete: 2 });

        // pulumi destroy
        const destroyRes = await stack.destroy({ userAgent });
        assert.deepStrictEqual(destroyRes.summary.resourceChanges, { delete: 2 });
        assert.strictEqual(destroyRes.summary.kind, "destroy");
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
        await assert.rejects(stack.workspace.removeStack(stackName));

        await stack.workspace.removeStack(stackName, { force: true });

        // we shouldn't be able to select the stack after it's been removed
        // we expect this error
        await assert.rejects(stack.workspace.selectStack(stackName));
    });
    it(`runs through the stack lifecycle with an inline program, testing removing with removeBackups`, async () => {
        const program = async () => {
            return {};
        };
        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        await stack.up({ userAgent });

        await stack.workspace.removeStack(stackName, { removeBackups: true });

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
    it("sends SIGINT when aborted", async () => {
        const controller = new AbortController();
        let timeout;
        const program = async () => {
            await new Promise((f) => {
                timeout = setTimeout(f, 60000);
            });
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
        clearTimeout(timeout);

        await stack.workspace.removeStack(stackName);
    });

    it("hooks", async function () {
        let beforeCreateCalled = false;
        let beforeDeleteCalled = false;

        const program = async () => {
            const beforeDelete = new ResourceHook("beforeDelete", async (args) => {
                beforeDeleteCalled = true;
            });
            const beforeCreate = new ResourceHook("beforeCreate", async (args) => {
                beforeCreateCalled = true;
            });
            class MyResource extends ComponentResource {
                constructor(name: string, opts?: ComponentResourceOptions) {
                    super("my:module:MyResource", name, {}, opts);
                }
            }
            new MyResource("res", {
                hooks: {
                    beforeCreate: [beforeCreate],
                    beforeDelete: [beforeDelete],
                },
            });
        };

        const projectName = "inline_node";
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await LocalWorkspace.createStack(
            { stackName, projectName, program },
            withTestBackend({}, "inline_node"),
        );

        await stack.up();
        assert.strictEqual(true, beforeCreateCalled);
        let state = await stack.exportStack();
        assert.strictEqual(state.deployment.resources.length, 2);
        const res = state.deployment.resources[1];
        assert.strictEqual(res.type, "my:module:MyResource");

        await stack.destroy({ runProgram: true });
        assert.strictEqual(true, beforeDeleteCalled);
        state = await stack.exportStack();
        assert.strictEqual(state.deployment.resources, undefined);
    });
});
