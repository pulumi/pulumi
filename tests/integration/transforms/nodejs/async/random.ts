// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface RandomArgs {
    length: pulumi.Input<number>;
    prefix?: pulumi.Input<string | undefined>;
}

export class Random extends pulumi.CustomResource {
    declare public readonly length: pulumi.Output<number>;
    declare public readonly result: pulumi.Output<string>;
    constructor(name: string, args: RandomArgs, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:Random", name, args, opts);
    }

    randomInvoke(args) {
        return pulumi.runtime.invoke("testprovider:index:returnArgs", args);
    }
}
