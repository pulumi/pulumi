// Copyright 2016-2018, Pulumi Corporation.
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
import { CustomResource } from "../index";
import { Output } from "../output";
import * as runtime from "../runtime";
import { asyncTest } from "./util";

class FakeResource extends CustomResource {
    public x?: Output<number>;
    constructor(name: string, props?: { x: number }) {
        super("nodejs:test:FakeResource", name, props);
    }
}

const testModeDisabledError = (err: Error) => {
    return err.message === "Program run without the Pulumi engine available; re-run using the `pulumi` CLI";
};

describe("testMode", () => {
    it("rejects non-test mode", () => {
        runtime._reset();
        runtime._setTestModeEnabled(false);

        // Allocating a resource directly while not in test mode errors out.
        assert.throws(() => { const _ = new FakeResource("fake"); }, testModeDisabledError);
        // Fetching the project name while not in test mode errors out.
        assert.throws(() => { const _ = runtime.getProject(); }, testModeDisabledError);
        // Fetching the stack name while not in test mode errors out.
        assert.throws(() => { const _ = runtime.getStack(); }, testModeDisabledError);
    });
    it("accepts test mode", asyncTest(async () => {
        // Set up all the test mode envvars, so that the test will pass.
        runtime._reset();
        runtime._setTestModeEnabled(true);

        const testProject = "TestProject";
        runtime._setProject(testProject);
        const testStack = "TestStack";
        runtime._setStack(testStack);
        try {
            // Allocating a resource directly while in test mode succeeds.
            let res: FakeResource | undefined;
            assert.doesNotThrow(() => { res = new FakeResource("fake", { x: 42 }); });
            const x = await new Promise((resolve) => res!.x!.apply(resolve));
            assert.strictEqual(x, 42);
            // Fetching the project name while in test mode succeeds.
            let project: string | undefined;
            assert.doesNotThrow(() => { project = runtime.getProject(); });
            assert.strictEqual(project, testProject);
            // Fetching the stack name while in test mode succeeds.
            let stack: string | undefined;
            assert.doesNotThrow(() => { stack = runtime.getStack(); });
            assert.strictEqual(stack, testStack);
        } finally {
            runtime._setTestModeEnabled(false);
            runtime._setProject("");
            runtime._setStack("");
        }
    }));
});
