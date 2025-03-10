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

import * as assert from "assert";
import { ComponentResource, CustomResource, Resource, ResourceOptions, runtime } from "../..";
import { MockResourceResult } from "../../runtime";
import { getAllTransitivelyReferencedResourceURNs } from "../../runtime/dependsOn";

class TestComponentResource extends ComponentResource {
    constructor(name: string, args: { depth: number }, opts?: ResourceOptions) {
        super("test:index:component", `${name}-${args.depth}`, args, opts);

        if (args.depth > 0) {
            new TestComponentResource(
                name,
                { depth: args.depth - 1 },
                {
                    parent: this,
                },
            );
        } else {
            new TestCustomResource(name, {
                parent: this,
            });
        }
    }
}

class TestCustomResource extends CustomResource {
    constructor(name: string, opts?: ResourceOptions) {
        super("test:index:custom", name, {}, opts);
    }
}

class TestMocks implements runtime.Mocks {
    call(args: runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    async newResource(args: runtime.MockResourceArgs): Promise<MockResourceResult> {
        switch (args.type) {
            case "test:index:component":
            case "test:index:custom":
                return {
                    id: args.name,
                    state: {},
                };
            default:
                throw new Error(`unknown resource type ${args.type}`);
        }
    }
}

describe("runtime", () => {
    beforeEach(() => {
        runtime._reset();
        runtime._resetResourcePackages();
        runtime._resetResourceModules();
    });

    describe("dependsOn", () => {
        describe("getAllTransitivelyReferencedResourceURNs", () => {
            it("collects all transitively referenced resource urns, waiting for all descendant resources to have resolved", async () => {
                // This test was written to reproduce a bug where, due to a missing "await" in the
                // "addTransitivelyReferencedChildResourcesOfComponentResources" function
                // the transitive set of resources could be incomplete, leading to resources being
                // created in the incorrect order.
                //
                // https://github.com/pulumi/pulumi/pull/17629
                await runtime.setMocks(new TestMocks());

                // The 'depth' arg controls how deep the resource hierarchy should be - the bug appears when depth >= 3
                const resource = new TestComponentResource("test", { depth: 3 });
                const urns = await getAllTransitivelyReferencedResourceURNs(new Set([resource]), new Set());

                assert.strictEqual(urns.size, 1, "Expected to find a resource");
            });
        });
    });
});
