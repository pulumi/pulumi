// Copyright 2016-2025, Pulumi Corporation.

/**
 * Tests that dynamic providers can return inputs from read() method.
 * This is a regression test for https://github.com/pulumi/pulumi/issues/13839
 *
 * The issue was that after `pulumi refresh`, `pulumi preview --diff` would show
 * all properties as changing because the stored inputs didn't match the refreshed
 * outputs. The fix allows read() to return inputs that will be used for subsequent
 * diffs.
 */

import * as pulumi from "@pulumi/pulumi";

class SimpleProvider implements pulumi.dynamic.ResourceProvider {
    async create(props: any): Promise<pulumi.dynamic.CreateResult> {
        return {
            id: "test-resource-id",
            outs: { value: props.value },
        };
    }

    async read(id: string, props: any): Promise<pulumi.dynamic.ReadResult> {
        // Read returns the current state as both outputs AND inputs.
        // This ensures that after a refresh, subsequent diffs will compare
        // against the refreshed state rather than the original inputs.
        return {
            id: id,
            props: props,
            inputs: { value: props.value }, // Return inputs to fix diff after refresh
        };
    }
}

class SimpleResource extends pulumi.dynamic.Resource {
    public readonly value!: pulumi.Output<string>;

    constructor(name: string, value: string, opts?: pulumi.ResourceOptions) {
        super(
            new SimpleProvider(),
            name,
            { value: value },
            opts,
        );
    }
}

// Create a simple resource
const res = new SimpleResource("test", "hello");

export const resourceId = res.id;
