// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    anAny: any;
    anyInput: pulumi.Input<any>;
}

export class MyComponent extends pulumi.ComponentResource {
    outAny: pulumi.Output<any>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
