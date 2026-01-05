// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { log, ResourceHook, ResourceHookArgs, ResourceOptions, ResourceTransform, ResourceTransformArgs, ResourceTransformResult } from "@pulumi/pulumi";
import { Random, Component } from "./random";


function fun(args: ResourceHookArgs) {
    if (args.name === "res") {
        log.info(`fun was called with length = ${args.newInputs["length"]}`);
        assert.strictEqual(args.name, "res", `Expected name 'res', got ${args.name}`);
        assert.strictEqual(args.type, "testprovider:index:Random", `Expected type 'testprovider:index:Random', got ${args.type}`);
    } else if (args.name === "comp") {
        const childId = args.newOutputs["childId"];
        log.info(`fun_comp was called with child = ${childId}`);
        if (!childId) {
            throw new Error(`expected non empty childId, got '${childId}'`);
        }
        assert.strictEqual(args.name, "comp", `Expected name 'comp', got ${args.name}`);
        assert.strictEqual(args.type, "testprovider:index:Component", `Expected type 'testprovider:index:Component', got ${args.type}`);
    } else {
        throw new Error(`got unexpected component name: ${args.name}`);
    }
}

const hook = new ResourceHook("hook_fun", fun);

function transform(args: ResourceTransformArgs): ResourceTransformResult {
    const opts = args.opts;

    const existingAfterCreate = opts.hooks?.afterCreate || [];
    const newHooks = {
        ...opts.hooks,
        afterCreate: [...existingAfterCreate, fun]
    };

    return {
        props: args.props,
        opts: {
            ...opts,
            hooks: newHooks
        }
    };
}

const res = new Random("res", { length: 10 }, {
    transforms: [transform]
});

const comp = new Component("comp", { length: 7 }, {
    transforms: [transform]
});
