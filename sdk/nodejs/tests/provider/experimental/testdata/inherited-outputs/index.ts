// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface MyComponentArgs {
    input: string;
}

// BaseComponent is a component with its own output. It is not registered directly; MyComponent extends it. This
// exercises the standing bug where outputs inherited from a base component were silently dropped.
class BaseComponent extends pulumi.ComponentResource {
    baseOutput: pulumi.Output<string>;

    constructor(name: string, args: any, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:BaseComponent", name, args, opts);
    }
}

export class MyComponent extends BaseComponent {
    childOutput: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super(name, args, opts);
    }
}
