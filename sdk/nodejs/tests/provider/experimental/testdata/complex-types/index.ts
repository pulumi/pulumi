// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// We can use a class or interface to define complex types.
export class MyClassType {
    aString: string;
}

export interface MyInterfaceType {
    aNumber: number;
}

export interface MyComponentArgs {
    anInterfaceType: MyInterfaceType;
    aClassType: MyClassType;
}

export class MyComponent extends pulumi.ComponentResource {
    anInterfaceTypeOutput: pulumi.Output<MyInterfaceType>;
    aClassTypeOutput: pulumi.Output<MyClassType>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
