// Copyright 2026, Pulumi Corporation.
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
import * as pulumi from "../index";
import { MockCallArgs, MockCallResult, MockResourceArgs, MockResourceResult } from "../runtime";

class MyResource extends pulumi.CustomResource {
    tags!: pulumi.Output<Record<string, string>>;
    constructor(name: string, props?: Record<string, any>, opts?: pulumi.CustomResourceOptions) {
        super("test:index:MyResource", name, props ?? {}, opts);
    }
}

const tagsTransform: pulumi.ResourceTransform = (args) => ({
    props: { ...args.props, tags: { ...(args.props.tags ?? {}), foo: "bar" } },
    opts: args.opts,
});

describe("mocks: transforms", function () {
    describe("mocks without transform hooks", function () {
        const resourceTransforms: pulumi.ResourceTransform[] = [];

        before(async () => {
            await pulumi.runtime.setMocks({
                call: (args: MockCallArgs): MockCallResult => ({ ...args.inputs }),
                newResource: (args: MockResourceArgs): MockResourceResult => {
                    resourceTransforms.push(...(args.transforms ?? []));
                    return {
                        id: `${args.name}_id`,
                        state: { ...args.inputs },
                    };
                },
            });
            pulumi.runtime.registerResourceTransform(tagsTransform);
            pulumi.runtime.registerInvokeTransform((args) => ({
                args: { ...args.args, extra: "added" },
                opts: args.opts,
            }));
        });

        it("accepts and delivers transform registrations without running them", async () => {
            const res = new MyResource("res", { tags: { group: "webservers" } }, { transforms: [tagsTransform] });
            assert.deepStrictEqual(await res.tags.promise(), { group: "webservers" });
            assert.deepStrictEqual(resourceTransforms, [tagsTransform]);

            const result = await pulumi.runtime.invoke("test:index:MyFunction", { orig: "value" });
            assert.strictEqual(result.orig, "value");
            assert.strictEqual(result.extra, undefined);
        });
    });

    describe("transform-aware mocks", function () {
        const stackTransforms: pulumi.ResourceTransform[] = [];
        const stackInvokeTransforms: pulumi.InvokeTransform[] = [];

        before(async () => {
            await pulumi.runtime.setMocks({
                call: async (args: MockCallArgs): Promise<MockCallResult> => {
                    let callArgs = { ...args.inputs };
                    for (const transform of stackInvokeTransforms) {
                        const result = await transform({ token: args.token, args: callArgs, opts: {} });
                        if (result !== undefined) {
                            callArgs = { ...result.args };
                        }
                    }
                    return callArgs;
                },
                newResource: async (args: MockResourceArgs): Promise<MockResourceResult> => {
                    let props = { ...args.inputs };
                    for (const transform of [...(args.transforms ?? []), ...stackTransforms]) {
                        const result = await transform({
                            custom: args.custom ?? false,
                            type: args.type,
                            name: args.name,
                            props: props,
                            opts: {},
                        });
                        if (result !== undefined) {
                            props = { ...result.props };
                        }
                    }
                    return { id: `${args.name}_id`, state: props };
                },
                registerTransform: (transform) => {
                    stackTransforms.push(transform);
                },
                registerInvokeTransform: (transform) => {
                    stackInvokeTransforms.push(transform);
                },
            });
        });

        it("exposes a resource's own transforms to newResource", async () => {
            const res = new MyResource("res", { tags: { group: "webservers" } }, { transforms: [tagsTransform] });
            assert.deepStrictEqual(await res.tags.promise(), { group: "webservers", foo: "bar" });
        });

        it("notifies the mocks of stack transform registrations", async () => {
            pulumi.runtime.registerResourceTransform(tagsTransform);
            const res = new MyResource("res2", { tags: { group: "webservers" } });
            assert.deepStrictEqual(await res.tags.promise(), { group: "webservers", foo: "bar" });
            assert.deepStrictEqual(stackTransforms, [tagsTransform]);
        });

        it("notifies the mocks of invoke transform registrations", async () => {
            pulumi.runtime.registerInvokeTransform((args) => ({
                args: { ...args.args, extra: "added" },
                opts: args.opts,
            }));
            const result = await pulumi.runtime.invoke("test:index:MyFunction", { orig: "value" });
            assert.strictEqual(result.orig, "value");
            assert.strictEqual(result.extra, "added");
        });
    });
});
