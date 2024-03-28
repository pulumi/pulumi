// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Random } from "./random";

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
    xTransforms: [
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
    xTransforms: [
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
pulumi.runtime.xRegisterStackTransform(async ({ type, props, opts }) => {
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
    xTransforms: [
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
    xTransforms: [
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
