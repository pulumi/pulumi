// Copyright 2016-2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class TestProvider extends pulumi.ProviderResource {
    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super("testprovider", name, {}, opts);
    }
}
