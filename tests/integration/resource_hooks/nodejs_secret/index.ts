// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { log, ResourceHook, ResourceHookArgs, ResourceOptions, secret } from "@pulumi/pulumi";
import { Echo } from "./echo";


async function fun(args: ResourceHookArgs) {
    const out = args.newInputs["echo"]
    assert.strictEqual(true, await out.isSecret)
    assert.strictEqual("hello secret", await out.promise())
    log.info("hook called")
}

new Echo("echo", { echo: secret("hello secret") }, {
    hooks: {
        beforeCreate: [fun]
    }
});
