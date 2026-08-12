// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as debuggable from "@pulumi/pulumi/runtime/debuggable.js";

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

debuggable.debuggablePromise(sleep(5 * 60 * 1000), "sleep")

process.exit(0);
