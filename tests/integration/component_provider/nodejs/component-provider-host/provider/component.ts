// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface Nested {
    aNumber: number;
}

export interface Complex {
    aNumber: number;
    nestedComplexType: Nested;
}

export interface MyComponentArgs {
    aNumber: number;
    anOptionalString?: string
    aBooleanInput: pulumi.Input<boolean>;
    aComplexTypeInput: pulumi.Input<Complex>;
}

export class MyComponent extends pulumi.ComponentResource {
    aNumberOutput: pulumi.Output<number>;
    anOptionalStringOutput?: pulumi.Output<string>;
    aBooleanOutput: pulumi.Output<boolean>;
    aComplexTypeOutput: pulumi.Output<Complex>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("nodejs-component-provider:index:MyComponent", name, args, opts);
        this.aNumberOutput = pulumi.output(args.aNumber * 2);
        this.anOptionalStringOutput = pulumi.output("Hello, " + (args.anOptionalString ?? "World") + "!");
        this.aBooleanOutput = pulumi.output(args.aBooleanInput).apply(b => !b);
        this.aComplexTypeOutput = pulumi.output(args.aComplexTypeInput).apply(ct => ({
            aNumber: ct.aNumber * 2,
            nestedComplexType: { aNumber: ct.nestedComplexType.aNumber * 2 }
        }));
        this.registerOutputs({
            aNumberOutput: this.aNumberOutput,
            anOptionalStringOutput: this.anOptionalStringOutput,
            aBooleanOutput: this.aBooleanOutput,
            aComplexTypeOutput: this.aComplexTypeOutput,
        });
    }
}
