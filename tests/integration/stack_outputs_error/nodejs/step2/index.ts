// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class MyResource extends pulumi.dynamic.Resource {
    constructor(name: string) {
        super({
            async create(inputs) {
                throw new Error("some error");
            }
        }, name, {});
    }
}

export let xyz = "DEF";

new MyResource("test");

export let foo = 1;
