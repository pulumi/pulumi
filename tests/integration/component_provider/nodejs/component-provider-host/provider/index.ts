// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

enum MyEnum {
    A = "a",
    B = "b",
}

const MyConstEnum = { C: "c", D: "d" } as const;
type MyConstEnum = typeof MyConstEnum[keyof typeof MyConstEnum];

interface Nested {
    aNumber: number;
}

interface Complex {
    aNumber: number;
    nestedComplexType: Nested;
}

interface MyComponentArgs {
    aNumber: number;
    anOptionalString?: string
    aBooleanInput: pulumi.Input<boolean>;
    aComplexTypeInput: pulumi.Input<Complex>;
    enumInput: pulumi.Input<MyEnum>;
}

export class MyComponent extends pulumi.ComponentResource {
    aNumberOutput: pulumi.Output<number>;
    anOptionalStringOutput?: pulumi.Output<string>;
    aBooleanOutput: pulumi.Output<boolean>;
    aComplexTypeOutput: pulumi.Output<Complex>;
    aResourceOutput: pulumi.Output<random.RandomPet>;
    aString: string;
    enumOutput: pulumi.Output<MyConstEnum>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("nodejs-component-provider:index:MyComponent", name, args, opts);
        this.aNumberOutput = pulumi.output(args.aNumber * 2);
        this.anOptionalStringOutput = pulumi.output("Hello, " + (args.anOptionalString ?? "World") + "!");
        this.aBooleanOutput = pulumi.output(args.aBooleanInput).apply(b => !b);
        this.aComplexTypeOutput = pulumi.output(args.aComplexTypeInput).apply(ct => ({
            aNumber: ct.aNumber * 2,
            nestedComplexType: { aNumber: ct.nestedComplexType.aNumber * 2 }
        }));
        this.aResourceOutput = pulumi.output(new random.RandomPet(name + "-pet"));
        this.aString = "hello";
        this.enumOutput = pulumi.output(args.enumInput).apply((e: MyEnum) => {
            if (e === MyEnum.A) {
                return MyConstEnum.C;
            } else if (e === MyEnum.B) {
                return MyConstEnum.D;
            }
            throw new Error(`Invalid enum value: ${e}`);
        });
        this.registerOutputs({
            aNumberOutput: this.aNumberOutput,
            anOptionalStringOutput: this.anOptionalStringOutput,
            aBooleanOutput: this.aBooleanOutput,
            aComplexTypeOutput: this.aComplexTypeOutput,
            aResourceOutput: this.aResourceOutput,
            aString: this.aString,
        });
    }
}
