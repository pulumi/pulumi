// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    /** A union of string and number - this should be rejected */
    mixedUnion?: pulumi.Input<string | number>;
}

export class MyComponent extends pulumi.ComponentResource {
    mixedUnion: pulumi.Output<string | number>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.mixedUnion = pulumi.output(args.mixedUnion || "default");
        this.registerOutputs({
            mixedUnion: this.mixedUnion,
        });
    }
}
