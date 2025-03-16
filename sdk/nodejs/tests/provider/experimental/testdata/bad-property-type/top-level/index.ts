// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    invalidProp: pulumi.Input<boolean | string>;
}

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
