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
import * as resource from "../resource";
import * as runtime from "../runtime";
import { asyncTest } from "./util";

// Not actually a resource (since we can't actually register things during testing).  But follows
// the shape necessary for testing things.
class TestResource {
    // tslint:disable-next-line:variable-name
    private readonly __pulumiResource: boolean = true;

    constructor(public readonly name: string, public readonly data?: resource.Input<resource.Input<TestResource>[]>) {
    }

    public dependsOn() {
        return this.data;
    }
}

async function getTransitiveDependenciesAsync(res: TestResource, opts: resource.ResourceOptions = { }) {
    const result = new Set<TestResource>();
    await runtime.addTransitiveDependenciesAsync("", <any>result, <any>res, opts);
    return result;
}

function setEquals(set: Set<TestResource>, ...array: TestResource[]) {
    assert.equal(set.size, array.length);

    for (const res of array) {
        if (!set.has(res)) {
            assert.fail(`${res.name} not found in [${[...set].map(r => r.name).join()}]`);
        }
    }
}

describe("resource", () => {
    it("is ok with no dependencies", asyncTest(async () => {
        const r1 = new TestResource("r1");
        const deps = await getTransitiveDependenciesAsync(r1);
        setEquals(deps);
    }));
    it("is ok with single dependency", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", [r0]);
        const r2 = new TestResource("r2");
        const deps1 = await getTransitiveDependenciesAsync(r1);
        const deps2 = await getTransitiveDependenciesAsync(r2, { dependsOn: <any>r1 });
        setEquals(deps1);
        setEquals(deps2, r1, r0);
    }));
    it("is ok with transitive dependency", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", [r0]);
        const r2 = new TestResource("r2", [r1]);
        const r3 = new TestResource("r3");
        const deps2 = await getTransitiveDependenciesAsync(r3, { dependsOn: <any>r2 });
        setEquals(deps2, r2, r1, r0);
    }));
    it("is ok with multiple paths to dependency #1", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", [r0]);
        const r2 = new TestResource("r2", [r1, r0]);
        const r3 = new TestResource("r3");
        const deps2 = await getTransitiveDependenciesAsync(r3, { dependsOn: <any>r2 });
        setEquals(deps2, r2, r1, r0);
    }));
    it("is ok with multiple paths to dependency #2", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", []);
        const r2 = new TestResource("r2", [r1, r0]);
        const r3 = new TestResource("r3");
        const deps2 = await getTransitiveDependenciesAsync(r3, { dependsOn: <any>r2 });
        setEquals(deps2, r2, r1, r0);
    }));
    it("is ok with cycle not containing root", asyncTest(async () => {
        const r1 = new TestResource("r1", []);
        const r2 = new TestResource("r2", [r1]);
        (<any>r1.data).push(r2);
        const r3 = new TestResource("r3");
        const deps2 = await getTransitiveDependenciesAsync(r3, { dependsOn: <any>r2 });
        setEquals(deps2, r2, r1);
    }));
    it("is ok with promise dependency #1", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", [Promise.resolve(r0)]);
        const r2 = new TestResource("r2");
        const deps2 = await getTransitiveDependenciesAsync(r2, { dependsOn: <any>r1 });
        setEquals(deps2, r1, r0);
    }));
    it("is ok with promise dependency #2", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", Promise.resolve([r0]));
        const r2 = new TestResource("r2");
        const deps2 = await getTransitiveDependenciesAsync(r2, { dependsOn: <any>r1 });
        setEquals(deps2, r1, r0);
    }));
    it("is ok with promise dependency #3", asyncTest(async () => {
        const r0 = new TestResource("r0");
        const r1 = new TestResource("r1", Promise.resolve([Promise.resolve(r0)]));
        const r2 = new TestResource("r2");
        const deps2 = await getTransitiveDependenciesAsync(r2, { dependsOn: <any>r1 });
        setEquals(deps2, r1, r0);
    }));
    it("circularity throws", asyncTest(async () => {
        const r1 = new TestResource("r1", []);
        const r2 = new TestResource("r2");
        (<any>r1.data).push(r2);
        try {
            const deps2 = await getTransitiveDependenciesAsync(r2, { dependsOn: <any>r1 });
        }
        catch (err) {
            const message: string = err.message;
            if (!message.includes("cycle")) {
                throw new Error("Unexpected error");
            }

            return;
        }

        assert.fail("Should have thrown error.");
    }));
});
