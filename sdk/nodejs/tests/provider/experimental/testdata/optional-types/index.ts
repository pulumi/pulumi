// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    optionalNumber?: number;
    optionalNumberType: number | undefined;
    optionalBoolean?: boolean;
    optionalBooleanType: boolean | undefined;
}

export class MyComponent extends pulumi.ComponentResource {
    optionalOutputNumber?: pulumi.Output<number>;
    optionalOutputType?: pulumi.Output<number | undefined>;
    optionalOutputBoolean?: pulumi.Output<boolean>;
    optionalOutputBooleanType?: pulumi.Output<boolean | undefined>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
