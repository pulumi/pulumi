// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Named extends pulumi.CustomResource {
    constructor(name, resourceName) {
        super("testprovider:index:Named", name, { name: resourceName });
    }
}

let done = false

process.on("beforeExit", () => {
    if (done) {
        return;
    }
    done = true;
    new Named("beforeExit");
})
