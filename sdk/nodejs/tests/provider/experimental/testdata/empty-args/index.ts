// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// biome-ignore lint/suspicious/noEmptyInterface: testing empty args interface behavior
interface MyComponentArgs {}

export class MyComponent extends pulumi.ComponentResource {
    outResult: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
