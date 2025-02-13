// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    aNumber: number;
    aString: string;
    aBoolean: boolean;
}

export class MyComponent extends pulumi.ComponentResource {
    outNumber: pulumi.Output<number>;
    outString: pulumi.Output<string>;
    outBoolean: pulumi.Output<boolean>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
