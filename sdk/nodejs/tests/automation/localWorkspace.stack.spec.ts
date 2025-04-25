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
import * as upath from "upath";

import {
    ConfigMap,
    fullyQualifiedStackName,
    LocalWorkspace,
} from "../../automation";
import { ComponentResource, ComponentResourceOptions, Config } from "../../index";
import { getTestOrg, getTestSuffix } from "./util";

const userAgent = "pulumi/pulumi/test";

import { withTestBackend } from "./localWorkspace.spec";

describe("Stack lifecycle", async() => {
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
});
