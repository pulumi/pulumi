// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    aNumber: number;
    anOptionalString?: string
}

export class MyComponent extends pulumi.ComponentResource {
    aNumberOutput: pulumi.Output<number>;
    anOptionalStringOutput?: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("nodejs-component-provider:index:MyComponent", name, args, opts);
    }
}
