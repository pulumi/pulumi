// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class TestProvider extends pulumi.ProviderResource {
    constructor(name: string) {
        super("testprovider", name);
    }
}
