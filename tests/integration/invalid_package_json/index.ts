// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";
import * as runtime from "@pulumi/pulumi/runtime"

(async function() {
    const config = new Config();
    const deps = await runtime.computeCodePaths();
    const actual = JSON.stringify(deps);
    const expected = "";
    if (actual !== expected) {
        throw new Error(`Got '${actual}' expected '${expected}`)
    }
})()