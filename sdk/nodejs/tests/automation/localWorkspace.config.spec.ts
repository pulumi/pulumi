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
import * as upath from "upath";

import {
    ConfigMap,
    EngineEvent,
    fullyQualifiedStackName,
    LocalWorkspace,
    ProjectSettings,
    Stack,
} from "../../automation";
import { getTestOrg, getTestSuffix, withTestBackend } from "./util";
import { Config } from "../../config";

describe("LocalWorkspace - Config", () => {
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
    it(`setAllConfigJson`, async () => {
        const projectName = "config_json_test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const ws = await LocalWorkspace.create(withTestBackend({ projectSettings }));
        const stackName = fullyQualifiedStackName(getTestOrg(), projectName, `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);

        // Set config using JSON format
        const configJson = JSON.stringify({
            [`${projectName}:plainKey`]: {
                value: "plainValue",
                secret: false,
            },
            [`${projectName}:secretKey`]: {
                value: "secretValue",
                secret: true,
            },
            [`${projectName}:numberKey`]: {
                value: "42",
                secret: false,
            },
        });

        await stack.setAllConfigJson(configJson);

        // Verify the config was set correctly
        const allConfig = await stack.getAllConfig();

        assert.strictEqual(allConfig[`${projectName}:plainKey`].value, "plainValue");
        assert.strictEqual(allConfig[`${projectName}:plainKey`].secret, false);

        assert.strictEqual(allConfig[`${projectName}:secretKey`].secret, true);

        assert.strictEqual(allConfig[`${projectName}:numberKey`].value, "42");
        assert.strictEqual(allConfig[`${projectName}:numberKey`].secret, false);

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
    // TODO[https://github.com/pulumi/pulumi/issues/7127]: Re-enabled the warning.
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
});

const normalizeConfigKey = (key: string, projectName: string) => {
    const parts = key.split(":");
    if (parts.length < 2) {
        return `${projectName}:${key}`;
    }
    return "";
};
