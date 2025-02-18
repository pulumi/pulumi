// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface TypeA {
    b: TypeB;
}

export interface TypeB {
    a: TypeA;
}

export interface MyComponentArgs {
    typeAInput: pulumi.Input<TypeA>;
}

export class MyComponent extends pulumi.ComponentResource {
    typeBOutput: pulumi.Output<TypeB>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
