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
        ({ props, opts }) => {
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
        ({ type, props, opts }) => {
            console.log("res2 transformation");
            if (type === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { optionalInput: "newDefault", ...props },
                    opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["output"] }),
                };
            }
        },
    ],
});

// Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
pulumi.runtime.registerStackTransformation(({ type, props, opts }) => {
    console.log("stack transformation");
    if (type === "pulumi-nodejs:dynamic:Resource") {
        return {
            props: { ...props, optionalInput: "stackDefault" },
            opts: pulumi.mergeOptions(opts, { additionalSecretOutputs: ["output"] }),
        };
    }
});

const res3 = new SimpleResource("res3", { input: "hello" });

// Scenario #4 - transformations are applied in order of decreasing specificity
// 1. (not in this example) Child transformation
// 2. First parent transformation
// 3. Second parent transformation
// 4. Stack transformation
const res4 = new MyComponent("res4", {
    transformations: [
        ({ type, props, opts }) => {
            console.log("res4 transformation");
            if (type === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { ...props, optionalInput: "default1" },
                    opts,
                };
            }
        },
        ({ type, props, opts }) => {
            console.log("res4 transformation 2");
            if (type === "pulumi-nodejs:dynamic:Resource") {
                return {
                    props: { ...props, optionalInput: "default2" },
                    opts,
                };
            }
        },
    ],
});

// Scenario #5 - cross-resource transformations that inject dependencies on one resource into another.
class MyOtherComponent extends pulumi.ComponentResource {
    child1: SimpleResource;
    child2: SimpleResource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:component:MyComponent", name, {}, opts);
        this.child1 = new SimpleResource(`${name}-child1`, { input: "hello" }, { parent: this });
        this.child2 = new SimpleResource(`${name}-child2`, { input: "hello" }, { parent: this });
        this.registerOutputs({});
    }
}

const transformChild1DependsOnChild2: pulumi.ResourceTransformation = (() => {
    // Create a promise that wil be resolved once we find child2.  This is needed because we do not
    // know what order we will see the resource registrations of child1 and child2.
    let child2Found: (res: pulumi.Resource) => void;
    const child2 = new Promise<pulumi.Resource>((res) => child2Found = res);

    // Return a transformation which will rewrite child1 to depend on the promise for child2, and
    // will resolve that promise when it finds child2.
    return (args: pulumi.ResourceTransformationArgs) => {
        if (args.name.endsWith("-child2")) {
            // Resolve the child2 promise with the child2 resource.
            child2Found(args.resource);
            return undefined;
        } else if (args.name.endsWith("-child1")) {
            // Overwrite the `input` to child2 with a dependency on the `output2` from child1.
            const child2Input = pulumi.output(args.props["input"]).apply(async (input) => {
                if (input !== "hello") {
                    // Not strictly necessary - but shows we can confirm invariants we expect to be
                    // true.
                    throw new Error("unexpected input value");
                }
                return child2.then(c2Res => c2Res["output2"]);
            });
            // Finally - overwrite the input of child2.
            return {
                props: { ...args.props, input: child2Input },
                opts: args.opts,
            };
        }
    };
})();

const res5 = new MyOtherComponent("res5", {
    transformations: [ transformChild1DependsOnChild2 ],
});
