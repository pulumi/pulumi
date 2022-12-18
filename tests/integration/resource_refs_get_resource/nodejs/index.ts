// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";

import * as pulumi from "@pulumi/pulumi";

class Child extends pulumi.ComponentResource {
    public readonly message!: pulumi.Output<string>;
    constructor(name: string, message?: string, opts?: pulumi.ResourceOptions) {
        const args = { message }
        super("test:index:Child", name, args, opts);
        if (opts?.urn) {
            return;
        }
        this.registerOutputs(args);
    }
}

class Container extends pulumi.ComponentResource {
    public readonly child!: pulumi.Output<Child>;
    constructor(name: string, child?: Child, opts?: pulumi.ResourceOptions) {
        const args = { child };
        super("test:index:Container", name, args, opts);
        if (opts?.urn) {
            return;
        }
        this.registerOutputs(args);
    }
}

pulumi.runtime.registerResourceModule("test", "index", {
    version: "0.0.1",
    construct: (name: string, type: string, urn: string) => {
        switch (type) {
            case "test:index:Child":
                return new Child(name, undefined, { urn });
            default:
                throw new Error(`unknown resource type: ${type}`);
        }
    },
});

const child = new Child("mychild", "hello world!");
const container = new Container("mycontainer", child);

export const childUrn = pulumi.all([child.urn, container.urn]).apply(([childUrn, urn]) => {
    const roundTrippedContainer = new Container("mycontainer", undefined, { urn })
    const roundTrippedContainerChildUrn = roundTrippedContainer.child.apply(c => c.urn);
    const roundTrippedContainerChildMessage = roundTrippedContainer.child.apply(c => c.message);
    return pulumi.all([childUrn, roundTrippedContainerChildUrn, roundTrippedContainerChildMessage])
        .apply(([expectedUrn, actualUrn, actualMessage]) => {
        assert.strictEqual(actualUrn, expectedUrn);
        assert.strictEqual(actualMessage, "hello world!");
        return expectedUrn;
    });
});
