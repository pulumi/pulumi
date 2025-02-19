// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    aRecordOfStrings: Record<string, string>;
    aRecordOfNumbers?: Record<string, number>;
    aRecordOfBooleans: Record<string, boolean>;
    aMapOfStrings: { [key: string]: string };
    aMapOfNumbers: { [key: string]: number };
    aMapOfBooleans?: { [key: string]: boolean };
}

export class MyComponent extends pulumi.ComponentResource {
    outMapOfStrings: pulumi.Output<Map<string, string>>;
    outMapOfNumbers: pulumi.Output<Map<string, number>>;
    outMapOfBooleans: pulumi.Output<Map<string, boolean>>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
