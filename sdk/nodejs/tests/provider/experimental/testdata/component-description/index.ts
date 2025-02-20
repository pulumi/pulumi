// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// biome-ignore lint: MyComponentArgs needs to be an interface but biome doesn't like that
export interface MyComponentArgs {}

/**
 * This is a description of MyComponent
 * It can span multiple lines
 */
export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
