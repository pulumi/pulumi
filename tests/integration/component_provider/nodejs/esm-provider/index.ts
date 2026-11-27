// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// Top-level await only works if the provider loads as an ES module.
const greeting: string = await Promise.resolve("Hello");

interface MyComponentArgs {
    name: pulumi.Input<string>;
}

export class MyComponent extends pulumi.ComponentResource {
    message: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("esm-component-provider:index:MyComponent", name, args, opts);
        this.message = pulumi.output(args.name).apply((n) => `${greeting}, ${n}!`);
        this.registerOutputs({ message: this.message });
    }
}
