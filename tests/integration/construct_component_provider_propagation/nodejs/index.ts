// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Component extends pulumi.ComponentResource {
    public readonly result!: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        const inputs = {
            result: undefined /*out*/,
        };
        super("testcomponent:index:Component", name, inputs, opts, true);
    }
}

class RandomProvider extends pulumi.ProviderResource {
    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super("testprovider", name, {}, opts);
    }
}

const explicitProvider = new RandomProvider("explicit");

new Component("uses_default");
new Component("uses_provider", {provider: explicitProvider});
new Component("uses_providers", {providers: [explicitProvider]});
new Component("uses_providers_map", {providers: {testprovider: explicitProvider}});
