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

/* eslint-disable */

import * as assert from "assert";
import { Output, all, concat, interpolate, output } from "../output";
import * as runtime from "../runtime";
import { asyncTest } from "./util";
import { allAliases, createUrn, ComponentResource, CustomResourceOptions, DependencyProviderResource } from "../resource";

class MyResource extends ComponentResource {
    constructor(name: string, opts?: CustomResourceOptions) {
        super("my:mod:MyResource", name, {}, opts);
    }
}

class MyParentResource extends ComponentResource {
    child: MyResource;
    constructor(name: string, opts?: CustomResourceOptions) {
        super("my:mod:MyParentResource", name, {}, opts);
        this.child = new MyResource(`${name}-child`, { parent: this });
    }
}

describe("createUrn", () => {
    before(() => {
        runtime._setTestModeEnabled(true);
        runtime._setProject("myproject");
        runtime._setStack("mystack");
    });

    after(() => {
        runtime._setTestModeEnabled(false);
        runtime._setProject(undefined);
        runtime._setStack(undefined);
    });

    it("handles name and type", asyncTest(async () => {
        const urn = await createUrn("n", "t").promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::t::n");
    }));

    it("handles name and type and parent", asyncTest(async () => {
        const res = new MyResource("myres");
        const urn = await createUrn("n", "t", res).promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::my:mod:MyResource$t::n");
    }));

    it("handles name and type and parent with parent", asyncTest(async () => {
        const res = new MyParentResource("myres");
        const urn = await createUrn("n", "t", res.child).promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::my:mod:MyParentResource$my:mod:MyResource$t::n");
    }));
});

class TestResource extends ComponentResource {
    constructor(name: string, opts?: CustomResourceOptions) {
        super("test:resource:type", name, {}, opts);
    }
}

describe("allAliases", () => {
    before(() => {
        runtime._setTestModeEnabled(true);
        runtime._setProject("project");
        runtime._setStack("stack");
    });

    after(() => {
        runtime._setTestModeEnabled(false);
        runtime._setProject(undefined);
        runtime._setStack(undefined);
    });

    const testCases = [
        {
            name: "no aliases",
            parentAliases: [],
            childAliases: [],
            results: [],
        },
        {
            name: "one child alias (type), no parent aliases",
            parentAliases: [],
            childAliases: [{ type: "test:resource:child2" }],
            results: ["urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres-child"],
        },
        {
            name: "one child alias (name), no parent aliases",
            parentAliases: [],
            childAliases: [{ name: "child2" }],
            results: ["urn:pulumi:stack::project::test:resource:type$test:resource:child::child2"],
        },
        {
            name: "one child alias (name), one parent alias (type)",
        	parentAliases:  [{type: "test:resource:type3"}],
        	childAliases:   [{name: "myres-child2"}],
        	results: [
        		"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
        		"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child",
        		"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child2",
            ],
        },
        {
            name: "one child alias (name), one parent alias (name)",
        	parentAliases:  [{name: "myres2"}],
        	childAliases:   [{name: "myres-child2"}],
        	results: [
        		"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
        		"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child",
        		"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child2",
            ],
        },
        {
            name: "two child aliases, three parent aliases",
            parentAliases: [{ name: "myres2" }, { type: "test:resource:type3" }, { name: "myres3" }],
            childAliases: [{ name: "myres-child2" }, { type: "test:resource:child2" }],
            results: [
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres-child",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child2",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres2-child",
                "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child",
                "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child2",
                "urn:pulumi:stack::project::test:resource:type3$test:resource:child2::myres-child",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres3-child",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres3-child2",
                "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres3-child",
            ],
        },
    ];

    for (const testCase of testCases) {
        it(testCase.name, asyncTest(async () => {
            const res = new TestResource("myres", { aliases: testCase.parentAliases });
            const aliases = allAliases(testCase.childAliases, "myres-child", "test:resource:child", res, "myres");
            assert.strictEqual(aliases.length, testCase.results.length);
            const aliasURNs = await all(aliases).promise();
            for (let i = 0; i < aliasURNs.length; i++) {
                assert.strictEqual(aliasURNs[i], testCase.results[i]);
            }
        }));
    }
});

describe("DependencyProviderResource", () => {
    describe("getPackage", () => {
        it("returns the expected package", () => {
            const res = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            assert.strictEqual(res.getPackage(), "aws");
        });
    });
});
