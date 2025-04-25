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

import { withTestBackend } from "./localWorkspace.spec";

import {
    fullyQualifiedStackName,
    LocalWorkspace,
    ProjectSettings,
} from "../../automation";
import { getTestOrg, getTestSuffix } from "./util";

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
