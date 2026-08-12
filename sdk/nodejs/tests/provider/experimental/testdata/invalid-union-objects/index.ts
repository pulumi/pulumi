// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface TypeA {
    fieldA: string;
}

interface TypeB {
    fieldB: number;
}

export interface MyComponentArgs {
    /** A union of two object types - this should be rejected */
    objectUnion?: pulumi.Input<TypeA | TypeB>;
}

export class MyComponent extends pulumi.ComponentResource {
    objectUnion: pulumi.Output<TypeA | TypeB>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.objectUnion = pulumi.output(args.objectUnion || { fieldA: "default" });
        this.registerOutputs({
            objectUnion: this.objectUnion,
        });
    }
}
