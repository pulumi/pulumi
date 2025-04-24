// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface SelfRecursive {
    self: SelfRecursive;
}

export interface SelfRecursiveComponentOutput {
    self: SelfRecursiveComponentOutput;
}

export interface MyComponentArgs {
    theSelfRecursiveTypeInput: pulumi.Input<SelfRecursive>;
}

export class MyComponent extends pulumi.ComponentResource {
    theSelfRecursiveTypeOutput: pulumi.Output<SelfRecursiveComponentOutput>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
