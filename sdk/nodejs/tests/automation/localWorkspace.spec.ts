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
import * as upath from "upath";

import { Config } from "../../index";
import { ConfigMap, normalizeConfigKey } from "../../x/automation/config";
import { LocalWorkspace } from "../../x/automation/localWorkspace";
import { ProjectSettings } from "../../x/automation/projectSettings";
import { Stack } from "../../x/automation/stack";
import { asyncTest } from "../util";

describe("LocalWorkspace", () => {
    it(`projectSettings from yaml/yml/json`, asyncTest(async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create({ workDir: upath.joinSafe(__dirname, "data", ext) });
            const settings = await ws.projectSettings();
            assert(settings.name, "testproj");
            assert(settings.runtime.name, "go");
            assert(settings.description, "A minimal Go Pulumi program");

        }
    }));

    it(`stackSettings from yaml/yml/json`, asyncTest(async () => {
        for (const ext of ["yaml", "yml", "json"]) {
            const ws = await LocalWorkspace.create({ workDir: upath.joinSafe(__dirname, "data", ext) });
            const settings = await ws.stackSettings("dev");
            assert.equal(settings.secretsProvider, "abc");
            assert.equal(settings.config!["plain"].value, "plain");
            assert.equal(settings.config!["secure"].secure, "secret");
        }
    }));

    it(`create/select/remove LocalWorkspace stack`, asyncTest(async () => {
        const projectSettings = new ProjectSettings();
        projectSettings.name = "node_test";
        projectSettings.runtime.name = "nodejs";
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        await ws.createStack(stackName);
        await ws.selectStack(stackName);
        await ws.removeStack(stackName);
    }));

    it(`create/select/createOrSelect Stack`, asyncTest(async () => {
        const projectSettings = new ProjectSettings();
        projectSettings.name = "node_test";
        projectSettings.runtime.name = "nodejs";
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        await Stack.create(stackName, ws);
        await Stack.select(stackName, ws);
        await Stack.createOrSelect(stackName, ws);
        await ws.removeStack(stackName);
    }));
    it(`Config`, asyncTest(async () => {
        const projectName = "node_test";
        const projectSettings = new ProjectSettings();
        projectSettings.name = projectName;
        projectSettings.runtime.name = "nodejs";
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
            const empty = await stack.getConfig(plainKey);
        } catch (error) {
            caught++;
        }
        assert.equal(caught, 1, "expected config get on empty value to throw");

        let values = await stack.getAllConfig();
        assert.equal(Object.keys(values).length, 0, "expected stack config to be empty");
        await stack.setAllConfig(config);
        values = await stack.getAllConfig();
        assert.equal(values[plainKey].value, "abc");
        assert.equal(values[plainKey].secret, false);
        assert.equal(values[secretKey].value, "def");
        assert.equal(values[secretKey].secret, true);

        await stack.removeConfig("plain");
        values = await stack.getAllConfig();
        assert.equal(Object.keys(values).length, 1, "expected stack config to be empty");
        await stack.setConfig("foo", { value: "bar" });
        values = await stack.getAllConfig();
        assert.equal(Object.keys(values).length, 2, "expected stack config to be empty");

        await ws.removeStack(stackName);
    }));
    it(`can list stacks and currently selected stack`, asyncTest(async () => {
        const projectSettings = new ProjectSettings();
        projectSettings.name = `node_list_test${getTestSuffix()}`;
        projectSettings.runtime.name = "nodejs";
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackNamer = () => `int_test${getTestSuffix()}`;
        const stackNames: string[] = [];
        for (let i = 0; i < 2; i++) {
            const stackName = stackNamer();
            stackNames[i] = stackName;
            await Stack.create(stackName, ws);
            const stackSummary = await ws.stack();
            assert.equal(stackSummary?.current, true);
            const stacks = await ws.listStacks();
            assert.equal(stacks.length, i + 1);
        }

        for (const name of stackNames) {
            await ws.removeStack(name);
        }
    }));
    it(`stack status methods`, asyncTest(async () => {
        const projectSettings = new ProjectSettings();
        projectSettings.name = "node_test";
        projectSettings.runtime.name = "nodejs";
        const ws = await LocalWorkspace.create({ projectSettings });
        const stackName = `int_test${getTestSuffix()}`;
        const stack = await Stack.create(stackName, ws);
        const histroy = await stack.history();
        assert.equal(histroy.length, 0);
        const info = await stack.info();
        assert.equal(typeof (info), "undefined");
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
        assert.equal(Object.keys(upRes.outputs).length, 3);
        assert.equal(upRes.outputs["exp_static"].value, "foo");
        assert.equal(upRes.outputs["exp_static"].secret, false);
        assert.equal(upRes.outputs["exp_cfg"].value, "abc");
        assert.equal(upRes.outputs["exp_cfg"].secret, false);
        assert.equal(upRes.outputs["exp_secret"].value, "secret");
        assert.equal(upRes.outputs["exp_secret"].secret, true);
        assert.equal(upRes.summary.kind, "update");
        assert.equal(upRes.summary.result, "succeeded");

        // pulumi preview
        await stack.preview();
        // TODO: update assertions when we have stuctured output

        // pulumi refresh
        const refRes = await stack.refresh();
        assert.equal(refRes.summary.kind, "refresh");
        assert.equal(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.equal(destroyRes.summary.kind, "destroy");
        assert.equal(destroyRes.summary.result, "succeeded");

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
        assert.equal(Object.keys(upRes.outputs).length, 3);
        assert.equal(upRes.outputs["exp_static"].value, "foo");
        assert.equal(upRes.outputs["exp_static"].secret, false);
        assert.equal(upRes.outputs["exp_cfg"].value, "abc");
        assert.equal(upRes.outputs["exp_cfg"].secret, false);
        assert.equal(upRes.outputs["exp_secret"].value, "secret");
        assert.equal(upRes.outputs["exp_secret"].secret, true);
        assert.equal(upRes.summary.kind, "update");
        assert.equal(upRes.summary.result, "succeeded");

        // pulumi preview
        await stack.preview();
        // TODO: update assertions when we have stuctured output

        // pulumi refresh
        const refRes = await stack.refresh();
        assert.equal(refRes.summary.kind, "refresh");
        assert.equal(refRes.summary.result, "succeeded");

        // pulumi destroy
        const destroyRes = await stack.destroy();
        assert.equal(destroyRes.summary.kind, "destroy");
        assert.equal(destroyRes.summary.result, "succeeded");

        await stack.workspace.removeStack(stackName);
    }));
});

const getTestSuffix = () => {
    return Math.floor(100000 + Math.random() * 900000);
};
