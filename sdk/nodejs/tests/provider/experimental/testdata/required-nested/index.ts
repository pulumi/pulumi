// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** An inner type with optional fields */
interface InnerType {
    /** Inner field x */
    x?: number;
    /** Inner field y */
    y?: string;
}

/** An outer type with optional fields including a nested type */
interface OuterType {
    /** An inner object */
    inner?: InnerType;
    /** An outer field */
    outerField?: boolean;
}

export interface MyComponentArgs {
    /** A required outer type with nested inner type */
    requiredOuter?: pulumi.Input<Required<OuterType>>;
    /** A nested required of the inner type */
    requiredInner?: pulumi.Input<Required<InnerType>>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The required outer type output */
    requiredOuter: pulumi.Output<Required<OuterType>>;
    /** The required inner type output */
    requiredInner: pulumi.Output<Required<InnerType>>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.requiredOuter = pulumi.output(args.requiredOuter || { inner: { x: 0, y: "" }, outerField: false });
        this.requiredInner = pulumi.output(args.requiredInner || { x: 0, y: "" });
        this.registerOutputs({
            requiredOuter: this.requiredOuter,
            requiredInner: this.requiredInner,
        });
    }
}
