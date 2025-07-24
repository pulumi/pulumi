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

// Exit with a non-zero code. We have in flight promises, but we don't wanat to
// report the promise leak message.
process.exit(123);
