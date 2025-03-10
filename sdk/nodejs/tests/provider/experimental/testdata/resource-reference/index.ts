// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export abstract class MyAbstractResource extends pulumi.CustomResource {
    constructor(name: string, args: {}, opts?: pulumi.CustomResourceOptions) {
        super("provider:index:MyAbstractResource", name, args, opts);
    }
}

export class MyResource extends MyAbstractResource {
    constructor(name: string, args: {}, opts?: pulumi.CustomResourceOptions) {
        super("provider:index:MyResource", name, args, opts);
    }
}

export interface MyComponentArgs {
    aResource: MyResource;
}

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
