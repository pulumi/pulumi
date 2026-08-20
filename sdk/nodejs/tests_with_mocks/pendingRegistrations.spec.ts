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
import { leakedPromises } from "../runtime/debuggable";
import { MockResourceArgs, MockResourceResult } from "../runtime";
import { getPendingResourceRegistrations } from "../runtime/state";

class ReproComponent extends pulumi.ComponentResource {
    constructor(name: string, props: pulumi.Inputs) {
        super("my:module:ReproComponent", name, props);
    }
}

class MyCustom extends pulumi.CustomResource {
    invokeArn!: pulumi.Output<string>;
    constructor(name: string, opts: pulumi.CustomResourceOptions) {
        super("test:index:MyCustom", name, { invokeArn: undefined }, opts);
    }
}

class SelfCyclicCustom extends pulumi.CustomResource {
    constructor(name: string, value: pulumi.Input<string>) {
        super("test:index:MyCustom", name, { value }, {});
    }
}

class UnresolvedSubclass extends ReproComponent {
    constructor(name: string) {
        const [invokeArn] = pulumi.deferredOutput<string>();
        super(name, { invokeArn });
        new MyCustom("child", { parent: this });
    }
}

class CyclicSubclass extends ReproComponent {
    constructor(name: string) {
        const [invokeArn, resolveInvokeArn] = pulumi.deferredOutput<string>();
        super(name, { invokeArn });
        const child = new MyCustom("cyclicChild", { parent: this });
        resolveInvokeArn(child.invokeArn);
    }
}

class ApplyCyclicSubclass extends ReproComponent {
    constructor(name: string) {
        const [invokeArn, resolveInvokeArn] = pulumi.deferredOutput<string>();
        super(name, { invokeArn });
        const child = new MyCustom("applyChild", { parent: this });
        resolveInvokeArn(child.invokeArn.apply((v) => v));
    }
}

async function settle(): Promise<void> {
    for (let i = 0; i < 20; i++) {
        await new Promise((resolve) => setImmediate(resolve));
    }
}

async function diagnose(build: () => void): Promise<string> {
    build();
    await settle();
    const [leaks, message] = leakedPromises();
    assert.ok(leaks.size > 0);
    return message;
}

describe("pending resource registration diagnostics", () => {
    let oldSkipComponentInputsEnv: string | undefined;

    before(async () => {
        oldSkipComponentInputsEnv = process.env.PULUMI_NODEJS_SKIP_COMPONENT_INPUTS;
        Reflect.deleteProperty(process.env, "PULUMI_NODEJS_SKIP_COMPONENT_INPUTS");

        await pulumi.runtime.setMocks({
            newResource: (args: MockResourceArgs): MockResourceResult => ({
                id: `${args.name}_id`,
                state: { ...args.inputs, invokeArn: "arn:aws:fake" },
            }),
            call: (args) => args.inputs,
        });
    });

    afterEach(() => {
        getPendingResourceRegistrations().clear();
        leakedPromises();
    });

    after(() => {
        if (oldSkipComponentInputsEnv !== undefined) {
            process.env.PULUMI_NODEJS_SKIP_COMPONENT_INPUTS = oldSkipComponentInputsEnv;
        }
    });

    it("describes registrations stuck on a component input that never resolves", async () => {
        const message = await diagnose(() => new UnresolvedSubclass("myResource"));
        assert.match(
            message,
            /"myResource" \[my:module:ReproComponent\] was waiting for the value of its input property "invokeArn", a deferred output that was never resolved/,
        );
        assert.match(
            message,
            /"child" \[test:index:MyCustom\] was waiting for its parent "myResource" \[my:module:ReproComponent\] to finish registering/,
        );
        assert.match(message, /registerResourceOutputs/);
    });

    it("explains the cycle when a deferred component input is resolved by a child", async () => {
        const message = await diagnose(() => new CyclicSubclass("cyclicResource"));
        assert.match(
            message,
            /input "invokeArn" of resource "cyclicResource" \[my:module:ReproComponent\] is a deferred output that is resolved by "cyclicChild" \[test:index:MyCustom\], a descendant/,
        );
        assert.match(message, /registerResourceOutputs/);
    });

    it("explains the cycle when a deferred component input is resolved by an output derived from a child", async () => {
        const message = await diagnose(() => new ApplyCyclicSubclass("applyResource"));
        assert.match(
            message,
            /input "invokeArn" of resource "applyResource" \[my:module:ReproComponent\] is a deferred output that is resolved by "applyChild" \[test:index:MyCustom\]/,
        );
    });

    it("explains the cycle when a deferred input is resolved from the resource's own output", async () => {
        const message = await diagnose(() => {
            const [value, resolveValue] = pulumi.deferredOutput<string>();
            const res = new SelfCyclicCustom("selfCyclic", value);
            resolveValue(res.urn);
        });
        assert.match(
            message,
            /input "value" of resource "selfCyclic" \[test:index:MyCustom\] is a deferred output that is resolved from one of/,
        );
    });

    it("explains the cycle when two components' deferred inputs are resolved from each other's outputs", async () => {
        const message = await diagnose(() => {
            const [valueA, resolveA] = pulumi.deferredOutput<string>();
            const [valueB, resolveB] = pulumi.deferredOutput<string>();
            const a = new ReproComponent("mutualA", { value: valueA });
            const b = new ReproComponent("mutualB", { value: valueB });
            resolveA(b.urn);
            resolveB(a.urn);
        });
        assert.match(
            message,
            /is a deferred output that is resolved by an output of "mutual[AB]" \[my:module:ReproComponent\]/,
        );
        assert.match(message, /own registration is in turn waiting on/);
    });
});
