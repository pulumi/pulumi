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
    ConfigMap,
    fullyQualifiedStackName,
    LocalWorkspace,
} from "../../automation";
import { ComponentResource, ComponentResourceOptions, Config } from "../../index";
import { getTestOrg, getTestSuffix } from "./util";

import { withTestBackend } from "./localWorkspace.spec";

const userAgent = "pulumi/pulumi/test";

describe("LocalWorkspace inline programs", async () => {
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
});
