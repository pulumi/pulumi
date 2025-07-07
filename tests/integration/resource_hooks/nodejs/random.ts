// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface RandomArgs {
    length: pulumi.Input<number>;
}

export class Random extends pulumi.CustomResource {
    public readonly length!: pulumi.Output<number>;
    public readonly result!: pulumi.Output<string>;
    constructor(name: string, args: RandomArgs, opts?: pulumi.CustomResourceOptions) {
        const props = {
            length: args.length,
            result: undefined,
        }
        super("testprovider:index:Random", name, props, opts);
    }
}
