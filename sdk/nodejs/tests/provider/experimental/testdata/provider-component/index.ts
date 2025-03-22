// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    message: string;
}

export class MyComponent extends pulumi.ComponentResource {
    public readonly formattedMessage: pulumi.Output<string>;
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
