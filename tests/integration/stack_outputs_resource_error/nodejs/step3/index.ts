// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class FailsOnCreate extends pulumi.CustomResource {
    public readonly value!: pulumi.Output<number>;
    constructor(name: string) {
        super("testprovider:index:FailsOnCreate", name, { value: undefined });
    }
}

export let xyz = "DEF";

new FailsOnCreate("test");

export let foo = 1;
