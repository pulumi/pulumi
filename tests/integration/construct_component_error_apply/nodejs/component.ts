// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface ComponentArgs {
    foo: pulumi.Input<string>;
}

export class Component extends pulumi.ComponentResource {
    public readonly foo!: pulumi.Output<string>;

    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        const inputs: any = {};
        inputs["foo"] = args.foo;

        super("testcomponent:index:Component", name, inputs, opts, true);
    }
}

