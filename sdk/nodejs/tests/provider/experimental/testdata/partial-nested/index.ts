// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** An inner type */
interface InnerType {
    /** Inner field x */
    x: number;
    /** Inner field y */
    y: string;
}

/** An outer type that contains another type */
interface OuterType {
    /** An inner object */
    inner: InnerType;
    /** An outer field */
    outerField: boolean;
}

export interface MyComponentArgs {
    /** A partial outer type with nested inner type */
    partialOuter?: pulumi.Input<Partial<OuterType>>;
    /** A nested partial of the inner type */
    partialInner?: pulumi.Input<Partial<InnerType>>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The partial outer type output */
    partialOuter: pulumi.Output<Partial<OuterType>>;
    /** The partial inner type output */
    partialInner: pulumi.Output<Partial<InnerType>>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.partialOuter = pulumi.output(args.partialOuter || {});
        this.partialInner = pulumi.output(args.partialInner || {});
        this.registerOutputs({
            partialOuter: this.partialOuter,
            partialInner: this.partialInner,
        });
    }
}
