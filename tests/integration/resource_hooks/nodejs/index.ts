// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { log, ResourceHook, ResourceHookArgs } from "@pulumi/pulumi";
import { Random, Component } from "./random";


function fun(args: ResourceHookArgs) {
    log.info(`fun was called with length = ${args.newInputs["length"]}`);
    assert.strictEqual(args.name, "res")
    assert.strictEqual(args.type, "testprovider:index:Random")
}

const hook = new ResourceHook("hook_fun", fun);

const res = new Random("res", { length: 10 }, {
    hooks: {
        beforeCreate: [hook]
    }
});

function funComp(args: ResourceHookArgs) {
    const childId = args.newOutputs["childId"]
    log.info(`funComp was called with child = ${childId}`);
    if (!childId) {
        throw new Error(`expected non empty childId, got '${childId}'`);
    }
    assert.strictEqual(args.name, "comp")
    assert.strictEqual(args.type, "testprovider:index:Component")
}

const hookComp = new ResourceHook("hook_fun_comp", funComp);

const comp = new Component("comp", { length: 7 }, {
    hooks: {
        afterCreate: [hookComp],
    }
});
