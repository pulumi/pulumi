// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";
import * as runtime from "@pulumi/pulumi/runtime"

(async function() {
    // Just test that basic config works.
    const config = new Config();
    await runtime.computeCodePaths();
})()