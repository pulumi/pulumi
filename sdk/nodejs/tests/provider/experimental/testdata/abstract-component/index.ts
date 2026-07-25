// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface MyComponentArgs {
    input: string;
}

export abstract class MyComponent extends pulumi.ComponentResource {
    outResult: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
