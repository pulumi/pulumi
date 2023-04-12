// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class FailsOnDelete extends pulumi.CustomResource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:FailsOnDelete", name, {}, opts);
    }
}
