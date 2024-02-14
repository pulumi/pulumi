// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as runtime from "@pulumi/pulumi/runtime"

(async function () {
    const deps = await runtime.computeCodePaths();

    const actual = JSON.stringify([...deps.keys()].sort());
    const expected = `["../node_modules/lru-cache","../node_modules/semver","../node_modules/yallist"]`;

    if (actual !== expected) {
        throw new Error(`Got '${actual}' expected '${expected}'`)
    }
})();