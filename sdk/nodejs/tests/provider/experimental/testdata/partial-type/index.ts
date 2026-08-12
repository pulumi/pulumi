// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** A type with required fields */
interface SomeType {
    /** A required string field */
    a: string;
    /** A required number field */
    b: number;
    /** A required boolean field */
    c: boolean;
}

export interface MyComponentArgs {
    /** The regular type with required fields */
    regularType?: pulumi.Input<SomeType>;
    /** A partial type where all fields are optional */
    partialType?: pulumi.Input<Partial<SomeType>>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The regular type output */
    regularType: pulumi.Output<SomeType>;
    /** The partial type output */
    partialType: pulumi.Output<Partial<SomeType>>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.regularType = pulumi.output(args.regularType || { a: "", b: 0, c: false });
        this.partialType = pulumi.output(args.partialType || {});
        this.registerOutputs({
            regularType: this.regularType,
            partialType: this.partialType,
        });
    }
}
