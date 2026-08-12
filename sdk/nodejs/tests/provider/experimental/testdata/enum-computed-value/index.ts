// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

function computeValue(x: number): number {
    return x * 10;
}

enum Priority {
    /** Computed values - this should be rejected, we only support literal values */
    Low = computeValue(1),
    Medium = computeValue(2),
    High = computeValue(3),
}

export interface MyComponentArgs {
    priority?: pulumi.Input<Priority>;
}

export class MyComponent extends pulumi.ComponentResource {
    priority: pulumi.Output<Priority>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.priority = pulumi.output(args.priority !== undefined ? args.priority : Priority.Low);
        this.registerOutputs({
            priority: this.priority,
        });
    }
}
