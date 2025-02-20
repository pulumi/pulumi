// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    anArrayOfStrings: Array<string>;
    anArrayOfNumbers?: Array<number>;
    anArrayOfBooleans: Array<boolean>;
    inputArrayOfStrings: pulumi.Input<Array<string>>;
    inputArrayOfNumbers?: pulumi.Input<Array<number>>;
    inputArrayOfBooleans: pulumi.Input<Array<boolean>>;
    inputOfInputStrings: pulumi.Input<Array<pulumi.Input<string>>>;
    inputOfInputNumbers?: pulumi.Input<Array<pulumi.Input<number>>>;
    inputOfInputBooleans: pulumi.Input<pulumi.Input<boolean>[]>;
    anArrayOfInputStrings: Array<pulumi.Input<string>>;
    anArrayOfInputNumbers?: Array<pulumi.Input<number>>;
    anArrayOfInputBooleans: Array<pulumi.Input<boolean>>;
    aListOfStrings: string[];
    aListOfNumbers: number[];
    aListOfBooleans?: boolean[];
}

export class MyComponent extends pulumi.ComponentResource {
    outArrayOfStrings: pulumi.Output<string[]>;
    outArrayOfNumbers: pulumi.Output<number[]>;
    outArrayOfBooleans: pulumi.Output<boolean[]>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
