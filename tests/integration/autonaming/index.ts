// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Named extends pulumi.CustomResource {
    public readonly name!: pulumi.Output<string>;
    constructor(name: string, resourceName?: string) {
        super("testprovider:index:Named", name, { name: resourceName });
    }
}

class MyComponent extends pulumi.ComponentResource {
    constructor(name: string) {
        // We have a type of shape provider:resourcename without a module name intentionally,
        // to test against regressing https://github.com/pulumi/pulumi/issues/18499.
        super('testprovider:my-component', name);
    }
}

export let autoName = new Named("test1").name;
export let explicitName = new Named("test2", "explicit-name").name;
export let componentUrn = new MyComponent("test3").urn;
