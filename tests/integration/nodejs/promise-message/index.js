// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Named extends pulumi.CustomResource {
    constructor(name, resourceName) {
        super("testprovider:index:Named", name, { name: resourceName });
    }
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

sleep(2000).then(() => {
    new Named("res");
})

process.exit(0);
