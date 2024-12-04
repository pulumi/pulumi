// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Named extends pulumi.CustomResource {
    public readonly name!: pulumi.Output<string>;
    constructor(name: string, resourceName?: string) {
        super("testprovider:index:Named", name, { name: resourceName });
    }
}

export let autoName = new Named("test1").name;
export let explicitName = new Named("test2", "explicit-name").name;
