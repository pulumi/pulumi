
// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { log, ResourceHook, ResourceHookArgs } from "@pulumi/pulumi";
import { Random, Component } from "./random";


function beforeCreate(args: ResourceHookArgs) {
    log.info(`beforeCreate was called with length = ${args.newInputs["length"]}`);
    assert.strictEqual(args.name, "res")
    assert.strictEqual(args.type, "testprovider:index:Random")
}

const beforeCreateHook = new ResourceHook("beforeCreate", beforeCreate);

function beforeDelete(args: ResourceHookArgs) {
    log.info(`beforeDelete was called with length = ${args.oldInputs["length"]}`);
    assert.strictEqual(args.name, "res")
    assert.strictEqual(args.type, "testprovider:index:Random")
}

const beforeDeleteHook = new ResourceHook("beforeDelete", beforeDelete);

// removed `res` here, we expect its beforeDelete hook to be called

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
