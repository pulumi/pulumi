// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

const simpleProvider: pulumi.dynamic.ResourceProvider = {
    async create(inputs: any) {
        return {
            id: "0",
            outs: { output: "a", output2: "b" },
        };
    },
};

interface SimpleArgs {
    input: pulumi.Input<string>;
    optionalInput?: pulumi.Input<string>;
}

class SimpleResource extends pulumi.dynamic.Resource {
    output: pulumi.Output<string>;
    output2: pulumi.Output<string>;
    constructor(name, args: SimpleArgs, opts?: pulumi.CustomResourceOptions) {
        super(simpleProvider, name, { ...args, output: undefined, output2: undefined }, opts);
    }
}

class MyComponent extends pulumi.ComponentResource {
    child: SimpleResource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:component:MyComponent", name, {}, opts);
        this.child = new SimpleResource(`${name}-child`, { input: "hello" }, {
            parent: this,
            additionalSecretOutputs: ["output2"],
        });
        this.registerOutputs({});
    }
}

// Scenario #1 - apply a transformation to a CustomResource
const res1 = new SimpleResource("res1", { input: "hello" }, {
    transformations: [
        (t, n, props, opts) => {
            console.log("res1 transformation");
            return {
                props: props,
                opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["output"] }),
            };
        },
    ],
});

// Scenario #2 - apply a transformation to a Component to transform it's children
const res2 = new MyComponent("res2", {
    transformations: [
        (t, n, props, opts) => {
            console.log("res2 transformation");
            if (t === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { optionalInput: "newDefault", ...props },
                    opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["output"] }),
                };
            }
        },
    ],
});

// Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
pulumi.runtime.registerStackTransformation((t, n, props, opts) => {
    console.log("stack transformation");
    if (t === "pulumi-nodejs:dynamic:Resource") {
        return {
            props: { ...props, optionalInput: "stackDefault" },
            opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["output"] }),
        };
    }
});

const res3 = new SimpleResource("res3", { input: "hello" });

// Scenario #4 - transformations are applied in order of increasing specificity
// 1. Stack transformation
// 2. First parent transformation
// 3. Second parent transformation
// 4. (not in this example) Child transformation
const res4 = new MyComponent("res4", {
    transformations: [
        (t, n, props, opts) => {
            console.log("res4 transformation");
            if (t === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { ...props, optionalInput: "default1" },
                    opts,
                };
            }
        },
        (t, n, props, opts) => {
            console.log("res4 transformation 2");
            if (t === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { ...props, optionalInput: "default2" },
                    opts,
                };
            }
        },
    ],
});
