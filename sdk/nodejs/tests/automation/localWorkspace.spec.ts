// Copyright 2016-2020, Pulumi Corporation.
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
import * as semver from "semver";
import * as upath from "upath";

import { Config } from "../../index";
import {
    ConfigMap,
    EngineEvent,
    fullyQualifiedStackName,
    LocalWorkspace,
    ProjectSettings,
    Stack,
    validatePulumiVersion,
} from "../../automation";
import { asyncTest } from "../util";

const versionRegex = /(\d+\.)(\d+\.)(\d+)(-.*)?/;

describe("LocalWorkspace", () => {
    it(`projectSettings from yaml/yml/json`, asyncTest(async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create({ workDir: upath.joinSafe(__dirname, "data", ext) });
            const settings = await ws.projectSettings();
            assert(settings.name, "testproj");
            assert(settings.runtime, "go");
            assert(settings.description, "A minimal Go Pulumi program");

        }
    }));

    it(`stackSettings from yaml/yml/json`, asyncTest(async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create({ workDir: upath.joinSafe(__dirname, "data", ext) });
            const settings = await ws.stackSettings("dev");
            assert.strictEqual(settings.secretsProvider, "abc");
            assert.strictEqual(settings.config!["plain"], "plain");
            assert.strictEqual(settings.config!["secure"].secure, "secret");
        }
    }));

    it(`adds/removes/lists plugins successfully`, asyncTest(async () => {
        const ws = await LocalWorkspace.create({});
        await ws.installPlugin("aws", "v3.0.0");
        await ws.removePlugin("aws", "3.0.0");
        await ws.listPlugins();
    }));

    it(`create/select/remove LocalWorkspace stack`, asyncTest(async () => {
        const projectSettings: ProjectSettings = {
            name: "node_test",
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        await ws.createStack(stackName);
        await ws.selectStack(stackName);
        await ws.removeStack(stackName);
    }));

    it(`create/select/createOrSelect Stack`, asyncTest(async () => {
        const projectSettings: ProjectSettings = {
            name: "node_test",
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        await Stack.create(stackName, ws);
        await Stack.select(stackName, ws);
        await Stack.createOrSelect(stackName, ws);
        await ws.removeStack(stackName);
    }));
    it(`Config`, asyncTest(async () => {
        const projectName = "node_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
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
    }));
    it(`nested_config`, asyncTest(async () => {
        const stackName = fullyQualifiedStackName("pulumi-test", "nested_config", "dev");
        const workDir = upath.joinSafe(__dirname, "data", "nested_config");
        const stack = await LocalWorkspace.createOrSelectStack({ stackName, workDir });

        const allConfig = await stack.getAllConfig();
        const outerVal = allConfig["nested_config:outer"];
        assert.strictEqual(outerVal.secret, true);
        assert.strictEqual(outerVal.value, "{\"inner\":\"my_secret\",\"other\":\"something_else\"}");

        const listVal = allConfig["nested_config:myList"];
        assert.strictEqual(listVal.secret, false);
        assert.strictEqual(listVal.value, "[\"one\",\"two\",\"three\"]");

        const outer = await stack.getConfig("outer");
        assert.strictEqual(outer.secret, true);
        assert.strictEqual(outer.value, "{\"inner\":\"my_secret\",\"other\":\"something_else\"}");

        const list = await stack.getConfig("myList");
        assert.strictEqual(list.secret, false);
        assert.strictEqual(list.value, "[\"one\",\"two\",\"three\"]");
    }));
    it(`can list stacks and currently selected stack`, asyncTest(async () => {
        const projectSettings: ProjectSettings = {
            name: `node_list_test${getTestSuffix()}`,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackNamer = () => `int_test${getTestSuffix()}`;
        const stackNames: string[] = [];
        for (let i = 0; i < 2; i++) {
            const stackName = stackNamer();
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
    }));
    it(`stack status methods`, asyncTest(async () => {
        const projectSettings: ProjectSettings = {
            name: "node_test",
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        const stack = await Stack.create(stackName, ws);
        const history = await stack.history();
        assert.strictEqual(history.length, 0);
        const info = await stack.info();
        assert.strictEqual(typeof (info), "undefined");
        await ws.removeStack(stackName);
    }));
    it(`runs through the stack lifecycle with a local program`, asyncTest(async () => {
        const stackName = `int_test${getTestSuffix()}`;
        const workDir = upath.joinSafe(__dirname, "data", "testproj");
        const stack = await LocalWorkspace.createStack({ stackName, workDir });

        const config: ConfigMap = {
            "bar": { value: "abc" },
            "buzz": { value: "secret", secret: true },
        };
        await stack.setAllConfig(config);

        // pulumi up
        const upRes = await stack.up();
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
        const preRes = await stack.preview();
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        const refRes = await stack.refresh();
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    }));
    it(`runs through the stack lifecycle with an inline program`, asyncTest(async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const stackName = `int_test${getTestSuffix()}`;
        const projectName = "inline_node";
        const stack = await LocalWorkspace.createStack({ stackName, projectName, program });

        const stackConfig: ConfigMap = {
            "bar": { value: "abc" },
            "buzz": { value: "secret", secret: true },
        };
        await stack.setAllConfig(stackConfig);

        // pulumi up
        const upRes = await stack.up();
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
        const preRes = await stack.preview();
        assert.strictEqual(preRes.changeSummary.same, 1);

        // pulumi refresh
        const refRes = await stack.refresh();
        assert.strictEqual(refRes.summary.kind, "refresh");
        assert.strictEqual(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    }));
    it(`handles events`, asyncTest(async () => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const stackName = `int_test${getTestSuffix()}`;
        const projectName = "inline_node";
        const stack = await LocalWorkspace.createStack({ stackName, projectName, program });

        const stackConfig: ConfigMap = {
            "bar": { value: "abc" },
            "buzz": { value: "secret", secret: true },
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
    }));
    it(`imports and exports stacks`, asyncTest(async() => {
        const program = async () => {
            const config = new Config();
            return {
                exp_static: "foo",
                exp_cfg: config.get("bar"),
                exp_secret: config.getSecret("buzz"),
            };
        };
        const stackName = `int_test${getTestSuffix()}`;
        const projectName = "import_export_node";
        const stack = await LocalWorkspace.createStack({ stackName, projectName, program });

        try {
            await stack.setAllConfig({
                "bar": { value: "abc" },
                "buzz": { value: "secret", secret: true },
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
    }));
    it(`runs an inline program that rejects a promise and exits gracefully`, asyncTest(async () => {
        const program = async () => {
            Promise.reject(new Error());
            return {};
        };
        const stackName = `int_test${getTestSuffix()}`;
        const projectName = "inline_node";
        const stack = await LocalWorkspace.createStack({ stackName, projectName, program });

        // pulumi up
        await assert.rejects(stack.up());

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.strictEqual(destroyRes.summary.kind, "destroy");
        assert.strictEqual(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    }));
    it(`sets pulumi version`, asyncTest(async () => {
        const ws = await LocalWorkspace.create({});
        assert(ws.pulumiVersion);
        assert.strictEqual(versionRegex.test(ws.pulumiVersion), true);
    }));
    it(`respects existing project settings`, asyncTest(async () => {
        const stackName = `int_test${getTestSuffix()}`;
        const projectName = "project_was_overwritten";
        const stack = await LocalWorkspace.createStack(
            {stackName, projectName, program: async() => { return; }},
            {workDir: upath.joinSafe(__dirname, "data", "correct_project")},
        );
        const projectSettings = await stack.workspace.projectSettings();
        assert.strictEqual(projectSettings.name, "correct_project");
        assert.strictEqual(projectSettings.description, "This is a description");
        await stack.workspace.removeStack(stackName);
    }));
});

describe(`checkVersionIsValid`, () => {
    const versionTests = [
        {
            name: "higher_major",
            currentVersion: "100.0.0",
            expectError: true,
        },
        {
            name: "lower_major",
            currentVersion: "1.0.0",
            expectError: true,
        },
        {
            name: "higher_minor",
            currentVersion: "v2.22.0",
            expectError: false,
        },
        {
            name: "lower_minor",
            currentVersion: "v2.1.0",
            expectError: true,
        },
        {
            name: "equal_minor_higher_patch",
            currentVersion: "v2.21.2",
            expectError: false,
        },
        {
            name: "equal_minor_equal_patch",
            currentVersion: "v2.21.1",
            expectError: false,
        },
        {
            name: "equal_minor_lower_patch",
            currentVersion: "v2.21.0",
            expectError: true,
        },
        {
            name: "equal_minor_equal_patch_prerelease",
            // Note that prerelease < release so this case will error
            currentVersion: "v2.21.1-alpha.1234",
            expectError: true,
        },
    ];
    const minVersion = new semver.SemVer("v2.21.1");

    versionTests.forEach(test => {
        it(`validates ${test.currentVersion}`, () => {
            const currentVersion = new semver.SemVer(test.currentVersion);

            if (test.expectError) {
                if (minVersion.major < currentVersion.major) {
                    assert.throws(() => validatePulumiVersion(minVersion, currentVersion), /Major version mismatch./);
                } else {
                    assert.throws(() => validatePulumiVersion(minVersion, currentVersion), /Minimum version requirement failed./);
                }
            } else {
                assert.doesNotThrow(() => validatePulumiVersion(minVersion, currentVersion));
            }
        });
    });
});


const getTestSuffix = () => {
    return Math.floor(100000 + Math.random() * 900000);
};

const normalizeConfigKey = (key: string, projectName: string) => {
    const parts = key.split(":");
    if (parts.length < 2) {
        return `${projectName}:${key}`;
    }
    return "";
};
