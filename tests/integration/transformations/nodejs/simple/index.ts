// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

const simpleProvider: pulumi.dynamic.ResourceProvider = {
    async create(inputs: any) {
        return {
            id: "0",
            outs: { output: "goodbye" },
        };
    },
};

interface SimpleArgs {
    input: pulumi.Input<string>;
    optionalInput?: pulumi.Input<string>;
}

class SimpleResource extends pulumi.dynamic.Resource {
    output: pulumi.Output<string>;
    constructor(name, args: SimpleArgs, opts?: pulumi.CustomResourceOptions) {
        super(simpleProvider, name, { ...args, output: undefined }, opts);
    }
}

class MyComponent extends pulumi.ComponentResource {
    child: SimpleResource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:component:MyComponent", name, {}, opts);
        this.child = new SimpleResource("child", { input: "hello" }, { parent: this });
        this.registerOutputs({});
    }
}

// Scenario #1 - rename a resource
const res1 = new SimpleResource("res1", { input: "hello" }, {
    transformations: [
        (t, n, props, opts) => {
            console.log("res1 transformation");
            return {
                props: props,
                opts: { ...opts, additionalSecretOutputs: ["output"] },
            };
        },
    ],
});

const res2 = new MyComponent("res2", {
    transformations: [
        (t, n, props, opts) => {
            console.log("res2 transformation");
            if (t === "pulumi-nodejs:dynamic:Resource") {
                const newAso = [...((opts as pulumi.CustomResourceOptions).additionalSecretOutputs || []), "output"];
                return {
                    props: { optionalInput: "newDefault", ...props },
                    opts: { ...opts, additionalSecretOutputs: newAso },
                };
            }
        },
    ],
});

pulumi.runtime.registerStackTransformation((t, n, props, opts) => {
    console.log("stack transformation");
    if (t === "pulumi-nodejs:dynamic:Resource") {
        const newAso = [...((opts as pulumi.CustomResourceOptions).additionalSecretOutputs || []), "output"];
        return {
            props: { optionalInput: "stackDefault", ...props },
            opts: { ...opts, additionalSecretOutputs: newAso },
        };
    }
});

const res3 = new SimpleResource("res3", { input: "hello" });
