// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import * as pulumi from "@pulumi/pulumi";

// FlakyCreate is a custom resource that uses the testprovider:index:FlakyCreate resource.
// This resource fails on the first create attempt and succeeds on retry.
class FlakyCreate extends pulumi.CustomResource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:FlakyCreate", name, {}, opts);
    }
}

async function onError(args: pulumi.ErrorHookArgs): Promise<boolean> {
    pulumi.log.info(`onError was called for ${args.name} (${args.failedOperation})`);

    assert.strictEqual(args.name, "res", `expected name to be 'res', got '${args.name}'`);
    assert.strictEqual(
        args.type,
        "testprovider:index:FlakyCreate",
        `expected type to be 'testprovider:index:FlakyCreate', got '${args.type}'`
    );
    assert.strictEqual(
        args.failedOperation,
        "create",
        `expected failed operation 'create', got '${args.failedOperation}'`
    );
    assert.ok(args.errors.length > 0, "expected at least one error message");

    return true;
}

const hook = new pulumi.ErrorHook("onError", onError);

const res = new FlakyCreate("res", {
    hooks: {
        onError: [hook],
    },
});
