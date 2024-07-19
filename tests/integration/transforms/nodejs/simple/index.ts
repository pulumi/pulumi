// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Component, Random, TestProvider} from "./random";

class MyComponent extends pulumi.ComponentResource {
    child: Random;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:component:MyComponent", name, {}, opts);
        this.child = new Random(`${name}-child`, { length: 5 }, {
            parent: this,
            additionalSecretOutputs: ["length"],
        });
        this.registerOutputs({});
    }
}

// Scenario #1 - apply a transform to a CustomResource
const res1 = new Random("res1", { length: 5 }, {
    transforms: [
        async ({ props, opts }) => {
            console.log("res1 transform");
            return {
                props: props,
                opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["result"] }),
            };
        },
    ],
});

// Scenario #2 - apply a transform to a Component to transform it's children
const res2 = new MyComponent("res2", {
    transforms: [
        async ({ type, props, opts }) => {
            console.log("res2 transform");
            if (type === "testprovider:index:Random") {
                return {
                    props: { prefix: "newDefault", ...props },
                    opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["result"] }),
                };
            }
        },
    ],
});

// Scenario #3 - apply a transform to the Stack to transform all (future) resources in the stack
pulumi.runtime.registerStackTransform(async ({ type, props, opts }) => {
    console.log("stack transform");
    if (type === "testprovider:index:Random") {
        return {
            props: { ...props, prefix: "stackDefault" },
            opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["result"] }),
        };
    }
});

const res3 = new Random("res3", { length: pulumi.secret(5) });

// Scenario #4 - transforms are applied in order of decreasing specificity
// 1. (not in this example) Child transform
// 2. First parent transform
// 3. Second parent transform
// 4. Stack transform
const res4 = new MyComponent("res4", {
    transforms: [
        async ({ type, props, opts }) => {
            console.log("res4 transform");
            if (type === "testprovider:index:Random") {
                return {
                    props: { ...props, prefix: "default1" },
                    opts,
                };
            }
        },
        async ({ type, props, opts }) => {
            console.log("res4 transform 2");
            if (type === "testprovider:index:Random") {
                return {
                    props: { ...props, prefix: "default2" },
                    opts,
                };
            }
        },
    ],
});


// Scenario #5 - mutate the properties of a resource
const res5 = new Random("res5", { length: 10 }, {
    transforms: [
        async ({ type, props, opts }) => {
            console.log("res5 transform");
            if (type === "testprovider:index:Random") {
                const length = props["length"];
                props["length"] = length * 2;
                return {
                    props: props,
                    opts: opts,
                };
            }
        },
    ],
});

// Scenario #6 - mutate the provider on a custom resource
const provider1 = new TestProvider("provider1");
const provider2 = new TestProvider("provider2");

const res6 = new Random("res6", { length: 10 }, {
    provider: provider1,
    transforms: [
        async ({ type, props, opts }) => {
            console.log("res6 transform");
            return {
                props: props,
                opts: pulumi.mergeOptions(opts, { provider: provider2 }),
            };
        },
    ],
});

// Scenario #7 - mutate the provider on a component resource
const res7 = new Component("res7", { length: 10 }, {
    provider: provider1,
    transforms: [
        async ({ type, props, opts }) => {
            console.log("res7 transform");
            return {
                props: props,
                opts: pulumi.mergeOptions(opts, { provider: provider2 }),
            };
        },
    ],
});

pulumi.runtime.registerInvokeTransform(async ({ token, args, opts }) => {
    return {
	args: { ...args, length: 11 },
	opts: opts,
    };
});

const res8 = new Random("res8", { length: pulumi.secret(5) });
const args = {
    length: 10,
    prefix: "test",
};

async () => {
    const result = await res8.randomInvoke(args);
    if (result.length !== 11) {
	throw new Error(`expected length to be 11, got ${result.length}`);
    }
    if (result.prefix !== "test") {
	throw new Error(`expected prefix to be test, got ${result.prefix}`);
    }
 };
