// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface BaseArgs {
    baseInput: string;
}

interface DerivedArgs extends BaseArgs {
    derivedInput: number;
}

export class Base extends pulumi.ComponentResource {
    baseOutput: pulumi.Output<string>;

    constructor(name: string, args: BaseArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:Base", name, args, opts);
    }
}

export class Derived extends Base {
    derivedOutput: pulumi.Output<number>;

    constructor(name: string, args: DerivedArgs, opts?: pulumi.ComponentResourceOptions) {
        super(name, args, opts);
    }
}
