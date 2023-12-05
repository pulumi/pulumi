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
import { all } from "../output";
import * as runtime from "../runtime";
import {
    allAliases,
    createUrn,
    ProviderResource,
    CustomResource,
    ComponentResource,
    ComponentResourceOptions,
    CustomResourceOptions,
    DependencyProviderResource,
} from "../resource";

class MyResource extends ComponentResource {
    constructor(name: string, opts?: ComponentResourceOptions) {
        super("my:mod:MyResource", name, {}, opts);
    }
}

class MyParentResource extends ComponentResource {
    child: MyResource;
    constructor(name: string, opts?: ComponentResourceOptions) {
        super("my:mod:MyParentResource", name, {}, opts);
        this.child = new MyResource(`${name}-child`, { parent: this });
    }
}

describe("createUrn", () => {
    before(() => {
        runtime._setProject("myproject");
        runtime._setStack("mystack");
    });

    after(() => {
        runtime._setProject(undefined);
        runtime._setStack(undefined);
    });

    it("handles name and type", async () => {
        const urn = await createUrn("n", "t").promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::t::n");
    });

    it("handles name and type and parent", async () => {
        const res = new MyResource("myres");
        const urn = await createUrn("n", "t", res).promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::my:mod:MyResource$t::n");
    });

    it("handles name and type and parent with parent", async () => {
        const res = new MyParentResource("myres");
        const urn = await createUrn("n", "t", res.child).promise();
        assert.strictEqual(urn, "urn:pulumi:mystack::myproject::my:mod:MyParentResource$my:mod:MyResource$t::n");
    });
});

class TestResource extends ComponentResource {
    constructor(name: string, opts?: CustomResourceOptions) {
        super("test:resource:type", name, {}, opts);
    }
}

describe("allAliases", () => {
    before(() => {
        runtime._setProject("project");
        runtime._setStack("stack");
    });

    after(() => {
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
            parentAliases: [{ type: "test:resource:type3" }],
            childAliases: [{ name: "myres-child2" }],
            results: [
                "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
                "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child",
                "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child2",
            ],
        },
        {
            name: "one child alias (name), one parent alias (name)",
            parentAliases: [{ name: "myres2" }],
            childAliases: [{ name: "myres-child2" }],
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
        it(testCase.name, async () => {
            const res = new TestResource("myres", { aliases: testCase.parentAliases });
            const aliases = allAliases(testCase.childAliases, "myres-child", "test:resource:child", res, "myres");
            assert.strictEqual(aliases.length, testCase.results.length);
            const aliasURNs = await all(aliases).promise();
            for (let i = 0; i < aliasURNs.length; i++) {
                assert.strictEqual(aliasURNs[i], testCase.results[i]);
            }
        });
    }
});

describe("DependencyProviderResource", () => {
    describe("getPackage", () => {
        it("returns the expected package", () => {
            const res = new DependencyProviderResource(
                "urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0",
            );
            assert.strictEqual(res.getPackage(), "aws");
        });
    });
});

describe("CustomResource", () => {
    runtime.setMocks({
        call: (_) => {
            throw new Error("unexpected call");
        },
        newResource: (args) => {
            return { id: `${args.name}_id`, state: {} };
        },
    });

    // https://github.com/pulumi/pulumi/issues/13777
    it("saves provider with same package as the resource in __prov", async () => {
        const provider = new MyProvider("prov");
        const custom = new MyCustomResource("custom", { provider: provider });
        assert.strictEqual(custom.__prov, provider);
    });

    // https://github.com/pulumi/pulumi/issues/13777
    it("does not save provider with different package as the resource in __prov", async () => {
        const provider = new MyOtherProvider("prov");
        const custom = new MyCustomResource("custom", { provider: provider });
        assert.strictEqual(custom.__prov, undefined);
    });
});

describe("ComponentResource", () => {
    runtime.setMocks({
        call: (_) => {
            throw new Error("unexpected call");
        },
        newResource: (args) => {
            return { id: `${args.name}_id`, state: {} };
        },
    });

    // https://github.com/pulumi/pulumi/issues/12161
    it("propagates provider to children", async () => {
        const provider = new MyProvider("prov");
        const component = new MyResource("comp", { provider: provider });
        const custom = new MyCustomResource("custom", { parent: component });
        assert.strictEqual(custom.__prov, provider);
    });

    // https://github.com/pulumi/pulumi/issues/12161
    it("propagates providers list to children", async () => {
        const provider = new MyProvider("prov");
        const component = new MyResource("comp", { providers: [provider] });
        const custom = new MyCustomResource("custom", { parent: component });
        assert.strictEqual(custom.__prov, provider);
    });
});

describe("RemoteComponentResource", () => {
    runtime.setMocks({
        call: (_) => {
            throw new Error("unexpected call");
        },
        newResource: (args) => {
            return { id: `${args.name}_id`, state: {} };
        },
    });

    // https://github.com/pulumi/pulumi/issues/13777
    it("saves provider with same package as the resource in __prov", async () => {
        const provider = new MyProvider("prov");
        const comp = new MyRemoteComponentResource("comp", { provider: provider });
        assert.strictEqual(comp.__prov, provider);
    });

    // https://github.com/pulumi/pulumi/issues/13777
    it("does not save provider with different package as the resource in __prov", async () => {
        const provider = new MyOtherProvider("prov");
        const comp = new MyRemoteComponentResource("comp", { provider: provider });
        assert.strictEqual(comp.__prov, undefined);
    });
});

class MyProvider extends ProviderResource {
    constructor(name: string) {
        super("test", name);
    }
}

class MyOtherProvider extends ProviderResource {
    constructor(name: string) {
        super("other", name);
    }
}

class MyCustomResource extends CustomResource {
    constructor(name: string, opts?: CustomResourceOptions) {
        super("test:index:MyCustomResource", name, {}, opts);
    }
}

class MyRemoteComponentResource extends ComponentResource {
    constructor(name: string, opts?: ComponentResourceOptions) {
        super("test:index:MyRemoteComponentResource", name, {}, opts, true /*remote*/);
    }
}

// Regression test for https://github.com/pulumi/pulumi/issues/12032
describe("parent and dependsOn are the same 12032", () => {
    runtime.setMocks({
        call: (_) => {
            throw new Error("unexpected call");
        },
        newResource: (args) => {
            return { id: `${args.name}_id`, state: {} };
        },
    });

    // https://github.com/pulumi/pulumi/issues/12161
    it("runs without error", async () => {
        const parent = new ComponentResource("pkg:index:first", "first");
        const child = new ComponentResource(
            "pkg:index:second",
            "second",
            {},
            {
                parent,
                dependsOn: parent,
            },
        );

        // This would result in warnings about leaked promises before the fix.
        new MyCustomResource("myresource", {
            parent: child,
        });
    });
});
