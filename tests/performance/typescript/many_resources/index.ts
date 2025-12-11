// Copyright 2025, Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";

class TestResourceProvider implements dynamic.ResourceProvider {
    async create(inputs: any): Promise<dynamic.CreateResult> {
        return {
            id: inputs.id || "test",
            outs: inputs,
        };
    }
}

class TestResource extends dynamic.Resource {
    constructor(name: string, props: any) {
        super(new TestResourceProvider(), name, props, undefined);
    }
}

for (let i = 0; i < 4000; i++) {
    new TestResource(`resource-${i}`, {
        id: `test-${i}`,
        value: `test-value-${i}`,
    });
}
