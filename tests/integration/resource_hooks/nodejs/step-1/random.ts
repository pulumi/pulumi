// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface RandomArgs {
    length: pulumi.Input<number>;
}

export class Random extends pulumi.CustomResource {
    public readonly length!: pulumi.Output<number>;
    public readonly result!: pulumi.Output<string>;
    constructor(name: string, args: RandomArgs, opts?: pulumi.CustomResourceOptions) {
        const props = {
            length: args.length,
            result: undefined,
        }
        super("testprovider:index:Random", name, props, opts);
    }
}

interface ComponentArgs {
    length: pulumi.Input<number>;
}

export class Component extends pulumi.ComponentResource {
    public readonly length!: pulumi.Output<number>;
    public readonly childId!: pulumi.Output<string>;
    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("testprovider:index:Component", name, args, opts, true);
    }
}

export class TestProvider extends pulumi.ProviderResource {
    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super("testprovider", name, {}, opts);
    }
}
