// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** A string enum with a single member */
enum Environment {
    /** The only environment */
    Dev = "development",
}

/** A numeric enum with a single member */
enum Version {
    V1 = 1,
}

/** An `as const` object with a single member */
const Mode = {
    Only: "only",
} as const;
type Mode = (typeof Mode)[keyof typeof Mode];

export interface MyComponentArgs {
    env: pulumi.Input<Environment>;
    version?: pulumi.Input<Version>;
    mode?: pulumi.Input<Mode>;
}

export class MyComponent extends pulumi.ComponentResource {
    env: pulumi.Output<Environment>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.env = pulumi.output(args.env);
        this.registerOutputs({ env: this.env });
    }
}
