// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { log, ResourceHook, ResourceHookArgs } from "@pulumi/pulumi";
import { Random } from "./random";


function fun(args: ResourceHookArgs) {
    log.info(`fun was called with length = ${args.newInputs["length"]}`);
    assert.strictEqual(args.name, "random")
    assert.strictEqual(args.type, "testprovider:index:Random")
}

const hook = new ResourceHook("hook_fun", fun);

const rand = new Random("random", { length: 10 }, {
    hooks: {
        beforeCreate: [hook]
    }
});
