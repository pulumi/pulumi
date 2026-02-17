// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** A type with optional fields */
interface SomeType {
    /** An optional string field */
    a?: string;
    /** An optional number field */
    b?: number;
    /** An optional boolean field */
    c?: boolean;
}

export interface MyComponentArgs {
    /** The regular type with optional fields */
    regularType?: pulumi.Input<SomeType>;
    /** A required type where all fields are required */
    requiredType?: pulumi.Input<Required<SomeType>>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The regular type output */
    regularType: pulumi.Output<SomeType>;
    /** The required type output */
    requiredType: pulumi.Output<Required<SomeType>>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.regularType = pulumi.output(args.regularType || {});
        this.requiredType = pulumi.output(args.requiredType || { a: "", b: 0, c: false });
        this.registerOutputs({
            regularType: this.regularType,
            requiredType: this.requiredType,
        });
    }
}
