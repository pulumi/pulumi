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
import { Output, concat, interpolate, output } from "../output";
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

describe("allAliases", () => {
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

    it("no aliases", asyncTest(async () => {
        const res = new MyResource("myres");
        const aliases = allAliases([], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 0);
    }));

    it("one child alias (type), no parent aliases", asyncTest(async () => {
        const res = new MyResource("myres");
        const aliases = allAliases([{ type: "t2" }], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 1);
    }));

    it("one child alias (name), no parent aliases", asyncTest(async () => {
        const res = new MyResource("myres");
        const aliases = allAliases([{ name: "n2" }], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 1);
    }));

    it("one child alias (name), one parent alias (type)", asyncTest(async () => {
        const res = new MyResource("myres", { aliases: [{ type: "t3" }] });
        const aliases = allAliases([{ name: "n2" }], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 3);
    }));

    it("one child alias (name), one parent alias (name)", asyncTest(async () => {
        const res = new MyResource("myres", { aliases: [{ name: "myres2" }] });
        const aliases = allAliases([{ name: "n2" }], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 3);
    }));

    it("two child aliases, three parent aliases ", asyncTest(async () => {
        const res = new MyResource("myres", { aliases: [{ name: "myres2" }, { type: "t3" }, { name: "myres3" }] });
        const aliases = allAliases([{ name: "n2" }, { type: "t2" }], "n", "t", res, "myres");
        assert.strictEqual(aliases.length, 11);
    }));
});

describe("DependencyProviderResource", () => {
    describe("getPackage", () => {
        it("returns the expected package", () => {
            const res = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            assert.strictEqual(res.getPackage(), "aws");
        });
    });
});
