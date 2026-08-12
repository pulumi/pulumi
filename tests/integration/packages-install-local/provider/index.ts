// Copyright 2025 Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi"

export interface MyComponentArgs { }

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
