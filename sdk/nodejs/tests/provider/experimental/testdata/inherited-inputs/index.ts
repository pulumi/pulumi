// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface BaseArgs {
    baseProp: string;
}

interface MyComponentArgs extends BaseArgs {
    childProp: string;
}

export class MyComponent extends pulumi.ComponentResource {
    outResult: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
