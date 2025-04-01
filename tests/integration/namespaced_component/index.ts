// Copyright 2025 Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi"

export interface Nested {
}

export interface MyComponentArgs {
    anInput: Nested;
}

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
	super("namespaced-component:component:MyComponent", name, args, opts);

	// Create a resource here
    }
}
