// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    aMapOfStrings: { [key: string]: string };
    aMapOfNumbers: { [key: string]: number };
    aMapOfBooleans?: { [key: string]: boolean };
    mapOfStringInputs: { [key: string]: pulumi.Input<string> };
    mapOfNumberInputs: { [key: string]: pulumi.Input<number> };
    mapOfBooleanInputs: { [key: string]: pulumi.Input<boolean> };
}

export class MyComponent extends pulumi.ComponentResource {
    outMapOfStrings: pulumi.Output<{ [key: string]: string }>;
    outMapOfNumbers: pulumi.Output<{ [key: string]: number }>;
    outMapOfBooleans: pulumi.Output<{ [key: string]: boolean }>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
